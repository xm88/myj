// 同伴 AI：回合里除开战者以外的队友立刻选择攻击、技能、药品或防御。
// 开战的人仍自己操作。超过二十秒没选时，也用这一套，不再固定普攻。

package main

import "math"

// companionOrderName 号令标识和战斗日志里的短名。
var companionOrderName = map[string]string{"steady": "稳打", "press": "猛攻", "cover": "照应", "focus": "集火"}

// companionOrder 读取这个人当前的同伴号令。旧档没有记过时按稳打，和原来的 45% 吃药线一样。
func (a *App) companionOrder(id string) string {
	r := a.player(id)
	if r == nil {
		return "steady"
	}
	order := str(obj(parse(str(r["flags"])))["companionOrder"])
	if companionOrderName[order] == "" {
		return "steady"
	}
	return order
}

// setCompanionOrder 把号令写进角色标记。下一手同伴和超时自动都会听这个。
func (a *App) setCompanionOrder(id, order string) {
	check(companionOrderName[order] != "", "没有这种号令")
	r := a.player(id)
	flags := obj(parse(str(r["flags"])))
	flags["companionOrder"] = order
	a.run("UPDATE players SET flags=?,updated_at=? WHERE id=?", js(flags), now(), id)
	a.toast(id, "号令换成「"+companionOrderName[order]+"」")
	a.notify(id)
}

// orderHealLine 不同号令开始考虑喝药、治疗或守势的血量线。
func orderHealLine(order string) float64 {
	switch order {
	case "press":
		return 0.18
	case "cover":
		return 0.70
	default:
		return 0.45
	}
}

// companionAction 给一个参战的人挑一个合法动作。不会选逃跑，避免把整场带走。
func (a *App) companionAction(b *Bout, id string) M {
	order := a.companionOrder(id)
	msg := M{"order": order, "orderName": companionOrderName[order]}
	if order == "focus" && b != nil && b.Leader != "" {
		if lead := a.live[b.Leader]; lead != nil {
			if target := str(lead.State["targetId"]); target != "" {
				msg["focusId"] = target
			}
		}
	}
	l := a.live[id]
	if l == nil {
		msg["action"] = "attack"
		return msg
	}
	maxHp := num(l.Stats["maxHp"])
	if maxHp < 1 {
		maxHp = 1
	}
	ratio := num(l.State["hp"]) / maxHp
	if ratio < orderHealLine(order) && !hasBoutEffect(b, id, "guard") {
		if item := a.companionItem(id); item != "" {
			msg["action"] = "item"
			msg["itemId"] = item
			return msg
		}
		if slot := a.companionSlot(l, true); slot > 0 {
			msg["action"] = "skill"
			msg["slot"] = float64(slot)
			return msg
		}
		// 猛攻缺药时继续打。稳打和照应才会转守势。
		if order != "press" {
			msg["action"] = "defend"
			return msg
		}
	}
	if slot := a.companionSlot(l, false); slot > 0 {
		msg["action"] = "skill"
		msg["slot"] = float64(slot)
		return msg
	}
	msg["action"] = "attack"
	return msg
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
