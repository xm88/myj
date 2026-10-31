// 物品与面板：背包堆叠、装备实例、仓库、消耗品、词条及最终战斗属性。

package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
)

// temperCap 一件装备最多淬炼的层数。失败不掉层，也不毁掉装备。
const temperCap = 6

// temperOf 读取装备上已经淬上的层数。旧档没有这个字段时是 0。
func temperOf(ex M) int {
	n := int(math.Floor(num(ex["temper"])))
	if n < 0 {
		return 0
	}
	if n > temperCap {
		return temperCap
	}
	return n
}

// temperNeed 当前层数再淬一层要的火砂和金币。层数越高，砂和钱都多一点。
func temperNeed(level int) (float64, float64) {
	if level < 0 {
		level = 0
	}
	return float64(1 + level/2), float64(8 + level*6)
}

// temperChance 成功率随层数下降，制造熟练每级拉回 2%。夹在 28% 到 94%。
func temperChance(level int, craft float64) float64 {
	return clamp(0.9-float64(level)*0.1+(craft-1)*0.02, 0.28, 0.94)
}

// temperSteps 按这一抽决定升几层。大成且还有空位时一次升两层，到顶就停。
func temperSteps(level, cap int, hit, spark bool) int {
	if !hit || level >= cap {
		return 0
	}
	gain := 1
	if spark && level+1 < cap {
		gain = 2
	}
	if level+gain > cap {
		return cap - level
	}
	return gain
}

// equipmentExtra 按装备品质抽取不重复词条，并生成适合存档的随机数值。
func (a *App) equipmentExtra(q string) M {
	pool := append([]M{}, objects(a.data["affixes"])...)
	out := []M{}
	for i := 0; i < qualityAffixes[q] && len(pool) > 0; i++ {
		j := rand.IntN(len(pool))
		d := pool[j]
		pool = append(pool[:j], pool[j+1:]...)
		v := num(d["min"]) + rand.Float64()*(num(d["max"])-num(d["min"]))
		stat := str(d["stat"])
		if stat == "critRate" || stat == "critDmg" || strings.HasPrefix(stat, "elem_") {
			v = math.Round(v*1000) / 1000
		} else {
			v = math.Max(1, math.Round(v))
		}
		out = append(out, M{"id": d["id"], "name": d["name"], "stat": stat, "value": v})
	}
	return M{"quality": q, "affixes": out}
}

// extra 解析装备品质与词条；旧存档缺失字段时按普通装备处理。
func extra(raw any) M {
	m := obj(parse(str(raw)))
	if m["quality"] == nil {
		m["quality"] = "white"
	}
	if m["affixes"] == nil {
		m["affixes"] = []M{}
	}
	return m
}

// items 读取角色物品，可按背包、仓库或装备位置筛选。
func (a *App) items(id, location string) []M {
	if location != "" {
		return a.rows("SELECT * FROM player_items WHERE player_id=? AND location=?", id, location)
	}
	return a.rows("SELECT * FROM player_items WHERE player_id=?", id)
}

// slots 统计某个存放位置占用的物品行数，用于容量判断。
func (a *App) slots(id, loc string) float64 {
	return num(a.one("SELECT COUNT(*) AS n FROM player_items WHERE player_id=? AND location=?", id, loc)["n"])
}

// countItem 统计背包中普通堆叠材料的数量，不计算装备实例和仓库物品。
func (a *App) countItem(id, tpl string) float64 {
	return num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id=? AND location='bag' AND extra IS NULL", id, tpl)["n"])
}

// mail 写入带可选附件的邮件，并通知客户端刷新档案。
func (a *App) mail(id, title, body string, attachments M) {
	var raw any
	if attachments != nil {
		raw = js(attachments)
	}
	a.run("INSERT INTO mail(id,player_id,title,body,attachments,is_read,claimed,created_at) VALUES(?,?,?,?,?,0,0,?)", uid(), id, title, body, raw, now())
	a.notify(id)
}

// addItem 材料合并堆叠、装备逐件建实例；空间不足时将剩余奖励寄到邮箱。
func (a *App) addItem(id, tplID string, count float64, ex M, loc string) {
	tpl := a.def("items", tplID)
	if tpl == nil || count <= 0 {
		return
	}
	count = math.Floor(count)
	cap := a.constant("bagCap")
	if loc == "bank" {
		cap = a.constant("bankCap")
	}
	instanced := str(tpl["kind"]) == "equipment" || ex != nil
	overflow := func(n float64) {
		a.mail(id, "背包满了", str(tpl["name"])+" 先寄存在邮箱。", M{"items": []M{{"templateId": tplID, "count": n, "extra": ex}}})
	}
	// 装备与带词条物品必须逐件保存，材料则按同模板和存放位置合并堆叠。
	if instanced {
		for i := 0.; i < count; i++ {
			if loc != "equipped" && a.slots(id, loc) >= cap {
				overflow(count - i)
				return
			}
			itemExtra := ex
			if itemExtra == nil {
				itemExtra = a.equipmentExtra("white")
			}
			a.run("INSERT INTO player_items(id,player_id,template_id,count,location,extra,created_at) VALUES(?,?,?,1,?,?,?)", uid(), id, tplID, loc, js(itemExtra), now())
		}
	} else {
		r := a.one("SELECT * FROM player_items WHERE player_id=? AND template_id=? AND location=? AND extra IS NULL", id, tplID, loc)
		if r != nil {
			a.run("UPDATE player_items SET count=count+? WHERE id=?", count, r["id"])
		} else if loc != "equipped" && a.slots(id, loc) >= cap {
			overflow(count)
			return
		} else {
			a.run("INSERT INTO player_items(id,player_id,template_id,count,location,extra,created_at) VALUES(?,?,?,?,?,NULL,?)", uid(), id, tplID, count, loc, now())
		}
	}
	// 只有进入背包的物品推进收集任务，仓库与已装备物品不计入。
	if loc == "bag" {
		a.emit(id, M{"type": "collect", "itemId": tplID, "count": count})
	}
}

// takeItem 按实例 ID 和角色归属读取物品，避免操作其他人的物品。
func (a *App) takeItem(id, item string) M {
	r := a.one("SELECT * FROM player_items WHERE id=? AND player_id=?", item, id)
	check(r != nil, "没有这件物品")
	return r
}

// consume 扣除已校验的物品数量，全部耗尽时删除对应实例。
func (a *App) consume(r M, count float64) {
	check(count > 0 && count <= num(r["count"]), "数量不对")
	if count == num(r["count"]) {
		a.run("DELETE FROM player_items WHERE id=?", r["id"])
	} else {
		a.run("UPDATE player_items SET count=count-? WHERE id=?", count, r["id"])
	}
}

// consumeTemplate 按材料模板扣除背包堆叠数量，材料不足时终止业务操作。
func (a *App) consumeTemplate(id, tpl string, count float64) {
	r := a.one("SELECT * FROM player_items WHERE player_id=? AND template_id=? AND location='bag' AND extra IS NULL", id, tpl)
	check(r != nil && num(r["count"]) >= count, fallback(a.def("items", tpl)["name"], "材料")+"不够")
	a.consume(r, count)
}

// equip 替换同部位装备；换武器时同时更新技能栏并重算角色面板。
func (a *App) equip(id, item string) {
	r := a.takeItem(id, item)
	tpl := a.def("items", str(r["template_id"]))
	check(str(r["location"]) == "bag" || str(r["location"]) == "equipped", "先从仓库取出来")
	check(str(tpl["kind"]) == "equipment" && str(tpl["slot"]) != "", "这件不能装备")
	for _, old := range a.items(id, "equipped") {
		if a.def("items", str(old["template_id"]))["slot"] == tpl["slot"] {
			a.run("UPDATE player_items SET location='bag' WHERE id=?", old["id"])
		}
	}
	a.run("UPDATE player_items SET location='equipped' WHERE id=?", item)
	if str(tpl["slot"]) == "weapon" {
		w := str(tpl["weaponType"])
		load := a.defaultLoadout(w)
		a.run("UPDATE players SET weapon_type=?,skill_loadout=?,updated_at=? WHERE id=?", w, js(load), now(), id)
		if l := a.live[id]; l != nil {
			l.State["weaponType"] = w
			l.State["loadout"] = load
		}
	}
	a.refresh(id)
	a.emit(id, M{"type": "equip", "quality": extra(r["extra"])["quality"]})
	a.notify(id)
}

// unequip 确认背包有空间后卸下指定部位装备，再重算面板。
func (a *App) unequip(id, slot string) {
	check(a.slots(id, "bag") < a.constant("bagCap"), "背包满了")
	var found M
	for _, r := range a.items(id, "equipped") {
		if str(a.def("items", str(r["template_id"]))["slot"]) == slot {
			found = r
			break
		}
	}
	check(found != nil, "这个部位是空的")
	a.run("UPDATE player_items SET location='bag' WHERE id=?", found["id"])
	a.refresh(id)
	a.notify(id)
}

// storage 在背包与仓库间移动物品，装备保持实例，材料允许部分转移。
func (a *App) storage(id, item, direction string, count float64) {
	check(direction == "deposit" || direction == "withdraw", "方向不对")
	r := a.takeItem(id, item)
	from, to := "bag", "bank"
	if direction == "withdraw" {
		from, to = "bank", "bag"
	}
	check(str(r["location"]) == from, "物品不在这边")
	tpl := a.def("items", str(r["template_id"]))
	check(tpl != nil, "物品不存在")
	check(!truth(tpl["bound"]), "这件不能放进仓库")
	cap := a.constant("bagCap")
	if to == "bank" {
		cap = a.constant("bankCap")
	}
	check(count > 0, "数量不对")
	if str(tpl["kind"]) == "equipment" || r["extra"] != nil {
		check(a.slots(id, to) < cap, "背包或仓库满了")
		a.run("UPDATE player_items SET location=? WHERE id=?", to, item)
	} else {
		n := math.Min(math.Floor(count), num(r["count"]))
		existing := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id=? AND location=? AND extra IS NULL", id, r["template_id"], to)
		check(existing != nil || a.slots(id, to) < cap, "背包或仓库满了")
		a.consume(r, n)
		a.addItem(id, str(r["template_id"]), n, nil, to)
	}
	a.emit(id, M{"type": "bank"})
	a.notify(id)
}

// useItem 使用消耗品并结算恢复、增益或灵伴奖励，最后消耗物品。
func (a *App) useItem(id, item string) {
	r := a.takeItem(id, item)
	tpl := a.def("items", str(r["template_id"]))
	check(str(r["location"]) == "bag", "只能使用背包里的东西")
	check(str(tpl["kind"]) == "consumable", "这件不能直接使用")
	check(str(tpl["id"]) != "capture_orb", "把目标打到残血后，用捕捉键契约")
	check(str(tpl["id"]) != "beast_grain" && str(tpl["id"]) != "vigor_fruit" && str(tpl["id"]) != "beast_seal", "在灵兽面板里喂给它，或给召唤兽刻印")
	effect := obj(tpl["effect"])
	if str(effect["grantPet"]) != "" {
		a.consume(r, 1)
		a.givePet(id, str(effect["grantPet"]), false)
		a.notify(id)
		return
	}
	if l := a.live[id]; l != nil {
		l.State["hp"] = math.Min(num(l.Stats["maxHp"]), num(l.State["hp"])+num(effect["healHp"]))
		l.State["mp"] = math.Min(num(l.Stats["maxMp"]), num(l.State["mp"])+num(effect["healMp"]))
		if b := obj(effect["buff"]); b["stat"] != nil {
			a.addBuff(l, str(tpl["id"]), b)
			a.refresh(id)
		}
	} else {
		a.run("UPDATE players SET hp=MIN(hp+?,9999) WHERE id=?", num(effect["healHp"]), id)
	}
	a.consume(r, 1)
	a.notify(id)
}

// hasWeapon 检查已装备物品中是否存在武器部位，用于技能与徒手面板判定。
func (a *App) hasWeapon(id string) bool {
	for _, r := range a.items(id, "equipped") {
		if str(a.def("items", str(r["template_id"]))["slot"]) == "weapon" {
			return true
		}
	}
	return false
}

// stats 依次叠加等级、武器风格、熟练、装备、灵伴、有效增益与种族属性。
func (a *App) stats(id string, level float64, weapon string, buffs any) M {
	styles := map[string]M{"sword_shield": {"interval": 1100, "range": 64, "def": 4, "hp": 12}, "greatsword": {"interval": 1450, "range": 72, "atkPct": .08}, "staff": {"interval": 1200, "range": 220, "mp": 22, "atkPct": -.04}, "bow": {"interval": 900, "range": 250, "crit": .04}}
	style := styles[weapon]
	if style == nil {
		style = styles["sword_shield"]
	}
	elem, elemRes := M{}, M{}
	for _, k := range []string{"physical", "fire", "water", "thunder", "wind", "earth"} {
		elem[k] = 0
		elemRes[k] = 0
	}
	s := M{"maxHp": 90 + level*14 + num(style["hp"]), "maxMp": 40 + level*4 + num(style["mp"]), "atk": (8 + level*2) * (1 + num(style["atkPct"])), "spell": 0, "def": 4 + level*1.2 + num(style["def"]), "critRate": .05 + num(style["crit"]), "critDmg": 1.5, "speed": 170, "attackInterval": style["interval"], "range": style["range"], "healPct": 1, "elem": elem, "elemRes": elemRes}
	// 先应用武器熟练，再叠加装备；该顺序与旧版面板保持一致。
	s["atk"] = num(s["atk"]) * (1 + (num(a.track(id, "weapon:"+weapon)["level"])-1)*.01)
	armed := false
	for _, r := range a.items(id, "equipped") {
		tpl := a.def("items", str(r["template_id"]))
		base := obj(tpl["base"])
		if len(base) == 0 {
			continue
		}
		if str(tpl["slot"]) == "weapon" {
			armed = true
		}
		ex := extra(r["extra"])
		mul := qualityMul[str(ex["quality"])]
		if mul == 0 {
			mul = 1
		}
		temper := temperOf(ex)
		for _, k := range []string{"hp", "mp", "atk", "def", "critRate", "critDmg", "speed"} {
			target := k
			if k == "hp" {
				target = "maxHp"
			}
			if k == "mp" {
				target = "maxMp"
			}
			v := num(base[k]) * mul
			if k == "hp" || k == "mp" || k == "speed" {
				v = math.Round(v)
			}
			s[target] = num(s[target]) + v
			// 淬炼只抬攻击、防御、生命和法力，按品质后的基础值每层加 8%。没淬过的装备和以前一样。
			if temper > 0 && (k == "atk" || k == "def" || k == "hp" || k == "mp") && num(base[k]) != 0 {
				bonus := num(base[k]) * mul * 0.08 * float64(temper)
				if k == "hp" || k == "mp" {
					bonus = math.Round(bonus)
				}
				s[target] = num(s[target]) + bonus
			}
		}
		for _, aff := range objects(ex["affixes"]) {
			k := str(aff["stat"])
			v := num(aff["value"])
			if strings.HasPrefix(k, "elem_") {
				elem[strings.TrimPrefix(k, "elem_")] = num(elem[strings.TrimPrefix(k, "elem_")]) + v
			} else {
				if k == "hp" {
					k = "maxHp"
				}
				if k == "mp" {
					k = "maxMp"
				}
				s[k] = num(s[k]) + v
			}
		}
	}
	// 未装备武器时按徒手规则降低攻击并调整射程和出手间隔。
	if !armed {
		s["atk"] = num(s["atk"]) * .55
		s["range"] = 48
		s["attackInterval"] = 1300
	}
	// 主战灵伴的光环随灵伴等级缩放；正在派遣的灵伴不会处于主战状态。
	if p := a.one("SELECT template_id,level FROM player_pets WHERE player_id=? AND active=1", id); p != nil {
		d := a.def("pets", str(p["template_id"]))
		au := obj(d["aura"])
		scale := 1 + (num(p["level"])-1)*.04
		s["critRate"] = num(s["critRate"]) + num(au["critRate"])*scale
		s["atk"] = num(s["atk"]) * (1 + num(au["atkPct"])*scale)
		s["def"] = num(s["def"]) + num(au["def"])*scale
		s["maxHp"] = num(s["maxHp"]) + num(au["hp"])*scale
		s["healPct"] = num(s["healPct"]) + num(au["healPct"])*scale
		for k, v := range obj(au["elem"]) {
			elem[k] = num(elem[k]) + num(v)*scale
		}
	}
	// 临时增益只在有效期内参与面板，过期后由世界循环重新计算。
	for _, b := range objects(buffs) {
		if num(b["until"]) <= now() {
			continue
		}
		v := num(b["value"])
		switch str(b["stat"]) {
		case "atkPct":
			s["atk"] = num(s["atk"]) * (1 + v)
		case "defPct":
			s["def"] = num(s["def"]) * (1 + v)
		case "speedPct":
			s["speed"] = num(s["speed"]) * (1 + v)
		case "critRate":
			s["critRate"] = num(s["critRate"]) + v
		}
	}
	p := a.player(id)
	// 种族自动成长与自行加点在最后叠加，随后按旧版显示精度舍入。
	attrBonus(s, totalAttrs(level, parseAttrs(str(p["attributes"])), str(p["race"])), str(p["race"]))
	for _, k := range []string{"maxHp", "maxMp", "speed"} {
		s[k] = math.Round(num(s[k]))
	}
	s["atk"] = math.Round(num(s["atk"])*10) / 10
	s["def"] = math.Round(num(s["def"])*10) / 10
	s["critRate"] = math.Round(num(s["critRate"])*1000) / 1000
	return s
}

// refresh 重算在线角色面板，保留受伤状态并将血量、法力限制在新上限内。
func (a *App) refresh(id string) {
	l := a.live[id]
	if l == nil {
		return
	}
	s := a.stats(id, num(l.State["level"]), str(l.State["weaponType"]), l.State["buffs"])
	ratio := num(l.State["hp"]) / num(l.Stats["maxHp"])
	l.Stats = s
	if ratio >= .99 {
		l.State["hp"] = s["maxHp"]
	} else {
		l.State["hp"] = clamp(num(l.State["hp"]), 1, num(s["maxHp"]))
	}
	l.State["mp"] = clamp(num(l.State["mp"]), 0, num(s["maxMp"]))
	l.State["armed"] = a.hasWeapon(id)
}

// addBuff 替换同来源的增益，按服务器时间计算新的失效时间。
func (a *App) addBuff(l *Live, id string, b M) {
	out := []M{}
	for _, old := range objects(l.State["buffs"]) {
		if str(old["id"]) != id {
			out = append(out, old)
		}
	}
	out = append(out, M{"id": id, "stat": b["stat"], "value": b["value"], "until": now() + num(b["ms"])})
	l.State["buffs"] = out
}

// temper 给背包或身上的装备淬一层。火砂和金币先扣；失败装备还在，层数不变。
func (a *App) temper(id, item string) {
	r := a.takeItem(id, item)
	tpl := a.def("items", str(r["template_id"]))
	check(tpl != nil && str(tpl["kind"]) == "equipment", "只有装备能淬炼")
	loc := str(r["location"])
	check(loc == "bag" || loc == "equipped", "先从仓库取出来")
	ex := extra(r["extra"])
	level := temperOf(ex)
	check(level < temperCap, "这件已经淬满了")
	grit, gold := temperNeed(level)
	craft := num(a.track(id, "life:craft")["level"])
	a.addGold(id, -gold)
	a.consumeTemplate(id, "temper_grit", grit)
	a.trackExp(id, "life:craft", 6+float64(level)*2, false)
	gain := temperSteps(level, temperCap, rand.Float64() < temperChance(level, craft), rand.Float64() < 0.12)
	name := str(tpl["name"])
	if gain == 0 {
		a.toast(id, name+" 火候差了一点，火砂散了。装备还在，层数没变。")
		a.notify(id)
		return
	}
	ex["temper"] = level + gain
	a.run("UPDATE player_items SET extra=? WHERE id=?", js(ex), item)
	if loc == "equipped" {
		a.refresh(id)
	}
	if gain >= 2 {
		a.toast(id, fmt.Sprintf("%s 火候正好，一次淬到 %d 层。", name, level+gain))
	} else {
		a.toast(id, fmt.Sprintf("%s 淬到 %d 层。攻击、防御和生命按这一层抬高。", name, level+gain))
	}
	a.notify(id)
}

// viewItem 把数据库物品与模板、品质、词条合并成网页可用的物品视图。
func (a *App) viewItem(r M, craft float64) M {
	t := a.def("items", str(r["template_id"]))
	ex := M{}
	var q any
	aff := []M{}
	if r["extra"] != nil {
		ex = extra(r["extra"])
		q = ex["quality"]
		aff = objects(ex["affixes"])
	}
	view := M{"id": r["id"], "templateId": r["template_id"], "name": fallback(t["name"], str(r["template_id"])), "desc": fallback(t["desc"], ""), "kind": fallback(t["kind"], "material"), "count": r["count"], "location": r["location"], "slot": t["slot"], "weaponType": t["weaponType"], "bound": truth(t["bound"]), "sell": num(t["sell"]), "quality": q, "affixes": aff, "lines": gearLines(t, ex)}
	if str(t["kind"]) == "equipment" {
		level := temperOf(ex)
		view["temper"] = level
		view["temperMax"] = temperCap
		if level < temperCap {
			grit, gold := temperNeed(level)
			view["temperGrit"] = grit
			view["temperGold"] = gold
			view["temperChance"] = math.Round(temperChance(level, craft) * 100)
		}
	}
	return view
}

// gearLines 把装备基础属性和词条整理成可对比的行。品质只乘基础值，词条按原值另列。
func gearLines(tpl, ex M) []M {
	lines := []M{}
	base := obj(tpl["base"])
	mul := qualityMul[str(ex["quality"])]
	if mul == 0 {
		mul = 1
	}
	labels := map[string]string{"atk": "攻击", "def": "防御", "hp": "生命", "mp": "法力", "speed": "速度", "critRate": "暴击", "critDmg": "暴伤"}
	temper := temperOf(ex)
	for _, k := range []string{"atk", "def", "hp", "mp", "speed", "critRate", "critDmg"} {
		v := math.Round(num(base[k])*mul*10) / 10
		if temper > 0 && (k == "atk" || k == "def" || k == "hp" || k == "mp") && num(base[k]) != 0 {
			bonus := num(base[k]) * mul * 0.08 * float64(temper)
			if k == "hp" || k == "mp" {
				bonus = math.Round(bonus)
			} else {
				bonus = math.Round(bonus*10) / 10
			}
			v = math.Round((v+bonus)*10) / 10
		}
		if v == 0 {
			continue
		}
		lines = append(lines, M{"stat": k, "name": labels[k], "value": v})
	}
	for _, aff := range objects(ex["affixes"]) {
		lines = append(lines, M{"stat": aff["stat"], "name": aff["name"], "value": aff["value"], "affix": true})
	}
	return lines
}

// fmtAmount 将属性贡献显示为至多一位小数，沿用旧版界面的舍入规则。
func fmtAmount(v float64) string { return fmt.Sprintf("%g", math.Round(v*10)/10) }
