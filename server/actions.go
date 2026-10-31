// 客户端动作：技能、闪避、契约、采集与精确互动；数值结算始终在服务器执行。

package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

// guard 进入一拍守势：这段时间少挨打，也暂缓下一次普通攻击。守势结束前不能续上。
func (a *App) guard(id string) {
	l := a.live[id]
	if l == nil || truth(l.State["safe"]) {
		return
	}
	s := l.State
	t := now()
	if num(s["guardUntil"]) > t {
		return
	}
	interval := num(l.Stats["attackInterval"])
	if interval < 650 {
		interval = 650
	}
	until := t + interval
	s["guardUntil"] = until
	if num(s["attackAt"]) < until {
		s["attackAt"] = until
	}
	push(l, M{"kind": "text", "text": "守势", "x": s["x"], "y": s["y"], "entityId": id})
}

// cast 处理武器技能、灵伴技能和闪避；先检查冷却、法力与距离，再由服务器结算效果。
func (a *App) cast(id string, slot any) {
	l := a.live[id]
	if l == nil || truth(l.State["safe"]) {
		return
	}
	s := l.State
	t := now()
	if str(slot) == "dodge" {
		if num(l.CD["dodge"]) > t {
			return
		}
		l.CD["dodge"] = t + 4000
		s["invulnUntil"] = t + 650
		len := math.Hypot(num(s["inputX"]), num(s["inputY"]))
		if len == 0 {
			len = 1
		}
		x, y := slide(a.def("maps", str(s["mapId"])), num(s["x"]), num(s["y"]), -num(s["inputX"])/len*46, -num(s["inputY"])/len*46, 14)
		s["x"], s["y"] = x, y
		push(l, M{"kind": "text", "text": "闪避", "x": x, "y": y})
		return
	}
	sid := ""
	if str(slot) == "pet" {
		p := a.one("SELECT template_id FROM player_pets WHERE player_id=? AND active=1", id)
		check(p != nil, "没有携带中的灵兽")
		sid = str(a.def("pets", str(p["template_id"]))["skillId"])
	} else {
		check(truth(s["armed"]), "没有装备武器")
		i := int(num(slot)) - 1
		loadout := arr(s["loadout"])
		if i >= 0 && i < len(loadout) {
			sid = str(loadout[i])
		}
	}
	skill := a.def("skills", sid)
	if skill == nil || num(l.CD[sid]) > t {
		return
	}
	check(num(s["mp"]) >= num(skill["mp"]), "法力不足")
	inst := a.instances[l.InstanceKey]
	if inst == nil {
		return
	}
	target := inst.Monsters[str(s["targetId"])]
	if num(skill["coefficient"]) > 0 && !truth(skill["aoe"]) {
		check(target != nil && truth(target["alive"]), "先选中一个目标")
		check(dist(s, target) <= num(skill["range"])+num(a.def("monsters", str(target["templateId"]))["radius"]), "距离不够")
	}
	s["mp"] = math.Max(0, num(s["mp"])-num(skill["mp"]))
	l.CD[sid] = t + num(skill["cooldown"])
	if skill["buff"] != nil {
		a.addBuff(l, sid, obj(skill["buff"]))
		a.refresh(id)
	}
	s["shield"] = num(s["shield"]) + num(skill["shieldFlat"])
	if num(skill["healPct"]) > 0 {
		amount := math.Max(1, math.Round(num(l.Stats["maxHp"])*num(skill["healPct"])*num(l.Stats["healPct"])))
		s["hp"] = math.Min(num(l.Stats["maxHp"]), num(s["hp"])+amount)
		push(l, M{"kind": "heal", "amount": amount, "x": s["x"], "y": s["y"], "entityId": id})
	}
	victims := []M{}
	if num(skill["coefficient"]) > 0 {
		if truth(skill["aoe"]) {
			for _, m := range inst.Monsters {
				if truth(m["alive"]) && dist(s, m) <= num(skill["range"])+num(a.def("monsters", str(m["templateId"]))["radius"]) {
					victims = append(victims, m)
				}
			}
		} else if truth(target["alive"]) {
			victims = append(victims, target)
		}
	}
	for _, m := range victims {
		for _, k := range []string{"taunt", "slow", "shred", "vulnerable"} {
			if num(skill[k+"Ms"]) > 0 {
				m[k+"Until"] = t + num(skill[k+"Ms"])
				if k == "taunt" {
					m["targetId"] = id
				}
			}
		}
		a.hurtMonster(inst, l, m, num(skill["coefficient"]), str(skill["element"]), truth(skill["forceCrit"]))
	}
	if num(skill["coefficient"]) > 0 || num(skill["healPct"]) > 0 {
		s["lastCombatAt"] = t
	}
}

// capture 校验契约目标、残血比例、时段与距离，消耗灵契球后由服务器掷出成功率。
func (a *App) capture(id string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "助手模式下不能捕捉")
	i := a.instances[l.InstanceKey]
	check(i != nil, "先进入世界")
	m := i.Monsters[str(l.State["targetId"])]
	check(truth(m["alive"]), "先选中目标")
	d := a.def("monsters", str(m["templateId"]))
	check(str(d["petId"]) != "" && d["captureBase"] != nil, "它不会回应契约")
	hp := num(m["hp"]) / num(m["maxHp"])
	check(hp <= .3, "打到三成血以下再用灵契球")
	check(d["captureTime"] == nil || contains(d["captureTime"], a.period), "这个时辰它不会停下")
	check(dist(l.State, m) <= 90, "再靠近一点")
	a.consumeTemplate(id, "capture_orb", 1)
	chance := clamp(num(d["captureBase"])+(.3-hp)*.4+num(obj(d["captureWeather"])[str(a.weather["id"])]), .06, .92)
	if rand.Float64() > chance {
		a.toast(id, "契约失败，灵契球碎了")
		a.notify(id)
		push(l, M{"kind": "text", "text": "契约失败", "x": m["x"], "y": m["y"]})
		return
	}
	m["alive"], m["hp"], m["deadAt"], m["respawnAt"] = false, 0, now(), 0
	if num(d["respawnMs"]) > 0 {
		m["respawnAt"] = now() + num(d["respawnMs"])
	}
	a.onMonsterGone(str(m["id"]))
	a.givePet(id, str(d["petId"]), true)
	push(l, M{"kind": "text", "text": "契约成功", "x": m["x"], "y": m["y"]})
}

// gather 检查资源点冷却，发放材料并推进采集熟练与任务进度。
func (a *App) gather(l *Live, i *Instance, n M) {
	t := now()
	check(t >= num(i.Nodes[str(n["id"])]), "还要等一会儿")
	i.Nodes[str(n["id"])] = t + num(n["respawnMs"])
	count := 1.
	id := str(l.State["id"])
	if p := a.one("SELECT template_id FROM player_pets WHERE player_id=? AND active=1", id); p != nil {
		d := obj(a.def("pets", str(p["template_id"]))["dispatch"])
		bonus := (num(d[str(n["kind"])]) + num(d["versatile"])) * .35
		if rand.Float64() < bonus {
			count++
		}
	}
	a.addItem(id, str(n["itemId"]), count, nil, "bag")
	a.trackExp(id, "life:gather", 8, false)
	a.emit(id, M{"type": "gather", "nodeKind": n["kind"]})
	a.toast(id, fmt.Sprintf("采集到 %s x%g", str(a.def("items", str(n["itemId"]))["name"]), count))
	a.notify(id)
}

// interact 从附近 NPC、资源点和传送门中选取目标；指定 focus 时只匹配该互动对象。
func (a *App) interact(id, focus string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能互动")
	m := a.def("maps", str(l.State["mapId"]))
	inst := a.instances[l.InstanceKey]
	if inst == nil {
		return
	}
	type option struct {
		d    float64
		kind string
		v    M
	}
	options := []option{}
	for _, n := range objects(a.data["npcs"]) {
		if focus != "" && focus != "npc:"+str(n["id"]) {
			continue
		}
		d, ok := a.bridgeGap(str(l.State["mapId"]), l.State, str(n["mapId"]), n)
		if ok && d <= 72 {
			options = append(options, option{d, "npc", n})
		}
	}
	for _, n := range objects(m["nodes"]) {
		if focus != "" && focus != "node:"+str(n["id"]) {
			continue
		}
		d := dist(l.State, n)
		if d <= 48 {
			options = append(options, option{d, "node", n})
		}
	}
	for _, p := range objects(m["portals"]) {
		key := fmt.Sprintf("portal:%g,%g", num(p["x"]), num(p["y"]))
		if focus != "" && focus != key {
			continue
		}
		d := dist(l.State, p)
		if d <= 58 {
			options = append(options, option{d, "portal", p})
		}
	}
	if len(options) == 0 {
		if focus != "" {
			fail("再走近一点")
		}
		fail("附近没有可以互动的东西")
	}
	best := options[0]
	for _, o := range options[1:] {
		if o.d < best.d {
			best = o
		}
	}
	halt(l, best.kind == "npc")
	switch best.kind {
	case "npc":
		a.send(l, merge(a.dialog(id, str(best.v["id"])), M{"type": "dialog"}))
	case "node":
		a.gather(l, inst, best.v)
	case "portal":
		to := str(best.v["to"])
		if str(a.def("maps", to)["kind"]) == "dungeon" {
			a.enter(l, to, true)
		} else {
			a.cross(l, to, true)
		}
	}
}

// socketMessage 分发客户端意图消息；客户端提交输入，数值变化由服务器计算。
func (a *App) socketMessage(id string, message M) {
	l := a.mustLive(id)
	s := l.State
	switch str(message["type"]) {
	case "input":
		if truth(s["safe"]) {
			return
		}
		s["inputX"], s["inputY"] = clamp(num(message["x"]), -1, 1), clamp(num(message["y"]), -1, 1)
		if message["destX"] != nil && message["destY"] != nil {
			s["destX"], s["destY"] = num(message["destX"]), num(message["destY"])
		}
	case "target":
		if !truth(s["safe"]) {
			if str(s["targetId"]) != str(message["id"]) {
				s["beats"] = 0
			}
			s["targetId"] = message["id"]
		}
	case "mode":
		s["safe"] = str(message["mode"]) == "assistant"
		if truth(s["safe"]) {
			halt(l, true)
		}
	case "skill":
		if str(s["boutId"]) != "" {
			a.actBout(id, M{"action": "skill", "slot": message["slot"]})
		} else {
			a.cast(id, message["slot"])
		}
	case "guard":
		if str(s["boutId"]) != "" {
			a.actBout(id, M{"action": "defend"})
		} else {
			a.guard(id)
		}
	case "engage":
		a.engage(id)
	case "bout":
		a.actBout(id, message)
	case "capture":
		a.capture(id)
	case "interact":
		a.interact(id, str(message["focus"]))
	case "chat":
		a.postChat(id, fallback(message["channel"], "world"), strings.TrimSpace(str(message["text"])))
	}
}
