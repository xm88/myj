// 任务系统：服务器事件匹配目标、进度持久化、接取交付条件与 NPC 对话生成。

package main

import (
	"fmt"
	"strings"
)

// need 读取任务目标所需数量，没有 count 字段的目标按一次处理。
func need(goal M) float64 { return number(goal["count"], 1) }

// goalTarget 提取任务目标的怪物、地图、NPC、灵伴、资源种类或物品标识。
func goalTarget(goal M) string {
	for _, k := range []string{"monsterId", "mapId", "npcId", "petId", "nodeKind", "itemId"} {
		if str(goal[k]) != "" {
			return str(goal[k])
		}
	}
	return ""
}

// questState 读取角色的指定任务状态与服务器记录的进度。
func (a *App) questState(id, q string) M {
	return a.one("SELECT * FROM player_quests WHERE player_id=? AND quest_id=?", id, q)
}

// canAccept 检查等级、解锁标记、前置任务和历史领取状态，决定任务是否可接。
func (a *App) canAccept(p, q M) bool {
	id := str(p["id"])
	if a.questState(id, str(q["id"])) != nil || num(p["level"]) < num(q["minLevel"]) {
		return false
	}
	if str(q["flag"]) != "" && !truth(obj(parse(str(p["flags"])))[str(q["flag"])]) {
		return false
	}
	for _, pre := range arr(q["prereq"]) {
		if str(a.questState(id, str(pre))["status"]) != "turned_in" {
			return false
		}
	}
	return true
}

// acceptQuest 创建活动任务，并用当前等级、背包材料和公会状态初始化适用目标。
func (a *App) acceptQuest(id, qid string, force bool) {
	q := a.def("quests", qid)
	check(q != nil, "没有这个任务")
	p := a.player(id)
	check(force || a.canAccept(p, q), "现在还不能接")
	check(a.questState(id, qid) == nil, "已经接过了")
	progress := []any{}
	for _, g := range objects(q["objectives"]) {
		n := 0.
		switch str(g["type"]) {
		case "reach_level":
			if num(p["level"]) >= num(g["level"]) {
				n = 1
			}
		case "collect":
			n = clamp(a.countItem(id, str(g["itemId"])), 0, need(g))
		case "join_guild":
			if a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id) != nil {
				n = 1
			}
		}
		progress = append(progress, n)
	}
	a.run("INSERT INTO player_quests(player_id,quest_id,status,progress,updated_at) VALUES(?,?,'active',?,?)", id, qid, js(progress), now())
	a.notify(id)
}

// questDone 比较所有目标的当前进度与需求，全部满足才允许交付。
func questDone(q M, progress any) bool {
	p := arr(progress)
	for i, g := range objects(q["objectives"]) {
		if i >= len(p) || num(p[i]) < need(g) {
			return false
		}
	}
	return true
}

// applyGoal 将服务器事件匹配到任务目标，只有相关事件才增加进度。
func applyGoal(g, ev M) float64 {
	gt, et := str(g["type"]), str(ev["type"])
	switch gt {
	case "kill":
		if et == "kill" && g["monsterId"] == ev["monsterId"] {
			return 1
		}
	case "kill_any":
		if et == "kill" {
			return 1
		}
	case "collect":
		if et == "collect" && g["itemId"] == ev["itemId"] {
			return num(ev["count"])
		}
	case "capture":
		if et == "capture" && g["petId"] == ev["petId"] {
			return 1
		}
	case "craft":
		if et == "craft" && g["recipeId"] == ev["recipeId"] {
			return 1
		}
	case "enter":
		if et == "enter" && g["mapId"] == ev["mapId"] {
			return 1
		}
	case "gather":
		if et == "gather" && g["nodeKind"] == ev["nodeKind"] {
			return 1
		}
	case "reach_level":
		if et == "level" && num(ev["level"]) >= num(g["level"]) {
			return 1
		}
	case "equip_quality":
		if et == "equip" && qualityRank(str(ev["quality"])) >= qualityRank(str(g["quality"])) {
			return 1
		}
	case "talk":
		if et == "talk" && g["npcId"] == ev["npcId"] {
			return 1
		}
	default:
		if gt == et {
			return 1
		}
	}
	return 0
}

// emit 把玩法事件分发给活动任务，保存变化并提示可交付任务。
func (a *App) emit(id string, ev M) {
	changed, ready := false, false
	for _, r := range a.rows("SELECT * FROM player_quests WHERE player_id=? AND status='active'", id) {
		q := a.def("quests", str(r["quest_id"]))
		if q == nil {
			continue
		}
		p := arr(parse(str(r["progress"])))
		local := false
		// 已达到需求的目标不再增长；每个相关事件按目标需求封顶。
		for i, g := range objects(q["objectives"]) {
			for len(p) <= i {
				p = append(p, 0)
			}
			if num(p[i]) >= need(g) {
				continue
			}
			step := applyGoal(g, ev)
			if step > 0 {
				p[i] = clamp(num(p[i])+step, 0, need(g))
				local = true
			}
		}
		if local {
			a.run("UPDATE player_quests SET progress=?,updated_at=? WHERE player_id=? AND quest_id=?", js(p), now(), id, r["quest_id"])
			changed = true
			if questDone(q, p) {
				ready = true
			}
		}
	}
	if changed {
		a.notify(id)
	}
	if ready {
		a.toast(id, "有任务可以交付了")
	}
}

// turnIn 校验任务完成后标记交付、发放奖励，并按前置条件接续后续主线。
func (a *App) turnIn(id, qid string) {
	q := a.def("quests", qid)
	check(q != nil, "没有这个任务")
	r := a.questState(id, qid)
	check(str(r["status"]) == "active", "任务还没接")
	check(questDone(q, parse(str(r["progress"]))), "还没完成")
	// 先标记任务交付，再发奖励。影响行数必须是 1，避免重复交付把奖励发两次。
	check(a.run("UPDATE player_quests SET status='turned_in',updated_at=? WHERE player_id=? AND quest_id=? AND status='active'", now(), id, qid) == 1, "任务还没接")
	rewards := obj(q["rewards"])
	a.addExp(id, num(rewards["exp"]))
	a.addGold(id, num(rewards["gold"]))
	for _, i := range objects(rewards["items"]) {
		var ex M
		if str(a.def("items", str(i["itemId"]))["kind"]) == "equipment" {
			ex = a.equipmentExtra(fallback(i["quality"], "white"))
		}
		a.addItem(id, str(i["itemId"]), num(i["count"]), ex, "bag")
	}
	if str(rewards["petId"]) != "" {
		a.givePet(id, str(rewards["petId"]), false)
	}
	a.toast(id, "完成了「"+str(q["name"])+"」")
	if next := a.def("quests", str(q["next"])); next != nil && a.canAccept(a.player(id), next) {
		a.acceptQuest(id, str(next["id"]), false)
	}
	a.notify(id)
}

// nearNPC 检查角色在线、冒险模式、同地图与距离，避免远程触发 NPC 对话。
func (a *App) nearNPC(id, npcID string) M {
	npc := a.def("npcs", npcID)
	check(npc != nil, "没有这个人")
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "助手模式下不能交谈，切回冒险")
	check(str(l.State["mapId"]) == str(npc["mapId"]) && dist(l.State, npc) <= 90, "再走近一点")
	return npc
}

// rewardText 将任务经验、金币、物品和灵伴奖励拼成对话展示文本。
func (a *App) rewardText(q M) string {
	r := obj(q["rewards"])
	p := []string{}
	if num(r["exp"]) > 0 {
		p = append(p, fmt.Sprintf("%g 经验", num(r["exp"])))
	}
	if num(r["gold"]) > 0 {
		p = append(p, fmt.Sprintf("%g 金币", num(r["gold"])))
	}
	for _, i := range objects(r["items"]) {
		p = append(p, fmt.Sprintf("%s ×%g", str(a.def("items", str(i["itemId"]))["name"]), num(i["count"])))
	}
	if str(r["petId"]) != "" {
		p = append(p, str(a.def("pets", str(r["petId"]))["name"]))
	}
	return strings.Join(p, "，")
}

// dialog 根据可接、可交和进行中的任务生成 NPC 话题与服务入口。
func (a *App) dialog(id, npcID string) M {
	npc := a.nearNPC(id, npcID)
	a.emit(id, M{"type": "talk", "npcId": npcID})
	p := a.player(id)
	actions, turnins, accepts, pending := []M{}, []M{}, []M{}, []M{}
	for _, q := range objects(a.data["quests"]) {
		qid := str(q["id"])
		topic := func(kind, detail string) M {
			return M{"id": kind + ":" + qid, "kind": kind, "name": q["name"], "desc": q["desc"], "detail": detail, "reward": a.rewardText(q)}
		}
		// 同一个 NPC 可以同时提供可接取、可交付和进行中的多个话题。
		if str(q["giver"]) == npcID && a.canAccept(p, q) {
			actions = append(actions, M{"id": "accept:" + qid, "label": "接受：" + str(q["name"])})
			parts := []string{}
			for _, g := range objects(q["objectives"]) {
				parts = append(parts, str(g["text"]))
			}
			accepts = append(accepts, topic("accept", strings.Join(parts, "，")))
		}
		r := a.questState(id, qid)
		if str(r["status"]) != "active" || (str(q["giver"]) != npcID && str(q["turnin"]) != npcID) {
			continue
		}
		progress := arr(parse(str(r["progress"])))
		done := questDone(q, progress)
		if str(q["turnin"]) == npcID && done {
			actions = append(actions, M{"id": "turnin:" + qid, "label": "交付：" + str(q["name"])})
			turnins = append(turnins, topic("turnin", ""))
		} else {
			detail := ""
			if done {
				detail = "做完了，交给" + str(a.def("npcs", str(q["turnin"]))["name"])
			} else {
				parts := []string{}
				for i, g := range objects(q["objectives"]) {
					cur := 0.
					if i < len(progress) {
						cur = num(progress[i])
					}
					parts = append(parts, fmt.Sprintf("%s %g/%g", str(g["text"]), cur, need(g)))
				}
				detail = strings.Join(parts, "，")
			}
			pending = append(pending, topic("progress", detail))
		}
	}
	labels := M{"shop": "买东西", "craft": "打造 / 料理", "market": "拍卖行", "dispatch": "派遣灵伴", "guild": "公会", "bank": "仓库", "travel": "航标", "skills": "技能"}
	for _, v := range arr(npc["panels"]) {
		panel := str(v)
		actions = append(actions, M{"id": "open:" + panel, "label": fallback(labels[panel], panel)})
	}
	text := "……"
	if lines := arr(npc["lines"]); len(lines) > 0 {
		text = str(lines[0])
	}
	if len(turnins) > 0 {
		text = "「" + str(turnins[0]["name"]) + "」可以交差了。"
	} else if len(accepts) == 1 && len(pending) == 0 {
		text = str(accepts[0]["desc"])
	} else if len(accepts) == 0 && len(pending) == 1 {
		text = "「" + str(pending[0]["name"]) + "」" + str(pending[0]["detail"]) + "。"
	}
	topics := append(append(turnins, accepts...), pending...)
	return M{"npcId": npcID, "name": npc["name"], "title": npc["title"], "text": text, "actions": actions, "topics": topics}
}
