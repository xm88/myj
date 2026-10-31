// 召唤兽与宠物共用 player_pets。召唤兽进回合单独出手；宠物给光环、可派遣，不占出手位。
// 根骨、长势、信赖、元息是旅团自己的养成词，不沿用别的游戏的资质表或技能名。

package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
)

var rootKeys = []string{"claw", "plate", "blood", "mind", "step"}

var rootNames = map[string]string{
	"claw": "爪骨", "plate": "甲骨", "blood": "血骨", "mind": "灵骨", "step": "步骨",
}

// 刻印池。兽纹印在空栏里随机学会一招。兽纹帖指定一招，栏满时可以覆掉选定的旧招。
var beastTeach = []string{"beast_pounce", "beast_shell", "beast_lick", "beast_shriek", "beast_ember", "beast_tide"}

// BoutBeast 是这一场里出战召唤兽的血量和出手资料，不写进世界怪物表。
type BoutBeast struct {
	ID       string         `json:"id"`
	Owner    string         `json:"owner"`
	Name     string         `json:"name"`
	Template string         `json:"template"`
	HP       float64        `json:"hp"`
	MaxHP    float64        `json:"maxHp"`
	Atk      float64        `json:"atk"`
	Def      float64        `json:"def"`
	Spell    float64        `json:"spell"`
	Speed    float64        `json:"speed"`
	Skills   []string       `json:"skills"`
	Alive    bool           `json:"alive"`
	Loyalty  float64        `json:"loyalty"`
	Guard    bool           `json:"guard"`
	Taunt    int            `json:"taunt"`
	Next     map[string]int `json:"next"`
}

func petRole(def M) string {
	if def != nil && str(def["role"]) == "beast" {
		return "beast"
	}
	return "pet"
}

func roleLabel(role string) string {
	if role == "beast" {
		return "召唤兽"
	}
	return "宠物"
}

func skillSlots(role string) int {
	if role == "beast" {
		return 4
	}
	return 1
}

func rootBands(rarity string) (lo, hi, glo, ghi float64) {
	switch rarity {
	case "common":
		return 22, 46, 0.90, 1.02
	case "uncommon":
		return 30, 58, 0.96, 1.08
	case "rare":
		return 40, 70, 1.02, 1.14
	case "epic":
		return 50, 80, 1.08, 1.20
	default:
		return 28, 54, 0.94, 1.06
	}
}

func unitFrom(seed string, fresh bool) float64 {
	if fresh {
		return rand.Float64()
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	return float64(h.Sum64()%10000) / 9999
}

func spanRoll(lo, hi, unit float64) float64 {
	if hi < lo {
		hi = lo
	}
	return math.Round(lo + (hi-lo)*unit)
}

// rollCompanion 掷长势和五项根骨。旧档用灵兽 id 做种子，重开不会另掷一套。
func rollCompanion(def M, seed string, fresh bool) (growth float64, roots M, skills []any, loyalty, vigor float64) {
	lo, hi, glo, ghi := rootBands(str(def["rarity"]))
	roots = M{}
	for _, k := range rootKeys {
		roots[k] = spanRoll(lo, hi, unitFrom(seed+"#"+k, fresh))
	}
	growth = math.Round((glo+(ghi-glo)*unitFrom(seed+"#g", fresh))*100) / 100
	skills = []any{}
	if sid := str(def["skillId"]); sid != "" {
		skills = append(skills, sid)
	}
	loyalty = 82
	if !fresh {
		loyalty = 68 + math.Round(24*unitFrom(seed+"#l", false))
	}
	vigor = 80
	return
}

// combatStats 把等级、长势和根骨合成这一场用的面板。40 点根骨算作中位。
func combatStats(level, growth float64, roots M) (hp, atk, def, spell, speed float64) {
	if growth <= 0 {
		growth = 1
	}
	if level < 1 {
		level = 1
	}
	scale := func(k string) float64 {
		v := num(roots[k])
		if v <= 0 {
			v = 40
		}
		return v / 40
	}
	hp = math.Round(40 + level*9*growth*scale("blood"))
	atk = math.Round((6+level*1.7)*growth*scale("claw")*10) / 10
	def = math.Round((2+level*0.7)*growth*scale("plate")*10) / 10
	spell = math.Round((4+level*1.3)*growth*scale("mind")*10) / 10
	speed = math.Round(64 + level*1.4*growth*scale("step"))
	return
}

func (a *App) migratePetColumns() {
	have := map[string]bool{}
	for _, c := range a.rows("PRAGMA table_info(player_pets)") {
		have[str(c["name"])] = true
	}
	alters := []struct{ name, ddl string }{
		{"growth", "REAL NOT NULL DEFAULT 1"},
		{"roots", "TEXT NOT NULL DEFAULT ''"},
		{"skills", "TEXT NOT NULL DEFAULT ''"},
		{"loyalty", "INTEGER NOT NULL DEFAULT 0"},
		{"vigor", "INTEGER NOT NULL DEFAULT 0"},
		{"deployed", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, col := range alters {
		if !have[col.name] {
			a.run("ALTER TABLE player_pets ADD COLUMN " + col.name + " " + col.ddl)
		}
	}
}

func (a *App) backfillPets() {
	for _, p := range a.rows("SELECT * FROM player_pets") {
		if len(obj(parse(str(p["roots"])))) == 0 {
			a.fillPetRow(p, false)
		}
	}
}

func (a *App) fillPetRow(p M, fresh bool) {
	def := a.def("pets", str(p["template_id"]))
	if def == nil {
		return
	}
	growth, roots, skills, loyalty, vigor := rollCompanion(def, str(p["id"]), fresh)
	deployed := 0.
	if !fresh && truth(p["active"]) && petRole(def) == "beast" {
		other := a.one("SELECT id FROM player_pets WHERE player_id=? AND deployed=1 AND id<>?", p["player_id"], p["id"])
		if other == nil {
			deployed = 1
		}
	}
	a.run("UPDATE player_pets SET growth=?,roots=?,skills=?,loyalty=?,vigor=?,deployed=? WHERE id=?", growth, js(roots), js(skills), loyalty, vigor, deployed, p["id"])
}

func (a *App) ensurePet(p M) M {
	if p == nil || len(obj(parse(str(p["roots"])))) > 0 {
		return p
	}
	a.fillPetRow(p, false)
	return a.one("SELECT * FROM player_pets WHERE id=?", p["id"])
}

func (a *App) beastCount(id, except string) int {
	n := 0
	for _, p := range a.rows("SELECT id, template_id FROM player_pets WHERE player_id=?", id) {
		if str(p["id"]) == except {
			continue
		}
		if petRole(a.def("pets", str(p["template_id"]))) == "beast" {
			n++
		}
	}
	return n
}

func (a *App) levelCap(def M) float64 {
	if petRole(def) == "beast" {
		return 30
	}
	return 20
}

// viewPet 把库存行收成面板用的档案。召唤兽附带这场会用到的面板。
func (a *App) viewPet(p M) M {
	p = a.ensurePet(p)
	def := a.def("pets", str(p["template_id"]))
	if def == nil {
		def = M{"name": "未知灵兽"}
	}
	role := petRole(def)
	roots := obj(parse(str(p["roots"])))
	rootRows := []M{}
	for _, k := range rootKeys {
		rootRows = append(rootRows, M{"key": k, "name": rootNames[k], "value": num(roots[k])})
	}
	skills := []M{}
	for _, id := range arr(parse(str(p["skills"]))) {
		sk := a.def("skills", str(id))
		skills = append(skills, M{"id": str(id), "name": fallback(sk["name"], str(id)), "desc": fallback(sk["desc"], "")})
	}
	hp, atk, defn, spell, speed := combatStats(num(p["level"]), num(p["growth"]), roots)
	return M{
		"id": p["id"], "templateId": p["template_id"], "name": def["name"], "rarity": def["rarity"],
		"battle": def["battle"], "work": def["work"], "desc": def["desc"],
		"level": p["level"], "exp": p["exp"], "expMax": 24 + num(p["level"])*18, "levelCap": a.levelCap(def),
		"active": truth(p["active"]), "deployed": truth(p["deployed"]), "busy": a.busy(str(p["id"])),
		"role": role, "roleName": roleLabel(role), "growth": num(p["growth"]),
		"roots": rootRows, "skills": skills, "slots": skillSlots(role),
		"loyalty": num(p["loyalty"]), "vigor": num(p["vigor"]), "vigorMax": 80,
		"mood": moodID(num(p["loyalty"])), "moodName": moodName(num(p["loyalty"])),
		"panel": M{"hp": hp, "atk": atk, "def": defn, "spell": spell, "speed": speed},
	}
}

func moodID(loyalty float64) string {
	switch {
	case loyalty >= 80:
		return "close"
	case loyalty >= 40:
		return "steady"
	case loyalty >= 15:
		return "wary"
	default:
		return "refuse"
	}
}

func moodName(loyalty float64) string {
	switch moodID(loyalty) {
	case "close":
		return "亲近"
	case "steady":
		return "寻常"
	case "wary":
		return "疏离"
	default:
		return "抗拒"
	}
}

func signed(v float64) string {
	v = math.Round(v)
	if v > 0 {
		return fmt.Sprintf("+%g", v)
	}
	return fmt.Sprintf("%g", v)
}

func (a *App) inBout(id string) bool {
	if l := a.live[id]; l != nil && str(l.State["boutId"]) != "" {
		return true
	}
	return false
}

// deployBeast 把一只召唤兽放上唯一的出战位。宠物不能占这个位。
func (a *App) deployBeast(id, pet string) {
	check(!a.inBout(id), "这场还没打完")
	p := a.ensurePet(a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", pet, id))
	check(p != nil, "没有这只灵兽")
	def := a.def("pets", str(p["template_id"]))
	check(petRole(def) == "beast", "宠物跟着走，不占出战位")
	check(!a.busy(pet), "它还在派遣途中")
	check(num(p["vigor"]) >= 1, "元息耗尽，先喂元息果或让它休息")
	check(num(p["loyalty"]) >= 15, "信赖太低，它不愿意进场")
	a.run("UPDATE player_pets SET deployed=0 WHERE player_id=?", id)
	a.run("UPDATE player_pets SET deployed=1 WHERE id=?", pet)
	a.toast(id, str(def["name"])+" 出战了")
	a.notify(id)
}

// restBeast 让出战召唤兽退到身后。光环位不动。
func (a *App) restBeast(id, pet string) {
	check(!a.inBout(id), "这场还没打完")
	p := a.one("SELECT id FROM player_pets WHERE id=? AND player_id=?", pet, id)
	check(p != nil, "没有这只灵兽")
	a.run("UPDATE player_pets SET deployed=0 WHERE id=?", pet)
	a.toast(id, "它退到身后休息")
	a.notify(id)
}

// feedPet 用干粮补经验和信赖，或用元息果补元息。
func (a *App) feedPet(id, pet, item string) {
	check(!a.inBout(id), "等这场打完再喂")
	p := a.ensurePet(a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", pet, id))
	check(p != nil, "没有这只灵兽")
	row := a.takeItem(id, item)
	check(str(row["location"]) == "bag", "只能用背包里的东西")
	kind := str(row["template_id"])
	check(kind == "beast_grain" || kind == "vigor_fruit", "这个不能喂")
	a.consume(row, 1)
	loyalty := num(p["loyalty"])
	vigor := num(p["vigor"])
	if kind == "beast_grain" {
		loyalty = clamp(loyalty+6, 0, 100)
		a.run("UPDATE player_pets SET loyalty=? WHERE id=?", loyalty, pet)
		a.petExp(pet, 14)
		a.toast(id, fmt.Sprintf("兽谷干粮：信赖 +6（%g · %s）", loyalty, moodName(loyalty)))
	} else {
		before := vigor
		vigor = math.Min(80, vigor+28)
		loyalty = clamp(loyalty+2, 0, 100)
		a.run("UPDATE player_pets SET loyalty=?,vigor=? WHERE id=?", loyalty, vigor, pet)
		a.toast(id, fmt.Sprintf("元息果：元息 %s（%g），信赖 +2（%g · %s）", signed(vigor-before), vigor, loyalty, moodName(loyalty)))
	}
	a.notify(id)
}

// teachPet 用兽纹印往空栏随机刻一招，或用兽纹帖刻指定的一招。栏满时只有兽纹帖能覆掉选定的旧招。
func (a *App) teachPet(id, pet, item, cover string) {
	check(!a.inBout(id), "等这场打完再刻印")
	p := a.ensurePet(a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", pet, id))
	check(p != nil, "没有这只灵兽")
	def := a.def("pets", str(p["template_id"]))
	check(petRole(def) == "beast", "宠物学不会这种兽技")
	row := a.takeItem(id, item)
	check(str(row["location"]) == "bag", "只能用背包里的东西")
	tpl := a.def("items", str(row["template_id"]))
	taught := str(tpl["teach"])
	skills := arr(parse(str(p["skills"])))
	have := map[string]bool{}
	for _, s := range skills {
		have[str(s)] = true
	}
	var pick string
	replaced := false
	if taught != "" {
		check(a.def("skills", taught) != nil, "没有这道兽纹")
		check(!have[taught], "它已经会这一招")
		if len(skills) < skillSlots("beast") {
			skills = append(skills, taught)
		} else {
			check(cover != "" && have[cover], "技能栏满了。选一招覆掉")
			next := []any{}
			for _, s := range skills {
				if str(s) == cover {
					next = append(next, taught)
					replaced = true
				} else {
					next = append(next, s)
				}
			}
			check(replaced, "它没有这一招可以覆")
			skills = next
		}
		pick = taught
	} else {
		check(str(row["template_id"]) == "beast_seal", "这不是兽纹印")
		check(len(skills) < skillSlots("beast"), "技能栏满了。兽纹帖可以覆掉旧招")
		pool := []string{}
		for _, sid := range beastTeach {
			if !have[sid] {
				pool = append(pool, sid)
			}
		}
		check(len(pool) > 0, "它已经会这些兽技了")
		pick = pool[int(rand.Float64()*float64(len(pool)))%len(pool)]
		skills = append(skills, pick)
	}
	a.consume(row, 1)
	a.run("UPDATE player_pets SET skills=? WHERE id=?", js(skills), pet)
	verb := "学会了"
	if replaced {
		verb = "覆上了"
	}
	a.toast(id, str(def["name"])+" "+verb+fallback(a.def("skills", pick)["name"], pick))
	a.notify(id)
}

// fusePets 把献上的召唤兽合进留下的那只。根骨和长势取较高的一侧，空着的技能栏抄一招，献上的那只离开。
func (a *App) fusePets(id, keepID, offerID string) {
	check(!a.inBout(id), "这场还没打完")
	check(keepID != "" && offerID != "" && keepID != offerID, "要选出留下的和献上的")
	keep := a.ensurePet(a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", keepID, id))
	offer := a.ensurePet(a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", offerID, id))
	check(keep != nil && offer != nil, "没有这两只灵兽")
	kd := a.def("pets", str(keep["template_id"]))
	od := a.def("pets", str(offer["template_id"]))
	check(petRole(kd) == "beast" && petRole(od) == "beast", "只有召唤兽能合灵")
	check(!a.busy(keepID) && !a.busy(offerID), "派遣中的不能合灵")
	_, hi, _, ghi := rootBands(str(kd["rarity"]))
	capRoot := hi + 8
	nextRoots := M{}
	kr, orr := obj(parse(str(keep["roots"]))), obj(parse(str(offer["roots"])))
	for _, k := range rootKeys {
		v := math.Max(num(kr[k]), num(orr[k]))
		if v > capRoot {
			v = capRoot
		}
		nextRoots[k] = v
	}
	growth := math.Max(num(keep["growth"]), num(offer["growth"])) + 0.02
	if growth > ghi+0.06 {
		growth = ghi + 0.06
	}
	growth = math.Round(growth*100) / 100
	skills := arr(parse(str(keep["skills"])))
	have := map[string]bool{}
	for _, s := range skills {
		have[str(s)] = true
	}
	learned := ""
	if len(skills) < skillSlots("beast") {
		for _, s := range arr(parse(str(offer["skills"]))) {
			sid := str(s)
			if sid != "" && !have[sid] {
				skills = append(skills, sid)
				learned = sid
				break
			}
		}
	}
	loyalty := clamp(math.Round((num(keep["loyalty"])+num(offer["loyalty"]))/2), 0, 100)
	a.run("UPDATE player_pets SET growth=?,roots=?,skills=?,loyalty=? WHERE id=?", growth, js(nextRoots), js(skills), loyalty, keepID)
	if truth(offer["active"]) && !truth(keep["active"]) {
		a.run("UPDATE player_pets SET active=0 WHERE player_id=?", id)
		a.run("UPDATE player_pets SET active=1 WHERE id=?", keepID)
	}
	if truth(offer["deployed"]) && !truth(keep["deployed"]) && num(keep["vigor"]) >= 1 && loyalty >= 15 {
		a.run("UPDATE player_pets SET deployed=0 WHERE player_id=?", id)
		a.run("UPDATE player_pets SET deployed=1 WHERE id=?", keepID)
	}
	a.run("DELETE FROM player_pets WHERE id=? AND player_id=?", offerID, id)
	a.refresh(id)
	extra := ""
	if learned != "" {
		extra = "，学会了" + fallback(a.def("skills", learned)["name"], learned)
	}
	a.toast(id, fmt.Sprintf("%s 合入了%s。长势 %g，信赖 %g（%s）%s", str(kd["name"]), str(od["name"]), growth, loyalty, moodName(loyalty), extra))
	a.notify(id)
}

// snareChance 野捕比契约更早能试，上限也更低。血越低越容易套住。
func snareChance(base, hpRatio float64) float64 {
	return clamp(base*0.45+(0.7-hpRatio)*0.2, 0.05, 0.46)
}

// snare 用捕兽索套野外还能契约的目标。七成血以下就能试，成功后的兽更野。
func (a *App) snare(id string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "助手模式下不能野捕")
	i := a.instances[l.InstanceKey]
	check(i != nil, "先进入世界")
	m := i.Monsters[str(l.State["targetId"])]
	check(truth(m["alive"]), "先选中目标")
	d := a.def("monsters", str(m["templateId"]))
	check(str(d["petId"]) != "" && d["captureBase"] != nil, "这只不能野捕")
	hp := num(m["hp"]) / math.Max(1, num(m["maxHp"]))
	check(hp <= 0.7, "先打到七成血以下，再用捕兽索")
	check(d["captureTime"] == nil || contains(d["captureTime"], a.period), "这个时辰它不会停下")
	check(dist(l.State, m) <= 90, "再靠近一点")
	a.consumeTemplate(id, "wild_snare", 1)
	if rand.Float64() > snareChance(num(d["captureBase"]), hp) {
		a.toast(id, "野捕失败，捕兽索断了")
		a.notify(id)
		push(l, M{"kind": "text", "text": "挣脱", "x": m["x"], "y": m["y"], "entityId": m["id"]})
		return
	}
	m["alive"], m["hp"], m["deadAt"], m["respawnAt"] = false, 0, now(), 0
	if num(d["respawnMs"]) > 0 {
		m["respawnAt"] = now() + num(d["respawnMs"])
	}
	a.onMonsterGone(str(m["id"]))
	a.adoptPet(id, str(d["petId"]), "snare")
	push(l, M{"kind": "text", "text": "套住", "x": m["x"], "y": m["y"], "entityId": m["id"]})
}

func (a *App) addBoutBeast(b *Bout, l *Live) {
	if b == nil || l == nil {
		return
	}
	id := str(l.State["id"])
	p := a.one("SELECT * FROM player_pets WHERE player_id=? AND deployed=1", id)
	p = a.ensurePet(p)
	if p == nil {
		return
	}
	def := a.def("pets", str(p["template_id"]))
	if def == nil || petRole(def) != "beast" {
		return
	}
	if a.busy(str(p["id"])) {
		return
	}
	if num(p["vigor"]) < 1 || num(p["loyalty"]) < 15 {
		a.toast(id, str(def["name"])+" 没能上场")
		return
	}
	roots := obj(parse(str(p["roots"])))
	hp, atk, defn, spell, speed := combatStats(num(p["level"]), num(p["growth"]), roots)
	skills := []string{}
	for _, s := range arr(parse(str(p["skills"]))) {
		if str(s) != "" {
			skills = append(skills, str(s))
		}
	}
	if len(skills) == 0 && str(def["skillId"]) != "" {
		skills = []string{str(def["skillId"])}
	}
	if b.Beasts == nil {
		b.Beasts = map[string]*BoutBeast{}
	}
	pid := str(p["id"])
	b.Beasts[pid] = &BoutBeast{
		ID: pid, Owner: id, Name: str(def["name"]), Template: str(def["id"]),
		HP: hp, MaxHP: hp, Atk: atk, Def: defn, Spell: spell, Speed: speed,
		Skills: skills, Alive: true, Loyalty: num(p["loyalty"]), Next: map[string]int{},
	}
	b.Home[pid] = l.InstanceKey
}

func (a *App) beastOf(b *Bout, owner string) *BoutBeast {
	if b == nil {
		return nil
	}
	for _, beast := range b.Beasts {
		if beast != nil && beast.Owner == owner && beast.Alive && beast.HP > 0 {
			return beast
		}
	}
	return nil
}

func (a *App) beastAuto(b *Bout, beast *BoutBeast) M {
	owner := a.live[beast.Owner]
	if owner != nil && num(owner.Stats["maxHp"]) > 0 && num(owner.State["hp"])/num(owner.Stats["maxHp"]) < 0.45 {
		for _, sid := range beast.Skills {
			sk := a.def("skills", sid)
			if sk != nil && num(sk["healPct"]) > 0 && b.Round >= beast.Next[sid] {
				return M{"action": "skill", "skillId": sid}
			}
		}
	}
	best, bestC := "", 0.
	for _, sid := range beast.Skills {
		if beast.Next != nil && b.Round < beast.Next[sid] {
			continue
		}
		sk := a.def("skills", sid)
		if sk == nil {
			continue
		}
		if num(sk["coefficient"]) > bestC {
			bestC = num(sk["coefficient"])
			best = sid
		}
	}
	if best != "" {
		return M{"action": "skill", "skillId": best}
	}
	return M{"action": "attack"}
}

func (a *App) resolveBeast(b *Bout, id string, msg M) {
	beast := b.Beasts[id]
	if beast == nil || !beast.Alive {
		return
	}
	if beast.Loyalty < 40 && rand.Float64() < 0.3 {
		a.note(b, beast.Name+" 扭过头，这一手没动")
		flare(a.live[beast.Owner], "text", "没动", beast.ID)
		return
	}
	switch str(msg["action"]) {
	case "defend":
		beast.Guard = true
		a.addEffect(b, id, RoundEffect{ID: "guard", Name: "守势", Rounds: 1, Taken: 0.55})
		a.note(b, beast.Name+" 守势")
		flare(a.live[beast.Owner], "text", "守势", beast.ID)
	case "skill":
		a.beastSkill(b, beast, str(msg["skillId"]))
	default:
		a.beastAttack(b, beast)
	}
	if b.Phase == "choose" && !a.anyMonsterAlive(b) {
		a.finishBout(b, "victory")
	}
}

func (a *App) beastAttack(b *Bout, beast *BoutBeast) {
	m, _ := a.firstFoe(b)
	if m == nil {
		return
	}
	a.note(b, beast.Name+" 攻击 "+str(m["name"]))
	flare(a.live[beast.Owner], "swing", "爪击", beast.ID)
	a.beastStrike(b, beast, m, 1, "physical", false)
}

func (a *App) beastSkill(b *Bout, beast *BoutBeast, sid string) {
	found := false
	for _, s := range beast.Skills {
		if s == sid {
			found = true
		}
	}
	check(found, "它不会这一招")
	if beast.Next == nil {
		beast.Next = map[string]int{}
	}
	check(b.Round >= beast.Next[sid], "这一招还没准备好")
	skill := a.def("skills", sid)
	check(skill != nil, "没有这个技能")
	beast.Next[sid] = b.Round + 2
	a.note(b, beast.Name+" 施放 "+str(skill["name"]))
	flare(a.live[beast.Owner], "cast", str(skill["name"]), beast.ID)
	owner := a.live[beast.Owner]
	if owner != nil && skill["buff"] != nil {
		a.addBuff(owner, sid, obj(skill["buff"]))
		a.refresh(beast.Owner)
	}
	if owner != nil && num(skill["shieldFlat"]) > 0 {
		owner.State["shield"] = num(owner.State["shield"]) + num(skill["shieldFlat"])
	}
	if owner != nil && num(skill["healPct"]) > 0 {
		amount := math.Max(1, math.Round(num(owner.Stats["maxHp"])*num(skill["healPct"])*0.7))
		owner.State["hp"] = math.Min(num(owner.Stats["maxHp"]), num(owner.State["hp"])+amount)
		push(owner, M{"kind": "heal", "amount": amount, "x": owner.State["x"], "y": owner.State["y"], "entityId": owner.State["id"]})
	}
	if num(skill["tauntMs"]) > 0 {
		beast.Taunt = 2
	}
	if num(skill["coefficient"]) > 0 {
		victims := []M{}
		if truth(skill["aoe"]) {
			for mid := range b.Monsters {
				if m := a.boutMonster(b, mid); m != nil && truth(m["alive"]) {
					victims = append(victims, m)
				}
			}
		} else if m, _ := a.firstFoe(b); m != nil {
			victims = append(victims, m)
		}
		for _, m := range victims {
			t := now()
			for _, k := range []string{"taunt", "slow", "shred", "vulnerable"} {
				if num(skill[k+"Ms"]) > 0 {
					m[k+"Until"] = t + num(skill[k+"Ms"])
				}
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
			a.beastStrike(b, beast, m, num(skill["coefficient"]), str(skill["element"]), truth(skill["forceCrit"]))
			if b.Phase != "choose" {
				break
			}
		}
	}
}

func (a *App) beastStrike(b *Bout, beast *BoutBeast, m M, coeff float64, element string, force bool) {
	if m == nil || !truth(m["alive"]) || coeff <= 0 || b.Phase != "choose" {
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
	attack := beast.Atk
	if element != "" && element != "physical" {
		attack = beast.Spell + beast.Atk*0.2
	}
	amount, crit := damage(attack, coeff, def, 0, num(obj(d["elemRes"])[element]), 0.05, 1.5, taken, force, rand.Float64)
	m["hp"] = num(m["hp"]) - amount
	inst := a.instances[b.Home[str(m["id"])]]
	if inst != nil {
		a.tell(inst, M{"kind": "dmg", "amount": amount, "crit": crit, "x": m["x"], "y": m["y"], "entityId": m["id"]})
	}
	if num(m["hp"]) <= 0 {
		if owner := a.live[beast.Owner]; owner != nil && inst != nil {
			a.killMonster(inst, owner, m)
		} else {
			m["hp"] = 0
			m["alive"] = false
			a.onMonsterGone(str(m["id"]))
		}
	}
}

func (a *App) hurtBeast(b *Bout, m M, beast *BoutBeast, coeff float64) {
	if beast == nil || !beast.Alive {
		return
	}
	d := a.def("monsters", str(m["templateId"]))
	taken := 1.
	if beast.Guard {
		taken = 0.55
	}
	amount, crit := damage(num(d["atk"]), coeff, beast.Def, 0, 0, .05, 1.5, taken, false, rand.Float64)
	beast.HP -= amount
	ev := M{"kind": "dmg", "amount": amount, "crit": crit, "guarded": beast.Guard, "beast": true, "entityId": beast.ID}
	if owner := a.live[beast.Owner]; owner != nil {
		ev["x"], ev["y"] = owner.State["x"], owner.State["y"]
	}
	if inst := a.instances[b.Home[beast.ID]]; inst != nil {
		a.tell(inst, ev)
	} else if owner := a.live[beast.Owner]; owner != nil {
		push(owner, ev)
	}
	if beast.HP <= 0 {
		beast.HP = 0
		beast.Alive = false
		a.note(b, beast.Name+" 倒下了")
	}
}

// settleBeasts 把这场的经验、信赖和元息写回存档。逃跑和战败也记。
func (a *App) settleBeasts(b *Bout, phase string) {
	if b.Settled {
		return
	}
	b.Settled = true
	for _, beast := range b.Beasts {
		if beast == nil {
			continue
		}
		exp, loyalty := 0., 0.
		switch phase {
		case "victory":
			exp = 20 + float64(b.Round)*3
			loyalty = 1
		case "defeat":
			exp = 8
			loyalty = -3
		default:
			loyalty = -1
		}
		if !beast.Alive {
			loyalty -= 2
			exp *= 0.5
		}
		p := a.one("SELECT * FROM player_pets WHERE id=?", beast.ID)
		if p == nil {
			continue
		}
		nextL := clamp(num(p["loyalty"])+loyalty, 0, 100)
		nextV := math.Max(0, num(p["vigor"])-1)
		a.run("UPDATE player_pets SET loyalty=?,vigor=? WHERE id=?", nextL, nextV, beast.ID)
		if exp > 0 {
			a.petExp(beast.ID, exp)
		}
		extra := ""
		if nextV <= 0 {
			extra += "，元息耗尽"
		}
		if nextL < 15 {
			extra += "，不愿出战"
		} else if nextL < 40 {
			extra += "，出手会犹豫"
		}
		a.toast(beast.Owner, fmt.Sprintf("%s 这场：经验 %s，信赖 %s（%g · %s），元息 -1（%g）%s", beast.Name, signed(exp), signed(loyalty), nextL, moodName(nextL), nextV, extra))
		a.notify(beast.Owner)
	}
}
