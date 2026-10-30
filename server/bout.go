// 回合战斗：进战后按速度排出手顺序。攻击、技能、物品、防御、逃跑各占一回合。
// 状态按回合衰减。没有开战时，原来的拍子普攻仍然有效。

package main

import (
	"encoding/json"
	"math"
	"sort"
)

// RoundEffect 是挂在某个参战者身上、按回合倒数的状态。
type RoundEffect struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rounds int     `json:"rounds"`
	DotPct float64 `json:"dotPct"`
	Taken  float64 `json:"taken"`
}

// BoutActor 是速度条上的一个出手位。
type BoutActor struct {
	ID    string  `json:"id"`
	Kind  string  `json:"kind"`
	Name  string  `json:"name"`
	Speed float64 `json:"speed"`
}

// Bout 一场独立的回合战斗。怪物可以在相邻分线，用 Home 找回它所在的实例。
type Bout struct {
	ID        string                   `json:"id"`
	Round     int                      `json:"round"`
	Index     int                      `json:"index"`
	Phase     string                   `json:"phase"`
	Await     string                   `json:"await"`
	WaitSince float64                  `json:"waitSince"`
	Ended     float64                  `json:"ended"`
	Leader    string                   `json:"leader"`
	Order     []BoutActor              `json:"order"`
	Players   map[string]bool          `json:"players"`
	Monsters  map[string]bool          `json:"monsters"`
	Home      map[string]string        `json:"home"`
	Down      map[string]bool          `json:"down"`
	Queued    map[string]M             `json:"queued"`
	Effects   map[string][]RoundEffect `json:"effects"`
	Log       []string                 `json:"log"`
}

func cloneBout(b *Bout) *Bout {
	if b == nil {
		return nil
	}
	var c Bout
	must(json.Unmarshal([]byte(js(b)), &c))
	if c.Players == nil {
		c.Players = map[string]bool{}
	}
	if c.Monsters == nil {
		c.Monsters = map[string]bool{}
	}
	if c.Home == nil {
		c.Home = map[string]string{}
	}
	if c.Down == nil {
		c.Down = map[string]bool{}
	}
	if c.Queued == nil {
		c.Queued = map[string]M{}
	}
	if c.Effects == nil {
		c.Effects = map[string][]RoundEffect{}
	}
	return &c
}

// engage 把当前目标和附近的同伴、怪物拉进同一场回合战斗。
func (a *App) engage(id string) {
	l := a.mustLive(id)
	s := l.State
	check(!truth(s["safe"]), "助手模式下不能开战")
	check(str(s["stallId"]) == "", "收摊之后才能打")
	check(str(s["tradeId"]) == "", "先把交易结束")
	check(str(s["boutId"]) == "", "已经在战斗中")
	check(str(s["targetId"]) != "", "先选中一个目标")
	inst, mob := a.locateMonster(str(s["targetId"]))
	check(inst != nil && mob != nil && truth(mob["alive"]), "目标已经不在了")
	check(a.boutBy[str(mob["id"])] == "", "它已经在别的战斗里")
	gap, ok := a.bridgeGap(str(s["mapId"]), s, inst.MapID, mob)
	reach := num(l.Stats["range"]) + num(a.def("monsters", str(mob["templateId"]))["radius"]) + 8
	check(ok && gap <= reach, "距离不够")

	b := &Bout{
		ID: uid(), Round: 1, Phase: "choose",
		Players: map[string]bool{}, Monsters: map[string]bool{}, Home: map[string]string{},
		Down: map[string]bool{}, Queued: map[string]M{}, Effects: map[string][]RoundEffect{},
	}
	a.addBoutPlayer(b, l)
	if party := a.parties[a.playerParty[id]]; party != nil {
		for _, v := range arr(party["memberIds"]) {
			pid := str(v)
			mate := a.live[pid]
			if pid == id || mate == nil || mate.InstanceKey != l.InstanceKey || truth(mate.State["safe"]) {
				continue
			}
			if str(mate.State["boutId"]) != "" || str(mate.State["stallId"]) != "" {
				continue
			}
			if dist(s, mate.State) > 480 || len(b.Players) >= 4 {
				continue
			}
			a.addBoutPlayer(b, mate)
		}
	}
	a.addBoutMonster(b, inst, mob)
	for _, other := range inst.Monsters {
		if len(b.Monsters) >= 4 || other["id"] == mob["id"] || !truth(other["alive"]) || a.boutBy[str(other["id"])] != "" {
			continue
		}
		if dist(mob, other) <= 150 {
			a.addBoutMonster(b, inst, other)
		}
	}
	b.Leader = id
	b.Order = a.boutOrder(b)
	a.bouts[b.ID] = b
	halt(l, false)
	a.note(b, "第 1 回合开始")
	a.advance(b)
}

func (a *App) addBoutPlayer(b *Bout, l *Live) {
	id := str(l.State["id"])
	b.Players[id] = true
	b.Home[id] = l.InstanceKey
	l.State["boutId"] = b.ID
	a.boutBy[id] = b.ID
	halt(l, false)
}

func (a *App) addBoutMonster(b *Bout, inst *Instance, m M) {
	id := str(m["id"])
	b.Monsters[id] = true
	b.Home[id] = inst.Key
	m["boutId"] = b.ID
	a.boutBy[id] = b.ID
}

func (a *App) boutOrder(b *Bout) []BoutActor {
	order := []BoutActor{}
	for id := range b.Players {
		l := a.live[id]
		speed := 100.
		name := id
		if l != nil {
			speed = num(l.Stats["speed"])
			name = str(l.State["name"])
		}
		order = append(order, BoutActor{ID: id, Kind: "player", Name: name, Speed: speed})
	}
	for id := range b.Monsters {
		inst := a.instances[b.Home[id]]
		m := M{}
		if inst != nil {
			m = inst.Monsters[id]
		}
		d := a.def("monsters", str(m["templateId"]))
		iv := num(d["interval"])
		if iv < 400 {
			iv = 400
		}
		order = append(order, BoutActor{ID: id, Kind: "monster", Name: str(m["name"]), Speed: 90 + 80000/iv})
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].Speed == order[j].Speed {
			return order[i].ID < order[j].ID
		}
		return order[i].Speed > order[j].Speed
	})
	return order
}

// actBout 提交当前回合的动作。还没轮到本人时拒绝。
func (a *App) actBout(id string, msg M) {
	l := a.mustLive(id)
	b := a.bouts[str(l.State["boutId"])]
	check(b != nil && b.Phase == "choose", "这场战斗已经结束")
	check(b.Await == id, "还没轮到你")
	action := str(msg["action"])
	switch action {
	case "attack", "skill", "item", "defend", "flee":
	default:
		fail("没有这个动作")
	}
	if b.Queued == nil {
		b.Queued = map[string]M{}
	}
	b.Queued[id] = msg
	b.Await = ""
	a.advance(b)
}

// advance 连续结算怪物和已经提交的动作，直到轮到某个人选择。
func (a *App) advance(b *Bout) {
	for n := 0; n < 16; n++ {
		if b == nil || b.Phase != "choose" {
			return
		}
		if !a.anyMonsterAlive(b) {
			a.finishBout(b, "victory")
			return
		}
		if !a.anyPlayerUp(b) {
			a.finishBout(b, "defeat")
			return
		}
		if b.Index >= len(b.Order) {
			a.endRound(b)
			if b.Phase != "choose" {
				return
			}
			b.Index = 0
			b.Round++
			a.note(b, "第 "+fmtInt(b.Round)+" 回合")
			continue
		}
		actor := b.Order[b.Index]
		if !a.actorUp(b, actor) {
			b.Index++
			continue
		}
		if actor.Kind == "monster" {
			a.monsterTurn(b, actor)
			b.Index++
			continue
		}
		if q := b.Queued[actor.ID]; q != nil {
			delete(b.Queued, actor.ID)
			a.playerTurn(b, actor.ID, q)
			b.Index++
			continue
		}
		// 被拉进这场的队友立刻由同伴 AI 出手。开战的人仍自己选。
		if b.Leader != "" && actor.ID != b.Leader {
			a.playerTurn(b, actor.ID, a.companionAction(b, actor.ID))
			b.Index++
			continue
		}
		b.Await = actor.ID
		b.WaitSince = now()
		return
	}
}

func (a *App) actorUp(b *Bout, actor BoutActor) bool {
	if actor.Kind == "player" {
		return b.Players[actor.ID] && !b.Down[actor.ID] && a.live[actor.ID] != nil
	}
	m := a.boutMonster(b, actor.ID)
	return m != nil && truth(m["alive"])
}

func (a *App) anyMonsterAlive(b *Bout) bool {
	for id := range b.Monsters {
		if m := a.boutMonster(b, id); m != nil && truth(m["alive"]) {
			return true
		}
	}
	return false
}

func (a *App) anyPlayerUp(b *Bout) bool {
	for id := range b.Players {
		if !b.Down[id] && a.live[id] != nil {
			return true
		}
	}
	return false
}

func (a *App) boutMonster(b *Bout, id string) M {
	inst := a.instances[b.Home[id]]
	if inst == nil {
		return nil
	}
	return inst.Monsters[id]
}

func (a *App) playerTurn(b *Bout, id string, msg M) {
	l := a.live[id]
	if l == nil {
		return
	}
	switch str(msg["action"]) {
	case "defend":
		l.State["guardUntil"] = now() + 30000
		a.addEffect(b, id, RoundEffect{ID: "guard", Name: "守势", Rounds: 1, Taken: 0.55})
		a.note(b, str(l.State["name"])+" 守势")
		push(l, M{"kind": "text", "text": "守势", "x": l.State["x"], "y": l.State["y"], "entityId": id})
	case "flee":
		if a.tryFlee(b, l) {
			return
		}
	case "item":
		a.useItem(id, str(msg["itemId"]))
		a.note(b, str(l.State["name"])+" 使用了物品")
	case "skill":
		a.boutSkill(b, l, num(msg["slot"]))
	default:
		a.boutAttack(b, l)
	}
	if b.Phase == "choose" && !a.anyMonsterAlive(b) {
		a.finishBout(b, "victory")
	}
}

func (a *App) boutAttack(b *Bout, l *Live) {
	m, mid := a.firstFoe(b)
	if m == nil {
		return
	}
	inst := a.instances[b.Home[mid]]
	a.hurtMonster(inst, l, m, 1, "physical", false)
	a.note(b, str(l.State["name"])+" 攻击 "+str(m["name"]))
	l.State["beats"] = num(l.State["beats"]) + 1
	l.State["attackAt"] = now() + num(l.Stats["attackInterval"])
}

func (a *App) boutSkill(b *Bout, l *Live, slot float64) {
	s := l.State
	sid := ""
	if slot == 0 {
		fail("先选技能")
	}
	i := int(slot) - 1
	loadout := arr(s["loadout"])
	if i >= 0 && i < len(loadout) {
		sid = str(loadout[i])
	}
	skill := a.def("skills", sid)
	check(skill != nil, "没有这个技能")
	t := now()
	check(num(l.CD[sid]) <= t, "技能还没好")
	check(num(s["mp"]) >= num(skill["mp"]), "法力不足")
	s["mp"] = math.Max(0, num(s["mp"])-num(skill["mp"]))
	l.CD[sid] = t + num(skill["cooldown"])
	if skill["buff"] != nil {
		a.addBuff(l, sid, obj(skill["buff"]))
		a.refresh(str(s["id"]))
	}
	s["shield"] = num(s["shield"]) + num(skill["shieldFlat"])
	if num(skill["healPct"]) > 0 {
		amount := math.Max(1, math.Round(num(l.Stats["maxHp"])*num(skill["healPct"])*num(l.Stats["healPct"])))
		s["hp"] = math.Min(num(l.Stats["maxHp"]), num(s["hp"])+amount)
		push(l, M{"kind": "heal", "amount": amount, "x": s["x"], "y": s["y"], "entityId": s["id"]})
	}
	rounds := func(ms float64) int {
		n := int(math.Round(ms / 2000))
		if n < 1 {
			n = 1
		}
		if n > 4 {
			n = 4
		}
		return n
	}
	victims := []M{}
	if num(skill["coefficient"]) > 0 || num(skill["tauntMs"]) > 0 {
		if truth(skill["aoe"]) {
			for mid := range b.Monsters {
				if m := a.boutMonster(b, mid); m != nil && truth(m["alive"]) {
					victims = append(victims, m)
				}
			}
		} else if _, mid := a.firstFoe(b); mid != "" {
			if m := a.boutMonster(b, mid); m != nil && truth(m["alive"]) {
				victims = append(victims, m)
			}
		}
	}
	for _, m := range victims {
		for _, k := range []string{"taunt", "slow", "shred", "vulnerable"} {
			if num(skill[k+"Ms"]) > 0 {
				m[k+"Until"] = t + num(skill[k+"Ms"])
				if k == "taunt" {
					m["targetId"] = s["id"]
				}
			}
		}
		if num(skill["tauntMs"]) > 0 {
			a.addEffect(b, str(m["id"]), RoundEffect{ID: "wound", Name: "裂伤", Rounds: rounds(num(skill["tauntMs"])), DotPct: 0.06})
		}
		switch str(skill["element"]) {
		case "fire":
			a.addEffect(b, str(m["id"]), RoundEffect{ID: "burn", Name: "灼烧", Rounds: 2, DotPct: 0.05})
		case "thunder":
			a.addEffect(b, str(m["id"]), RoundEffect{ID: "numb", Name: "麻痹", Rounds: 2, DotPct: 0.04})
		case "wind":
			a.addEffect(b, str(m["id"]), RoundEffect{ID: "gust", Name: "裂风", Rounds: 2, DotPct: 0.04})
		}
		if num(skill["coefficient"]) > 0 {
			inst := a.instances[b.Home[str(m["id"])]]
			a.hurtMonster(inst, l, m, num(skill["coefficient"]), str(skill["element"]), truth(skill["forceCrit"]))
		}
	}
	a.note(b, str(s["name"])+" 施放 "+str(skill["name"]))
	s["lastCombatAt"] = t
}

func (a *App) tryFlee(b *Bout, l *Live) bool {
	for id := range b.Monsters {
		m := a.boutMonster(b, id)
		if m == nil || !truth(m["alive"]) {
			continue
		}
		role := str(a.def("monsters", str(m["templateId"]))["role"])
		if role == "boss" || role == "world" {
			a.note(b, "对面不让你走")
			return false
		}
	}
	a.note(b, str(l.State["name"])+" 脱离了战斗")
	a.finishBout(b, "fled")
	return true
}

func (a *App) monsterTurn(b *Bout, actor BoutActor) {
	m := a.boutMonster(b, actor.ID)
	if m == nil || !truth(m["alive"]) {
		return
	}
	var foe *Live
	for id := range b.Players {
		if b.Down[id] {
			continue
		}
		if l := a.live[id]; l != nil && !truth(l.State["safe"]) {
			foe = l
			break
		}
	}
	if foe == nil {
		return
	}
	inst := a.instances[foe.InstanceKey]
	if inst == nil {
		inst = a.instances[b.Home[actor.ID]]
	}
	if a.hurtPlayer(inst, m, foe, 1) {
		b.Down[str(foe.State["id"])] = true
		a.note(b, str(foe.State["name"])+" 倒下了")
		if !a.anyPlayerUp(b) {
			a.finishBout(b, "defeat")
		}
	}
}

func (a *App) firstFoe(b *Bout) (M, string) {
	for _, actor := range b.Order {
		if actor.Kind != "monster" {
			continue
		}
		if m := a.boutMonster(b, actor.ID); m != nil && truth(m["alive"]) {
			return m, actor.ID
		}
	}
	for id := range b.Monsters {
		if m := a.boutMonster(b, id); m != nil && truth(m["alive"]) {
			return m, id
		}
	}
	return nil, ""
}

func (a *App) addEffect(b *Bout, id string, ef RoundEffect) {
	if b.Effects == nil {
		b.Effects = map[string][]RoundEffect{}
	}
	list := []RoundEffect{}
	for _, old := range b.Effects[id] {
		if old.ID != ef.ID {
			list = append(list, old)
		}
	}
	list = append(list, ef)
	b.Effects[id] = list
}

// endRound 结算持续伤害，再让状态少一回合。守势打完这一轮就卸掉。
func (a *App) endRound(b *Bout) {
	if b.Phase != "choose" {
		return
	}
	for id, list := range b.Effects {
		for _, ef := range list {
			if ef.DotPct <= 0 || ef.Rounds <= 0 {
				continue
			}
			if b.Players[id] {
				if l := a.live[id]; l != nil && !b.Down[id] {
					amount := math.Max(1, math.Round(num(l.Stats["maxHp"])*ef.DotPct))
					l.State["hp"] = num(l.State["hp"]) - amount
					push(l, M{"kind": "dmg", "amount": amount, "text": ef.Name, "x": l.State["x"], "y": l.State["y"], "entityId": id})
					if num(l.State["hp"]) <= 0 {
						if inst := a.instances[l.InstanceKey]; inst != nil {
							a.downPlayer(inst, l)
						}
						b.Down[id] = true
					}
				}
				continue
			}
			m := a.boutMonster(b, id)
			if m == nil || !truth(m["alive"]) {
				continue
			}
			amount := math.Max(1, math.Round(num(m["maxHp"])*ef.DotPct))
			m["hp"] = num(m["hp"]) - amount
			if inst := a.instances[b.Home[id]]; inst != nil {
				a.tell(inst, M{"kind": "dmg", "amount": amount, "text": ef.Name, "x": m["x"], "y": m["y"], "entityId": id})
				if num(m["hp"]) <= 0 {
					if killer := a.boutKiller(b); killer != nil {
						a.killMonster(inst, killer, m)
					}
				}
			}
		}
	}
	next := map[string][]RoundEffect{}
	for id, list := range b.Effects {
		keep := []RoundEffect{}
		for _, ef := range list {
			ef.Rounds--
			if ef.Rounds > 0 {
				keep = append(keep, ef)
			}
		}
		if len(keep) > 0 {
			next[id] = keep
		}
	}
	b.Effects = next
	for id := range b.Players {
		still := false
		for _, ef := range b.Effects[id] {
			if ef.ID == "guard" {
				still = true
			}
		}
		if !still {
			if l := a.live[id]; l != nil && num(l.State["guardUntil"]) > now() {
				l.State["guardUntil"] = 0
			}
		}
	}
	if !a.anyMonsterAlive(b) {
		a.finishBout(b, "victory")
	} else if !a.anyPlayerUp(b) {
		a.finishBout(b, "defeat")
	}
}

func (a *App) boutKiller(b *Bout) *Live {
	for id := range b.Players {
		if !b.Down[id] && a.live[id] != nil {
			return a.live[id]
		}
	}
	for id := range b.Players {
		if a.live[id] != nil {
			return a.live[id]
		}
	}
	return nil
}

func (a *App) note(b *Bout, text string) {
	if text == "" {
		return
	}
	b.Log = append(b.Log, text)
	if len(b.Log) > 16 {
		b.Log = b.Log[len(b.Log)-16:]
	}
}

func (a *App) finishBout(b *Bout, phase string) {
	if b.Phase != "choose" {
		return
	}
	b.Phase = phase
	b.Await = ""
	b.Ended = now()
	switch phase {
	case "victory":
		a.note(b, "胜利。战利品已经进背包")
	case "defeat":
		a.note(b, "战败")
	case "fled":
		a.note(b, "你从战斗里脱身了")
	}
	for id := range b.Players {
		if l := a.live[id]; l != nil {
			l.State["targetId"] = nil
			if phase == "fled" {
				push(l, M{"kind": "text", "text": "脱身", "x": l.State["x"], "y": l.State["y"], "entityId": id})
			}
		}
	}
}

// dismiss 把一场已经结束的战斗从世界里拿掉，怪物恢复原来的游走。
func (a *App) dismiss(b *Bout) {
	for id := range b.Players {
		if l := a.live[id]; l != nil && str(l.State["boutId"]) == b.ID {
			l.State["boutId"] = ""
			l.State["guardUntil"] = 0
		}
		if a.boutBy[id] == b.ID {
			delete(a.boutBy, id)
		}
	}
	for id := range b.Monsters {
		if m := a.boutMonster(b, id); m != nil && str(m["boutId"]) == b.ID {
			m["boutId"] = ""
		}
		if a.boutBy[id] == b.ID {
			delete(a.boutBy, id)
		}
	}
	delete(a.bouts, b.ID)
}

// onMonsterGone 怪物被击败或契约带走时，检查这场是不是该收了。
func (a *App) onMonsterGone(mid string) {
	b := a.bouts[a.boutBy[mid]]
	if b == nil || b.Phase != "choose" {
		return
	}
	if !a.anyMonsterAlive(b) {
		a.finishBout(b, "victory")
	}
}

// forfeitBout 断线时退出战斗。只剩自己就直接脱身，怪物不再被这场按住。
func (a *App) forfeitBout(id string) {
	b := a.bouts[a.boutBy[id]]
	if b == nil {
		return
	}
	b.Down[id] = true
	if !a.anyPlayerUp(b) {
		a.finishBout(b, "fled")
		a.dismiss(b)
		return
	}
	if b.Await == id && b.Phase == "choose" {
		b.Await = ""
		b.Index++
		a.advance(b)
	}
}

// tickBouts 长时间不操作就交给同伴 AI，结束画面保留一小会儿再拆掉。
func (a *App) tickBouts(t float64) {
	for _, b := range a.bouts {
		if b.Phase == "choose" && b.Await != "" && b.WaitSince > 0 && t-b.WaitSince > 20000 {
			b.Queued[b.Await] = a.companionAction(b, b.Await)
			b.Await = ""
			a.advance(b)
			continue
		}
		if b.Phase != "choose" && b.Ended > 0 && t-b.Ended > 1600 {
			a.dismiss(b)
		}
	}
}

// locateMonster 按实例 ID 在所有分线里找一只怪物。
func (a *App) locateMonster(id string) (*Instance, M) {
	if id == "" {
		return nil, nil
	}
	for _, inst := range a.instances {
		if m := inst.Monsters[id]; m != nil {
			return inst, m
		}
	}
	return nil, nil
}

// boutView 给当前角色的战斗快照。其他人的选择面板不会标成「轮到你」。
func (a *App) boutView(b *Bout, pid string) M {
	if b == nil {
		return nil
	}
	order := []M{}
	for _, actor := range b.Order {
		hp, maxHp := 0., 1.
		alive := false
		if actor.Kind == "player" {
			if l := a.live[actor.ID]; l != nil {
				hp, maxHp = num(l.State["hp"]), num(l.Stats["maxHp"])
				alive = !b.Down[actor.ID]
			}
		} else if m := a.boutMonster(b, actor.ID); m != nil {
			hp, maxHp = math.Max(0, num(m["hp"])), num(m["maxHp"])
			alive = truth(m["alive"])
		}
		effects := []M{}
		for _, ef := range b.Effects[actor.ID] {
			effects = append(effects, M{"id": ef.ID, "name": ef.Name, "rounds": ef.Rounds})
		}
		mark := ""
		level := 0.
		if actor.Kind == "player" {
			mark = "hero"
			if l := a.live[actor.ID]; l != nil {
				level = num(l.State["level"])
			}
		} else if m := a.boutMonster(b, actor.ID); m != nil {
			mark = str(m["templateId"])
			if d := a.def("monsters", mark); d != nil {
				level = num(d["level"])
			}
		}
		// mark、level 只给出手条画头像和小标，不参与回合结算。
		order = append(order, M{"id": actor.ID, "kind": actor.Kind, "name": actor.Name, "speed": actor.Speed, "hp": hp, "maxHp": maxHp, "alive": alive, "effects": effects, "companion": actor.Kind == "player" && b.Leader != "" && actor.ID != b.Leader, "mark": mark, "level": level})
	}
	awaitName := ""
	for _, actor := range b.Order {
		if actor.ID == b.Await {
			awaitName = actor.Name
		}
	}
	log := b.Log
	if len(log) > 8 {
		log = log[len(log)-8:]
	}
	return M{"id": b.ID, "round": b.Round, "phase": b.Phase, "yours": b.Await == pid && b.Phase == "choose", "await": b.Await, "awaitName": awaitName, "order": order, "log": log}
}
