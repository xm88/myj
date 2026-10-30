// 世界模拟：地图分线、队伍副本、移动碰撞、怪物 AI、伤害掉落和每100ms快照广播。

package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// Live 角色在世界中的临时状态，关联战斗面板、冷却、事件和当前连接。
type Live struct {
	// State 保存协议字段与在线动作状态；数据库列在连接时转换成这些字段。
	State M
	// Stats 是最终战斗面板，装备、增益、等级或灵伴改变时重新计算。
	Stats M
	// CD 保存技能的下次可用毫秒时间，快照中换算成剩余时长。
	CD M
	// Events 是本帧待发飘字与提示，发送快照后清空。
	Events []M
	// Peer 指向当前有效连接；新连接接管后旧连接的命令会被忽略。
	Peer *Peer
	// InstanceKey 将角色定位到公开分线或队伍独立副本。
	InstanceKey string
}

// Instance 地图公开分线或队伍副本，独立维护怪物、角色、采集点与稀有刷新计时。
type Instance struct {
	// Key 唯一标识实例，MapID 标识所使用的地图模板。
	Key, MapID string
	// 公开地图从一号线开始；副本使用零号线并按队伍归属隔离。
	Line    int
	Dungeon bool
	// 怪物与角色的场内状态互不跨线共享。
	Monsters map[string]M
	Players  map[string]bool
	// Nodes 按资源点 ID 保存下次可采集时间，RareAt 控制下一轮稀有检查。
	Nodes  M
	RareAt float64
	// EmptySince 记录分线变空的时间。野外空闲超过一段时间后卸掉，避免没人时继续跑怪物 AI。
	EmptySince float64
}

// mustLive 获取世界中的在线角色状态，不存在时提示先进入世界。
func (a *App) mustLive(id string) *Live {
	l := a.live[id]
	check(l != nil, "先进入世界")
	return l
}

// halt 清除方向输入和点击目的地，可选地取消当前攻击目标。
func halt(l *Live, clear bool) {
	for _, k := range []string{"inputX", "inputY"} {
		l.State[k] = 0
	}
	l.State["destX"], l.State["destY"] = nil, nil
	if clear {
		l.State["targetId"] = nil
	}
}

// attach 恢复角色世界状态或接管已有连接，加入分线后发送天气与时段。
func (a *App) attach(id string, p *Peer) {
	l := a.live[id]
	if l == nil {
		r := a.player(id)
		s := M{"id": id, "userId": r["user_id"], "name": r["name"], "level": r["level"], "exp": r["exp"], "gold": r["gold"], "mapId": r["map_id"], "x": r["x"], "y": r["y"], "hp": r["hp"], "mp": r["mp"], "weaponType": r["weapon_type"], "loadout": arr(parse(str(r["skill_loadout"]))), "buffs": objects(parse(str(r["buffs"]))), "safe": false, "armed": a.hasWeapon(id), "inputX": 0, "inputY": 0, "destX": nil, "destY": nil, "targetId": nil, "shield": 0, "invulnUntil": 0, "attackAt": 0, "lastCombatAt": 0, "chatAt": 0, "disconnectAt": 0, "eventSeq": 1}
		stats := a.stats(id, num(r["level"]), str(r["weapon_type"]), s["buffs"])
		s["hp"] = clamp(num(s["hp"]), 1, num(stats["maxHp"]))
		s["mp"] = clamp(num(s["mp"]), 0, num(stats["maxMp"]))
		l = &Live{State: s, Stats: stats, CD: M{}, Events: []M{}, Peer: p}
		a.live[id] = l
		a.enter(l, str(s["mapId"]), false)
	} else {
		old := l.Peer
		l.Peer = p
		l.State["safe"] = false
		l.State["disconnectAt"] = 0
		if old != nil {
			old.closeWith(closeReplaced)
		}
		if a.instances[l.InstanceKey] == nil {
			a.enter(l, str(l.State["mapId"]), false)
		}
	}
	a.resumeStall(l)
	a.send(l, M{"type": "world", "weather": a.def("weathers", str(a.weather["id"])), "period": a.period, "periodName": periodLabels[a.period]})
}

// detach 只处理当前连接的断开；停止移动、切换安全状态并保存角色。
func (a *App) detach(id string, p *Peer) {
	l := a.live[id]
	if l == nil || l.Peer != p {
		return
	}
	l.Peer = nil
	l.State["disconnectAt"] = now()
	l.State["safe"] = true
	halt(l, false)
	if str(l.State["tradeId"]) != "" {
		a.cancelTrade(str(l.State["id"]))
	}
	if str(l.State["boutId"]) != "" {
		a.forfeitBout(str(l.State["id"]))
	}
	a.save(l)
}

// enter 检查地图门槛并选择公开分线或队伍副本，更新坐标、进入地图任务和存档。
func (a *App) enter(l *Live, mapID string, spawn bool) {
	m := a.def("maps", mapID)
	check(m != nil, "没有这张地图")
	check(num(l.State["level"]) >= num(m["unlockLevel"]), fmt.Sprintf("需要 %g 级", num(m["unlockLevel"])))
	id := str(l.State["id"])
	if old := a.instances[l.InstanceKey]; old != nil {
		delete(old.Players, id)
	}
	// 公开地图按容量分线；独立副本按队长标识共享，同队进入同一份实例。
	dungeon := str(m["kind"]) == "dungeon"
	key := ""
	if dungeon {
		key = "dungeon:" + mapID + ":" + a.partyKey(id)
	} else {
		line := math.MaxInt
		for _, inst := range a.instances {
			if inst.MapID == mapID && !inst.Dungeon && len(inst.Players) < int(a.constant("lineCap")) && inst.Line < line {
				key, line = inst.Key, inst.Line
			}
		}
		if key == "" {
			for i := 1; ; i++ {
				candidate := fmt.Sprintf("%s#%d", mapID, i)
				if a.instances[candidate] == nil {
					key = candidate
					break
				}
			}
		}
	}
	inst := a.instances[key]
	if inst == nil {
		line := 0
		if !dungeon {
			parts := strings.Split(key, "#")
			line, _ = strconv.Atoi(parts[len(parts)-1])
		}
		inst = &Instance{Key: key, MapID: mapID, Line: line, Dungeon: dungeon, Monsters: map[string]M{}, Players: map[string]bool{}, Nodes: M{}, RareAt: now() + 20000}
		for _, s := range objects(m["spawns"]) {
			a.spawnMonster(inst, str(s["monsterId"]), num(s["x"]), num(s["y"]))
		}
		a.instances[key] = inst
	}
	l.InstanceKey = key
	l.State["mapId"] = mapID
	halt(l, true)
	if spawn {
		pos := obj(m["spawn"])
		l.State["x"], l.State["y"] = pos["x"], pos["y"]
	}
	l.State["x"] = clamp(num(l.State["x"]), 20, num(m["width"])-20)
	l.State["y"] = clamp(num(l.State["y"]), 20, num(m["height"])-20)
	if spawn && str(m["kind"]) == "town" {
		l.State["hp"], l.State["mp"] = l.Stats["maxHp"], l.Stats["maxMp"]
	}
	inst.Players[id] = true
	inst.EmptySince = 0
	a.emit(id, M{"type": "enter", "mapId": mapID})
	a.save(l)
}

// spawnMonster 按模板生成怪物实例，初始化出生点、血量、冷却与控制状态。
func (a *App) spawnMonster(i *Instance, tpl string, x, y float64) {
	d := a.def("monsters", tpl)
	if d == nil {
		return
	}
	id := uid()
	i.Monsters[id] = M{"id": id, "templateId": tpl, "name": d["name"], "x": x, "y": y, "homeX": x, "homeY": y, "hp": d["hp"], "maxHp": d["hp"], "alive": true, "respawnAt": 0, "deadAt": 0, "attackAt": 0, "targetId": nil, "tauntUntil": 0, "slowUntil": 0, "shredUntil": 0, "vulnerableUntil": 0, "heavyCount": 0}
}

// travel 校验航标使用地点和目的地图，城镇可直接前往解锁地图。
func (a *App) travel(id, dest string) {
	l := a.mustLive(id)
	m := a.def("maps", str(l.State["mapId"]))
	check(a.def("maps", dest) != nil, "没有这张地图")
	check(str(m["kind"]) != "wild", "野外要走传送门，或者先回星落镇")
	check(str(m["kind"]) != "dungeon" || dest == "town_starfall", "先离开遗迹")
	a.enter(l, dest, true)
}

// blocked 判断角色或怪物圆形碰撞范围是否与地图阻挡矩形相交。
func blocked(m M, x, y, r float64) bool {
	for _, b := range objects(m["blocks"]) {
		if x+r > num(b["x"]) && x-r < num(b["x"])+num(b["w"]) && y+r > num(b["y"]) && y-r < num(b["y"])+num(b["h"]) {
			return true
		}
	}
	return false
}

// slide 分别尝试横向和纵向位移，使实体遇到阻挡时可以沿边缘滑动。
func slide(m M, x, y, dx, dy, r float64) (float64, float64) {
	nx, ny := clamp(x+dx, r, num(m["width"])-r), clamp(y+dy, r, num(m["height"])-r)
	if blocked(m, nx, y, r) {
		nx = x
	}
	if blocked(m, nx, ny, r) {
		ny = y
	}
	return nx, ny
}

// move 将方向或目的地换算为本帧位移，限制对角速度并应用服务器碰撞判断。
func move(m M, l *Live, dt float64) {
	s := l.State
	if truth(s["safe"]) || str(s["stallId"]) != "" || str(s["boutId"]) != "" || str(s["tradeId"]) != "" {
		return
	}
	ax, ay := num(s["inputX"]), num(s["inputY"])
	len := math.Hypot(ax, ay)
	// 对角输入归一化，防止同时按两方向使移动速度超过面板值。
	if len > 1 {
		ax /= len
		ay /= len
	}
	// 键盘方向优先于点击目的地，主动输入会取消自动移动。
	if len > .08 {
		s["destX"], s["destY"] = nil, nil
	} else if s["destX"] != nil && s["destY"] != nil {
		dx, dy := num(s["destX"])-num(s["x"]), num(s["destY"])-num(s["y"])
		d := math.Hypot(dx, dy)
		if d < 8 {
			s["destX"], s["destY"] = nil, nil
			ax, ay = 0, 0
		} else {
			ax, ay = dx/d, dy/d
		}
	}
	x, y := slide(m, num(s["x"]), num(s["y"]), ax*num(l.Stats["speed"])*dt, ay*num(l.Stats["speed"])*dt, 14)
	s["x"], s["y"] = x, y
}

// push 给角色加入带递增编号的战斗事件，只保留最近三十条。
func push(l *Live, ev M) {
	e := clone(ev)
	e["id"] = num(l.State["eventSeq"])
	l.State["eventSeq"] = num(l.State["eventSeq"]) + 1
	l.Events = append(l.Events, e)
	if len(l.Events) > 30 {
		l.Events = l.Events[len(l.Events)-30:]
	}
}

// tell 向当前分线中的所有角色加入同一战斗事件。
func (a *App) tell(i *Instance, ev M) {
	for id := range i.Players {
		if l := a.live[id]; l != nil {
			push(l, ev)
		}
	}
}

// hurtMonster 由服务器计算角色对怪物的伤害，记录熟练与事件，血量耗尽时结算击杀。
func (a *App) hurtMonster(i *Instance, l *Live, m M, coeff float64, element string, force bool) {
	if !truth(m["alive"]) {
		return
	}
	d := a.def("monsters", str(m["templateId"]))
	t := now()
	def := num(d["def"])
	if num(m["shredUntil"]) > t {
		def *= .72
	}
	taken := 1.
	if num(m["vulnerableUntil"]) > t {
		taken = 1.15
	}
	attack := num(l.Stats["atk"])
	// 灵力只附加到元素技能，物理普攻仍使用纯攻击属性。
	if element != "physical" {
		attack += num(l.Stats["spell"])
	}
	amount, crit := damage(attack, coeff, def, num(obj(l.Stats["elem"])[element]), num(obj(d["elemRes"])[element]), num(l.Stats["critRate"]), num(l.Stats["critDmg"]), taken, force, rand.Float64)
	m["hp"] = num(m["hp"]) - amount
	l.State["lastCombatAt"] = t
	if str(m["targetId"]) == "" {
		m["targetId"] = l.State["id"]
	}
	a.tell(i, M{"kind": "dmg", "amount": amount, "crit": crit, "x": m["x"], "y": m["y"], "entityId": m["id"]})
	id := str(l.State["id"])
	if str(d["role"]) == "world" {
		a.emit(id, M{"type": "world_boss"})
	}
	a.trackExp(id, "weapon:"+str(l.State["weaponType"]), 1, true)
	if num(m["hp"]) <= 0 {
		a.killMonster(i, l, m)
	}
}

// hurtPlayer 计算怪物伤害、天气倍率、闪避与护盾；倒地后送回当前地图入口。返回是否倒下。
func (a *App) hurtPlayer(i *Instance, m M, l *Live, coeff float64) bool {
	s := l.State
	t := now()
	if truth(s["safe"]) || num(s["invulnUntil"]) > t {
		a.tell(i, M{"kind": "miss", "text": "闪避", "x": s["x"], "y": s["y"], "entityId": s["id"]})
		return false
	}
	d := a.def("monsters", str(m["templateId"]))
	attack := num(d["atk"])
	weather := str(a.weather["id"])
	if str(d["element"]) == "thunder" && weather == "storm" {
		attack *= 1.2
	}
	if truth(d["nightPower"]) && (a.period == "night" || a.period == "late") {
		attack *= 1.25
	}
	if weather == "blood_moon" {
		attack *= 1.08
	}
	amount, crit := damage(attack, coeff, num(l.Stats["def"]), 0, num(obj(l.Stats["elemRes"])[str(d["element"])]), .05, 1.5, 1, false, rand.Float64)
	guarded := num(s["guardUntil"]) > t
	if guarded {
		amount = math.Max(1, math.Round(amount*0.55))
	}
	// 先由护盾吸收，再把剩余伤害扣到角色生命。
	absorbed := math.Min(num(s["shield"]), amount)
	s["shield"] = num(s["shield"]) - absorbed
	amount -= absorbed
	s["hp"] = num(s["hp"]) - amount
	s["lastCombatAt"] = t
	a.tell(i, M{"kind": "dmg", "amount": amount, "crit": crit, "guarded": guarded, "x": s["x"], "y": s["y"], "entityId": s["id"]})
	// 倒地后回到本地图入口，清理战斗增益并短暂无敌，怪物清除对该角色的仇恨。
	if num(s["hp"]) <= 0 {
		a.downPlayer(i, l)
		return true
	}
	return false
}

// downPlayer 把倒下的角色送回当前地图入口，并让盯着他的怪物散去。
func (a *App) downPlayer(i *Instance, l *Live) {
	s := l.State
	spawn := obj(a.def("maps", str(s["mapId"]))["spawn"])
	s["hp"] = math.Max(1, math.Floor(num(l.Stats["maxHp"])*.35))
	s["x"], s["y"] = spawn["x"], spawn["y"]
	halt(l, true)
	s["shield"] = 0
	s["buffs"] = []M{}
	s["invulnUntil"] = now() + 4000
	if i != nil {
		for _, other := range i.Monsters {
			if other["targetId"] == s["id"] {
				other["targetId"] = nil
				other["x"], other["y"], other["hp"] = other["homeX"], other["homeY"], other["maxHp"]
			}
		}
	}
	a.refresh(str(s["id"]))
	a.toast(str(s["id"]), "你被击倒了，退回到入口")
	push(l, M{"kind": "text", "text": "被击倒", "x": s["x"], "y": s["y"]})
}

// rollQuality 按固定品质或配置权重决定装备掉落品质。
func rollQuality(d M) string {
	if str(d["quality"]) != "" {
		return str(d["quality"])
	}
	total := 0.
	for _, q := range objects(d["qualities"]) {
		total += num(q["weight"])
	}
	cursor := rand.Float64() * total
	for _, q := range objects(d["qualities"]) {
		cursor -= num(q["weight"])
		if cursor <= 0 {
			return str(q["quality"])
		}
	}
	return "white"
}

// killMonster 标记怪物死亡与重生时间，结算经验、金币、队伍分享、掉落和击杀任务。
func (a *App) killMonster(i *Instance, l *Live, m M) {
	d := a.def("monsters", str(m["templateId"]))
	m["alive"], m["hp"], m["deadAt"], m["respawnAt"] = false, 0, now(), 0
	if num(d["respawnMs"]) > 0 {
		m["respawnAt"] = now() + num(d["respawnMs"])
	}
	id := str(l.State["id"])
	if l.State["targetId"] == m["id"] {
		l.State["targetId"] = nil
	}
	a.addExp(id, num(d["exp"]))
	a.addGold(id, randInt(num(d["goldMin"]), num(d["goldMax"])))
	if p := a.one("SELECT id FROM player_pets WHERE player_id=? AND active=1", id); p != nil {
		a.petExp(str(p["id"]), 8)
	}
	// 同分线且足够接近的队友分享一半经验，金币和物品仍属于击杀角色。
	if party := a.parties[a.playerParty[id]]; party != nil {
		for _, v := range arr(party["memberIds"]) {
			pid := str(v)
			mate := a.live[pid]
			if pid != id && mate != nil && mate.InstanceKey == l.InstanceKey && dist(l.State, mate.State) <= 800 {
				a.addExp(pid, math.Round(num(d["exp"])*.5))
			}
		}
	}
	// 每项掉落独立判断概率；天气可提高指定掉落率，装备另掷品质和词条。
	for _, drop := range objects(d["drops"]) {
		chance := num(drop["chance"])
		if drop["weatherBoost"] == a.weather["id"] {
			chance = math.Min(.95, chance*2)
		}
		if rand.Float64() > chance {
			continue
		}
		count := math.Max(1, math.Round(num(drop["min"])+rand.Float64()*(num(drop["max"])-num(drop["min"]))))
		tpl := a.def("items", str(drop["itemId"]))
		if tpl == nil {
			continue
		}
		var ex M
		if str(tpl["kind"]) == "equipment" {
			count = 1
			ex = a.equipmentExtra(rollQuality(drop))
		}
		a.addItem(id, str(drop["itemId"]), count, ex, "bag")
		push(l, M{"kind": "loot", "text": fmt.Sprintf("获得 %s x%g", str(tpl["name"]), count), "x": m["x"], "y": m["y"]})
	}
	a.emit(id, M{"type": "kill", "monsterId": d["id"]})
	a.tell(i, M{"kind": "text", "text": str(d["name"]) + " 被击败", "x": m["x"], "y": m["y"]})
	a.notify(id)
	a.onMonsterGone(str(m["id"]))
}

// think 处理怪物仇恨、嘲讽、追击、回位和普攻，不攻击安全模式角色。
func (a *App) think(i *Instance, mp, m M, t, dt float64) {
	d := a.def("monsters", str(m["templateId"]))
	target := a.live[str(m["targetId"])]
	taunted := num(m["tauntUntil"]) > t && target != nil && !truth(target.State["safe"]) && target.InstanceKey == i.Key
	if !taunted && (target == nil || truth(target.State["safe"]) || target.InstanceKey != i.Key || dist(m, target.State) > num(d["leash"])) {
		m["targetId"] = nil
		best := num(d["aggro"])
		for id := range i.Players {
			p := a.live[id]
			if p == nil || truth(p.State["safe"]) || str(p.State["boutId"]) != "" {
				continue
			}
			gap := dist(m, p.State)
			if gap < best {
				best = gap
				m["targetId"] = id
			}
		}
		for _, other := range a.instances {
			if other.Key == i.Key || other.Dungeon {
				continue
			}
			if _, _, linked := a.link(i.MapID, other.MapID); !linked {
				continue
			}
			for id := range other.Players {
				p := a.live[id]
				if p == nil || truth(p.State["safe"]) || str(p.State["boutId"]) != "" {
					continue
				}
				gap, ok := a.bridgeGap(i.MapID, m, other.MapID, p.State)
				if ok && gap < best {
					best = gap
					m["targetId"] = id
				}
			}
		}
	}
	if str(m["boutId"]) != "" {
		return
	}
	p := a.live[str(m["targetId"])]
	if p != nil && p.InstanceKey != i.Key {
		gap, ok := a.bridgeGap(i.MapID, m, str(p.State["mapId"]), p.State)
		if ok && gap <= num(d["range"])+num(d["radius"]) && t >= num(m["attackAt"]) {
			m["attackAt"] = t + num(d["interval"])
			a.hurtPlayer(a.instances[p.InstanceKey], m, p, 1)
		}
		return
	}
	x, y := num(m["x"]), num(m["y"])
	home := M{"x": m["homeX"], "y": m["homeY"]}
	if p == nil || truth(p.State["safe"]) || p.InstanceKey != i.Key {
		gap := dist(m, home)
		if gap > 8 {
			nx, ny := slide(mp, x, y, (num(home["x"])-x)/gap*70*dt, (num(home["y"])-y)/gap*70*dt, num(d["radius"]))
			m["x"], m["y"] = nx, ny
		}
		if gap < 12 {
			m["hp"] = m["maxHp"]
		}
		return
	}
	if dist(m, home) > num(d["leash"])+40 {
		m["x"], m["y"], m["hp"], m["targetId"] = home["x"], home["y"], m["maxHp"], nil
		return
	}
	gap := dist(m, p.State)
	slow := 1.
	if num(m["slowUntil"]) > t {
		slow = .55
	}
	if gap > num(d["range"])*.85 {
		nx, ny := slide(mp, x, y, (num(p.State["x"])-x)/gap*90*slow*dt, (num(p.State["y"])-y)/gap*90*slow*dt, num(d["radius"]))
		m["x"], m["y"] = nx, ny
	} else if t >= num(m["attackAt"]) {
		m["attackAt"] = t + num(d["interval"])
		m["heavyCount"] = num(m["heavyCount"]) + 1
		coeff := 1.
		if num(d["heavyEvery"]) > 0 && math.Mod(num(m["heavyCount"]), num(d["heavyEvery"])) == 0 {
			coeff = 1.8
		}
		a.hurtPlayer(i, m, p, coeff)
	}
}

// rares 检查天气、时段与场内数量，再按概率刷新稀有怪物。
func (a *App) rares(i *Instance, mp M) {
	for _, r := range objects(mp["rares"]) {
		if r["weather"] != nil && !contains(r["weather"], str(a.weather["id"])) {
			continue
		}
		if r["time"] != nil && !contains(r["time"], a.period) {
			continue
		}
		alive := 0.
		for _, m := range i.Monsters {
			if truth(m["alive"]) && m["templateId"] == r["monsterId"] {
				alive++
			}
		}
		if alive >= num(r["max"]) {
			continue
		}
		chance := num(r["chance"])
		if str(r["monsterId"]) == "gold_slime" && str(a.weather["id"]) == "star_tide" {
			chance *= 1.8
		}
		if rand.Float64() <= chance {
			a.spawnMonster(i, str(r["monsterId"]), num(r["x"]), num(r["y"]))
		}
	}
}

// tick 每100ms更新天气、离线保留、分线移动与战斗，最后广播快照并清空已发送事件。
func (a *App) tick() {
	t := now()
	worldChanged := false
	a.tickBouts(t)
	if t >= num(a.weather["until"]) {
		list := objects(a.data["weathers"])
		index := 0
		for i, d := range list {
			if d["id"] == a.weather["id"] {
				index = i
				break
			}
		}
		a.weather = M{"id": list[(index+1)%len(list)]["id"], "until": t + a.constant("weatherMs")}
		a.run("UPDATE world_state SET value=? WHERE key='weather'", js(a.weather))
		worldChanged = true
	}
	period := dayPeriod(time.Now())
	if period != a.period {
		a.period = period
		worldChanged = true
	}
	if worldChanged {
		a.broadcast(M{"type": "world", "weather": a.def("weathers", str(a.weather["id"])), "period": a.period, "periodName": periodLabels[a.period]})
	}
	// 断线角色保持六十秒安全状态，超时后保存并从分线移除。
	for id, l := range a.live {
		if l.Peer == nil && num(l.State["disconnectAt"]) > 0 && t-num(l.State["disconnectAt"]) > 60000 {
			a.save(l)
			if i := a.instances[l.InstanceKey]; i != nil {
				delete(i.Players, id)
			}
			delete(a.live, id)
		}
	}
	// 每条分线依次处理重生、稀有刷新、玩家移动恢复与双方自动攻击。
	for key, i := range a.instances {
		mp := a.def("maps", i.MapID)
		for id, m := range i.Monsters {
			if truth(m["alive"]) {
				continue
			}
			if num(m["respawnAt"]) > 0 && t >= num(m["respawnAt"]) {
				d := a.def("monsters", str(m["templateId"]))
				m["alive"], m["hp"], m["x"], m["y"], m["targetId"], m["respawnAt"] = true, d["hp"], m["homeX"], m["homeY"], nil, 0
			} else if num(m["respawnAt"]) == 0 && t-num(m["deadAt"]) > 1600 {
				delete(i.Monsters, id)
			}
		}
		if t >= i.RareAt {
			i.RareAt = t + 45000
			a.rares(i, mp)
		}
		for id := range i.Players {
			l := a.live[id]
			if l == nil {
				continue
			}
			buffs := []M{}
			expired := false
			for _, b := range objects(l.State["buffs"]) {
				if num(b["until"]) > t {
					buffs = append(buffs, b)
				} else {
					expired = true
				}
			}
			if expired {
				l.State["buffs"] = buffs
				a.refresh(id)
			}
			move(mp, l, .1)
			a.maybeCross(l)
			if l.InstanceKey != i.Key {
				continue
			}
			s := l.State
			s["mp"] = math.Min(num(l.Stats["maxMp"]), num(s["mp"])+.4)
			if t-num(s["lastCombatAt"]) > 6000 {
				s["hp"] = math.Min(num(l.Stats["maxHp"]), num(s["hp"])+num(l.Stats["maxHp"])*.0025)
			}
			m := i.Monsters[str(s["targetId"])]
			if str(s["boutId"]) != "" {
				// 回合战斗里不再按拍子自动挥砍，出手由回合动作决定。
			} else if m == nil || !truth(m["alive"]) {
				s["targetId"] = nil
				s["beats"] = 0
			} else if !truth(s["safe"]) && t >= num(s["attackAt"]) {
				if num(s["guardUntil"]) > t {
					s["attackAt"] = num(s["guardUntil"])
				} else {
					d := a.def("monsters", str(m["templateId"]))
					if dist(s, m) <= num(l.Stats["range"])+num(d["radius"]) {
						s["attackAt"] = t + num(l.Stats["attackInterval"])
						s["beats"] = num(s["beats"]) + 1
						a.hurtMonster(i, l, m, 1, "physical", false)
					}
				}
			}
		}
		for _, m := range i.Monsters {
			if truth(m["alive"]) {
				a.think(i, mp, m, t, .1)
			}
		}
		if len(i.Players) == 0 {
			if i.EmptySince == 0 {
				i.EmptySince = t
			}
			// 副本没人立刻拆。野外分线再空转 90 秒就卸掉，下次有人进来会按配置重新刷怪。
			if i.Dungeon || t-i.EmptySince > 90000 {
				delete(a.instances, key)
			}
		} else {
			i.EmptySince = 0
		}
	}
	// 所有分线结算完毕才发送快照，事件随快照发送后清空。
	for _, l := range a.live {
		if l.Peer != nil {
			a.send(l, a.snapshot(l, t))
			l.Events = []M{}
		}
	}
}

// snapshot 输出当前分线实体、自身战斗状态和剩余冷却，供客户端渲染。
func (a *App) snapshot(l *Live, t float64) M {
	entities := []M{}
	line := 1
	i := a.instances[l.InstanceKey]
	if i != nil {
		line = i.Line
		for _, m := range i.Monsters {
			if !truth(m["alive"]) && t-num(m["deadAt"]) > 1500 {
				continue
			}
			d := a.def("monsters", str(m["templateId"]))
			entities = append(entities, M{"id": m["id"], "kind": "monster", "templateId": m["templateId"], "name": m["name"], "x": m["x"], "y": m["y"], "hp": math.Max(0, num(m["hp"])), "maxHp": m["maxHp"], "alive": m["alive"], "tint": d["tint"], "role": d["role"], "radius": d["radius"], "capturable": str(d["petId"]) != ""})
		}
		for id := range i.Players {
			p := a.live[id]
			if p == nil || p == l {
				continue
			}
			s := p.State
			entities = append(entities, M{"id": id, "kind": "player", "templateId": s["weaponType"], "name": s["name"], "level": s["level"], "x": s["x"], "y": s["y"], "hp": s["hp"], "maxHp": p.Stats["maxHp"], "alive": true, "tint": "#f2d48a", "role": "player", "radius": 14, "stalling": str(s["stallId"]) != ""})
		}
		for _, stall := range a.rows("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.open=1 AND s.map_id=?", i.MapID) {
			entities = append(entities, M{"id": "stall:" + str(stall["id"]), "kind": "stall", "stallId": stall["id"], "name": stall["title"], "owner": stall["owner_name"], "x": stall["x"], "y": stall["y"], "alive": true})
		}
	}
	entities = append(entities, a.afarEntities(l)...)
	seen := map[string]bool{}
	for _, e := range entities {
		seen[str(e["id"])] = true
	}
	for _, e := range a.seamEntities(l) {
		if seen[str(e["id"])] {
			continue
		}
		entities = append(entities, e)
	}
	mapID := str(l.State["mapId"])
	for _, e := range entities {
		if e["worldX"] != nil {
			continue
		}
		if e["mapId"] == nil {
			e["mapId"] = mapID
		}
		mid := mapID
		if !truth(e["seam"]) && str(e["mapId"]) != "" {
			mid = str(e["mapId"])
		}
		wx, wy := a.worldPos(mid, num(e["x"]), num(e["y"]))
		e["worldX"], e["worldY"] = wx, wy
	}
	self := M{}
	for _, k := range []string{"x", "y", "hp", "mp", "level", "exp", "gold", "targetId", "shield", "weaponType", "safe"} {
		self[k] = l.State[k]
	}
	self["maxHp"], self["maxMp"], self["expMax"] = l.Stats["maxHp"], l.Stats["maxMp"], expNext(num(l.State["level"]))
	self["swing"] = math.Max(0, num(l.State["attackAt"])-t)
	self["guard"] = math.Max(0, num(l.State["guardUntil"])-t)
	self["interval"] = num(l.Stats["attackInterval"])
	self["speed"] = num(l.Stats["speed"])
	self["beats"] = num(l.State["beats"])
	wx, wy := a.worldPos(str(l.State["mapId"]), num(l.State["x"]), num(l.State["y"]))
	self["worldX"], self["worldY"] = wx, wy
	if ox, oy, ok := a.originOf(str(l.State["mapId"])); ok {
		self["originX"], self["originY"] = ox, oy
	}
	cd := M{}
	for k, v := range l.CD {
		cd[k] = math.Max(0, num(v)-t)
	}
	snap := M{"type": "snapshot", "t": t, "mapId": l.State["mapId"], "line": line, "self": self, "entities": entities, "cooldowns": cd, "events": l.Events, "gates": a.gates(str(l.State["mapId"])), "chunks": a.cameraChunks(str(l.State["mapId"])), "camera": cameraSpan}
	if b := a.bouts[str(l.State["boutId"])]; b != nil {
		snap["bout"] = a.boutView(b, str(l.State["id"]))
	}
	return snap
}
