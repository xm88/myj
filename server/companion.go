// 同伴 AI：回合里除开战者以外的队友立刻选择攻击、技能、药品或防御。
// 开战的人仍自己操作。超过二十秒没选时，也用这一套，不再固定普攻。

package main

import "math"

// companionAction 给一个参战的人挑一个合法动作。不会选逃跑，避免把整场带走。
func (a *App) companionAction(b *Bout, id string) M {
	l := a.live[id]
	if l == nil {
		return M{"action": "attack"}
	}
	maxHp := num(l.Stats["maxHp"])
	if maxHp < 1 {
		maxHp = 1
	}
	ratio := num(l.State["hp"]) / maxHp
	if ratio < 0.45 && !hasBoutEffect(b, id, "guard") {
		if item := a.companionItem(id); item != "" {
			return M{"action": "item", "itemId": item}
		}
		if slot := a.companionSlot(l, true); slot > 0 {
			return M{"action": "skill", "slot": float64(slot)}
		}
		return M{"action": "defend"}
	}
	if slot := a.companionSlot(l, false); slot > 0 {
		return M{"action": "skill", "slot": float64(slot)}
	}
	return M{"action": "attack"}
}

func hasBoutEffect(b *Bout, id, effect string) bool {
	if b == nil {
		return false
	}
	for _, ef := range b.Effects[id] {
		if ef.ID == effect && ef.Rounds > 0 {
			return true
		}
	}
	return false
}

// companionItem 选一瓶能回血的药。优先够补上缺口的最小剂量，否则用最强的那瓶。
func (a *App) companionItem(id string) string {
	l := a.live[id]
	missing := 1.0
	if l != nil {
		missing = math.Max(1, num(l.Stats["maxHp"])-num(l.State["hp"]))
	}
	best, bestHeal := "", 0.0
	fit, fitHeal := "", 1e18
	for _, row := range a.rows("SELECT id, template_id FROM player_items WHERE player_id=? AND location='bag' AND count>=1", id) {
		tpl := a.def("items", str(row["template_id"]))
		if tpl == nil || str(tpl["kind"]) != "consumable" || str(tpl["id"]) == "capture_orb" {
			continue
		}
		heal := num(obj(tpl["effect"])["healHp"])
		if heal <= 0 {
			continue
		}
		if heal >= missing*0.5 && heal < fitHeal {
			fit, fitHeal = str(row["id"]), heal
		}
		if heal > bestHeal {
			best, bestHeal = str(row["id"]), heal
		}
	}
	if fit != "" {
		return fit
	}
	return best
}

// companionSlot 在技能栏里挑一个冷却好、法力够的技能。heal 为真时挑治疗，否则挑系数最高的伤害技。
func (a *App) companionSlot(l *Live, heal bool) int {
	if l == nil {
		return 0
	}
	best, score := 0, 0.0
	t := now()
	mp := num(l.State["mp"])
	for i, v := range arr(l.State["loadout"]) {
		sid := str(v)
		skill := a.def("skills", sid)
		if skill == nil || num(skill["mp"]) > mp {
			continue
		}
		cd := 0.0
		if l.CD != nil {
			cd = num(l.CD[sid])
		}
		if cd > t {
			continue
		}
		value := num(skill["coefficient"])
		if heal {
			value = num(skill["healPct"])
		}
		if value > score {
			score = value
			best = i + 1
		}
	}
	return best
}
