// 客户端数据视图：完整角色档案与公开配置；实时战斗状态和持久化业务数据在此合并。

package main

import (
	"fmt"
	"math"
)

// profile 组装完整角色档案；位置与战斗状态优先读在线状态，背包和任务读数据库。
func (a *App) profile(id string) M {
	r := a.player(id)
	s := M{"id": id, "name": r["name"], "level": r["level"], "exp": r["exp"], "gold": r["gold"], "mapId": r["map_id"], "x": r["x"], "y": r["y"], "hp": r["hp"], "mp": r["mp"], "weaponType": r["weapon_type"], "loadout": arr(parse(str(r["skill_loadout"]))), "buffs": objects(parse(str(r["buffs"]))), "online": false, "safe": false, "devTools": a.dev}
	l := a.live[id]
	if l != nil {
		for _, k := range []string{"level", "exp", "gold", "mapId", "x", "y", "hp", "mp", "weaponType", "loadout", "buffs", "safe"} {
			s[k] = l.State[k]
		}
		s["online"] = l.Peer != nil
	}
	buffs := []M{}
	for _, b := range objects(s["buffs"]) {
		if num(b["until"]) > now() {
			buffs = append(buffs, b)
		}
	}
	s["buffs"] = buffs
	stats := a.stats(id, num(s["level"]), str(s["weaponType"]), buffs)
	if l != nil {
		stats = l.Stats
	}
	s["stats"] = stats
	s["maxHp"] = stats["maxHp"]
	s["maxMp"] = stats["maxMp"]
	s["expMax"] = expNext(num(s["level"]))
	s["weaponName"] = weaponLabels[str(s["weaponType"])]
	// 物品按装备部位、背包和仓库分别输出，词条与品质由实例存档解析。
	equipment := M{"weapon": nil, "head": nil, "chest": nil, "gloves": nil, "boots": nil, "ring": nil}
	bag, bank := []M{}, []M{}
	for _, r := range a.items(id, "") {
		v := a.viewItem(r)
		switch str(r["location"]) {
		case "equipped":
			equipment[str(v["slot"])] = v
		case "bank":
			bank = append(bank, v)
		case "bag":
			bag = append(bag, v)
		}
	}
	s["equipment"], s["bag"], s["bank"] = equipment, bag, bank
	pets := []M{}
	petNames := map[string]string{}
	for _, p := range a.rows("SELECT * FROM player_pets WHERE player_id=?", id) {
		d := a.def("pets", str(p["template_id"]))
		petNames[str(p["id"])] = fallback(d["name"], "灵伴")
		pets = append(pets, M{"id": p["id"], "templateId": p["template_id"], "name": d["name"], "rarity": d["rarity"], "battle": d["battle"], "work": d["work"], "desc": d["desc"], "level": p["level"], "exp": p["exp"], "expMax": 24 + num(p["level"])*18, "active": truth(p["active"]), "busy": a.busy(str(p["id"]))})
	}
	jobs := []M{}
	for _, j := range a.jobs(id) {
		n := a.def("dispatchNodes", str(j["node_id"]))
		jobs = append(jobs, M{"id": j["id"], "petName": petNames[str(j["player_pet_id"])], "nodeId": j["node_id"], "nodeName": n["name"], "durationId": j["duration_id"], "endsAt": j["ends_at"], "ready": now() >= num(j["ends_at"])})
	}
	s["pets"], s["dispatches"] = pets, jobs
	// 活动任务带当前进度，可接取任务按服务器前置条件计算，客户端不自行判定。
	quests, available := []M{}, []M{}
	for _, r := range a.rows("SELECT * FROM player_quests WHERE player_id=? AND status='active'", id) {
		q := a.def("quests", str(r["quest_id"]))
		p := arr(parse(str(r["progress"])))
		goals := []M{}
		for i, g := range objects(q["objectives"]) {
			cur := 0.
			if i < len(p) {
				cur = num(p[i])
			}
			goals = append(goals, M{"text": g["text"], "current": cur, "need": need(g), "type": g["type"], "target": goalTarget(g)})
		}
		quests = append(quests, M{"id": r["quest_id"], "name": q["name"], "desc": q["desc"], "kind": q["kind"], "turnin": a.def("npcs", str(q["turnin"]))["name"], "turninId": q["turnin"], "objectives": goals})
	}
	for _, q := range objects(a.data["quests"]) {
		if a.canAccept(r, q) {
			available = append(available, M{"id": q["id"], "name": q["name"], "desc": q["desc"], "kind": q["kind"], "giver": a.def("npcs", str(q["giver"]))["name"], "giverId": q["giver"], "minLevel": q["minLevel"]})
		}
	}
	s["quests"], s["available"] = quests, available
	s["guild"] = nil
	if member := a.one("SELECT * FROM guild_members WHERE player_id=?", id); member != nil {
		g := a.one("SELECT * FROM guilds WHERE id=?", member["guild_id"])
		members := []M{}
		for _, m := range a.rows("SELECT m.player_id,m.role,p.name FROM guild_members m JOIN players p ON p.id=m.player_id WHERE m.guild_id=?", member["guild_id"]) {
			members = append(members, M{"id": m["player_id"], "name": m["name"], "role": m["role"]})
		}
		s["guild"] = M{"id": g["id"], "name": g["name"], "level": g["level"], "exp": g["exp"], "notice": g["notice"], "role": member["role"], "members": members}
	}
	s["unreadMail"] = num(a.one("SELECT COUNT(*) AS n FROM mail WHERE player_id=? AND is_read=0", id)["n"])
	tracks := M{}
	for _, k := range []string{"weapon", "gather", "craft", "cook"} {
		name := "life:" + k
		if k == "weapon" {
			name = "weapon:" + str(s["weaponType"])
		}
		tr := a.track(id, name)
		tr["expMax"] = trackNext(num(tr["level"]), k == "weapon")
		tracks[k] = tr
	}
	s["tracks"] = tracks
	s["weather"] = a.def("weathers", str(a.weather["id"]))
	s["period"] = a.period
	s["periodName"] = periodLabels[a.period]
	s["flags"] = obj(parse(str(r["flags"])))
	// 潜力余额仅扣自行分配点数，属性展示行还包含种族逐级自动成长。
	race := raceID(str(r["race"]))
	rr := races[race]
	spent := parseAttrs(str(r["attributes"]))
	attrs := totalAttrs(num(s["level"]), spent, race)
	s["race"], s["raceName"], s["raceBlurb"], s["raceGrowth"] = race, rr["name"], rr["blurb"], growthLine(race)
	s["attributes"] = spent
	s["unspent"] = math.Max(0, num(s["level"])*5-spentAttrs(spent))
	rate := obj(rr["rate"])
	rows := []M{}
	for _, k := range attrKeys {
		v := num(attrs[k])
		hint, gain := "", ""
		switch k {
		case "vit":
			hint = fmt.Sprintf("每点生命 +%g，灵力 +0.3，速度 +0.1", num(rate["vitHp"]))
			gain = fmt.Sprintf("生命 +%s，灵力 +%s，速度 +%s", fmtAmount(v*num(rate["vitHp"])), fmtAmount(v*.3), fmtAmount(v*.1))
		case "mag":
			hint = fmt.Sprintf("每点法力 +%g，灵力 +0.7，不加速度", num(rate["magMp"]))
			gain = fmt.Sprintf("法力 +%s，灵力 +%s", fmtAmount(v*num(rate["magMp"])), fmtAmount(v*.7))
		case "str":
			hint = fmt.Sprintf("每点物理伤害 +%s，灵力 +0.4，速度 +0.1", fmtAmount(num(rate["strAtk"])))
			gain = fmt.Sprintf("伤害 +%s，灵力 +%s，速度 +%s", fmtAmount(v*num(rate["strAtk"])), fmtAmount(v*.4), fmtAmount(v*.1))
		case "agi":
			hint = "每点速度 +0.7，出手和走路更快，不加灵力"
			gain = "速度 +" + fmtAmount(v*.7)
		case "end":
			hint = fmt.Sprintf("每点防御 +%g，灵力 +0.2，速度 +0.1", num(rate["endDef"]))
			gain = fmt.Sprintf("防御 +%s，灵力 +%s，速度 +%s", fmtAmount(v*num(rate["endDef"])), fmtAmount(v*.2), fmtAmount(v*.1))
		}
		if v == 0 {
			gain = ""
		}
		rows = append(rows, M{"key": k, "name": attrNames[k], "hint": hint, "value": v, "gain": gain})
	}
	s["attributeRows"] = rows
	return s
}

// publicConfig 输出客户端显示与操作需要的配置，怪物列表省略服务器战斗结算细节。
func (a *App) publicConfig() M {
	out := M{"version": "0.1.2"}
	for _, k := range []string{"maps", "npcs", "items", "skills", "pets", "recipes", "durations", "dispatchNodes", "weathers", "constants"} {
		out[k] = a.data[k]
	}
	monsters, quests := []M{}, []M{}
	for _, m := range objects(a.data["monsters"]) {
		v := M{}
		for _, k := range []string{"id", "name", "level", "role", "tint", "petId", "element"} {
			v[k] = m[k]
		}
		monsters = append(monsters, v)
	}
	for _, q := range objects(a.data["quests"]) {
		v := M{}
		for _, k := range []string{"id", "name", "desc", "kind", "minLevel"} {
			v[k] = q[k]
		}
		quests = append(quests, v)
	}
	out["monsters"], out["quests"] = monsters, quests
	return out
}
