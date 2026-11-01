// 旅册：旅迹、异兽、灵兽和踏过的地方。只记见闻，不改回合、掉落和经验。

package main

import "sort"

type albumFacts struct {
	kills      map[string]float64
	seen       map[string]bool
	maps       map[string]bool
	pets       map[string]bool
	totalKills float64
	bossKills  float64
	mapCount   float64
	petCount   float64
	level      float64
	craft      float64
	cook       float64
	guild      bool
}

// journalBag 取出旅册里的一格。缺了就补一张空表，调用方改的是同一张。
func journalBag(j M, key string) M {
	bag := obj(j[key])
	j[key] = bag
	return bag
}

func (a *App) loadJournal(id string) M {
	flags := obj(parse(str(a.player(id)["flags"])))
	return obj(flags["journal"])
}

func (a *App) albumFacts(id string, j M) albumFacts {
	f := albumFacts{
		kills: map[string]float64{},
		seen:  map[string]bool{},
		maps:  map[string]bool{},
		pets:  map[string]bool{},
	}
	for key, value := range obj(j["kills"]) {
		n := num(value)
		if n <= 0 {
			continue
		}
		f.kills[key] = n
		f.totalKills += n
		role := str(a.def("monsters", key)["role"])
		if role == "boss" || role == "world" {
			f.bossKills += n
		}
	}
	for key, value := range obj(j["seen"]) {
		if truth(value) {
			f.seen[key] = true
		}
	}
	for key, value := range obj(j["maps"]) {
		if truth(value) {
			f.maps[key] = true
			f.mapCount++
		}
	}
	for key, value := range obj(j["pets"]) {
		if truth(value) {
			f.pets[key] = true
			f.petCount++
		}
	}
	p := a.player(id)
	f.level = num(p["level"])
	f.craft = num(a.track(id, "life:craft")["level"])
	f.cook = num(a.track(id, "life:cook")["level"])
	f.guild = a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id) != nil
	return f
}

func (a *App) deedRows(id string, j M) []M {
	f := a.albumFacts(id, j)
	hand := f.craft
	if f.cook > hand {
		hand = f.cook
	}
	if hand < 1 {
		hand = 1
	}
	type spec struct {
		id, name, blurb string
		current, need   float64
	}
	rows := []spec{
		{"first_town", "落脚星落", "在星落镇落过脚。", boolNum(f.maps["town_starfall"]), 1},
		{"into_woods", "踏进林子", "去过翠叶森林。", boolNum(f.maps["wild_greenleaf"]), 1},
		{"first_blow", "初次猎获", "击败过一只异兽。", f.totalKills, 1},
		{"ten_hunts", "十次猎获", "累计击败十只异兽。", f.totalKills, 10},
		{"boss_mark", "见过首领", "击败过首领，或那只稀有的大东西。", f.bossKills, 1},
		{"first_bond", "第一份契约", "契约过一只灵兽。", f.petCount, 1},
		{"two_bonds", "双兽在册", "契约过两只不同的灵兽。", f.petCount, 2},
		{"rank_five", "五级旅人", "自己的等级达到 5。", f.level, 5},
		{"three_lands", "走过三处", "踏过三张不同的地图。", f.mapCount, 3},
		{"ruins_gate", "遗迹门槛", "进入过古代遗迹。", boolNum(f.maps["dungeon_ruins"]), 1},
		{"hand_craft", "手艺起步", "把制造或料理练到 2 级。", hand - 1, 1},
		{"joined_guild", "入了公会", "创建或加入一个公会。", boolNum(f.guild), 1},
	}
	out := make([]M, 0, len(rows))
	for _, row := range rows {
		current := row.current
		if current < 0 {
			current = 0
		}
		out = append(out, M{
			"id": row.id, "name": row.name, "blurb": row.blurb,
			"current": current, "need": row.need, "done": current >= row.need,
		})
	}
	return out
}

func boolNum(on bool) float64 {
	if on {
		return 1
	}
	return 0
}

// touchAlbum 补上旧档里已经有的见闻，再记下这一次的变化。新达成的旅迹会提示一次。
func (a *App) touchAlbum(id string, mutate func(M)) {
	j := a.loadJournal(id)
	before := js(j)
	historical := j["told"] == nil
	journalBag(j, "kills")
	journalBag(j, "seen")
	journalBag(j, "pets")
	journalBag(j, "maps")
	if historical {
		pets := journalBag(j, "pets")
		for _, row := range a.rows("SELECT template_id FROM player_pets WHERE player_id=?", id) {
			if tid := str(row["template_id"]); tid != "" {
				pets[tid] = true
			}
		}
		maps := journalBag(j, "maps")
		if !journalAny(maps) {
			if mid := str(a.player(id)["map_id"]); a.def("maps", mid) != nil {
				maps[mid] = true
			}
		}
		j["told"] = a.toldOf(id, j)
	}
	told := journalBag(j, "told")
	known := map[string]bool{}
	for key, value := range told {
		if truth(value) {
			known[key] = true
		}
	}
	if mutate != nil {
		mutate(j)
	}
	for _, deed := range a.deedRows(id, j) {
		did := str(deed["id"])
		if !truth(deed["done"]) || known[did] {
			continue
		}
		told[did] = true
		a.toast(id, "旅迹达成："+str(deed["name"]))
	}
	if js(j) == before {
		return
	}
	flags := obj(parse(str(a.player(id)["flags"])))
	flags["journal"] = j
	a.run("UPDATE players SET flags=?,updated_at=? WHERE id=?", js(flags), now(), id)
	if a.live[id] != nil {
		a.notify(id)
	}
}

func journalAny(m M) bool {
	for _, value := range m {
		if truth(value) {
			return true
		}
	}
	return false
}

func (a *App) toldOf(id string, j M) M {
	told := M{}
	for _, deed := range a.deedRows(id, j) {
		if truth(deed["done"]) {
			told[str(deed["id"])] = true
		}
	}
	return told
}

func (a *App) markMap(id, mapID string) {
	if a.def("maps", mapID) == nil {
		return
	}
	a.touchAlbum(id, func(j M) { journalBag(j, "maps")[mapID] = true })
}

func (a *App) markPet(id, template string) {
	if a.def("pets", template) == nil {
		return
	}
	a.touchAlbum(id, func(j M) { journalBag(j, "pets")[template] = true })
}

func (a *App) markKill(id, template string) {
	if a.def("monsters", template) == nil {
		return
	}
	a.touchAlbum(id, func(j M) {
		journalBag(j, "seen")[template] = true
		kills := journalBag(j, "kills")
		kills[template] = num(kills[template]) + 1
	})
}

func (a *App) markSeen(id string, templates []string) {
	if len(templates) == 0 {
		return
	}
	a.touchAlbum(id, func(j M) {
		seen := journalBag(j, "seen")
		for _, template := range templates {
			if a.def("monsters", template) != nil {
				seen[template] = true
			}
		}
	})
}

// markBoutSeen 这场里的人遇见了拉进来的异兽。逃走也算见过，击败另计。
func (a *App) markBoutSeen(b *Bout) {
	templates := []string{}
	have := map[string]bool{}
	for mid := range b.Monsters {
		m := a.boutMonster(b, mid)
		if m == nil {
			continue
		}
		tid := str(m["templateId"])
		if tid == "" || have[tid] {
			continue
		}
		have[tid] = true
		templates = append(templates, tid)
	}
	if len(templates) == 0 {
		return
	}
	for pid := range b.Players {
		a.markSeen(pid, templates)
	}
}

func (a *App) album(id string) M {
	a.touchAlbum(id, nil)
	return a.albumView(id, a.loadJournal(id))
}

func (a *App) albumView(id string, j M) M {
	f := a.albumFacts(id, j)
	deeds := a.deedRows(id, j)
	doneN := 0.
	for _, deed := range deeds {
		if truth(deed["done"]) {
			doneN++
		}
	}
	habitats := a.habitats()
	beasts := []M{}
	seenN, felledN := 0., 0.
	for _, def := range objects(a.data["monsters"]) {
		mid := str(def["id"])
		kills := f.kills[mid]
		state := "unknown"
		if kills > 0 {
			state = "felled"
			felledN++
			seenN++
		} else if f.seen[mid] {
			state = "met"
			seenN++
		}
		places := []string{}
		if state != "unknown" {
			places = append(places, habitats[mid]...)
		}
		beasts = append(beasts, M{
			"id": mid, "name": nameIf(state != "unknown", str(def["name"])),
			"level": def["level"], "role": str(def["role"]),
			"roleName": nameIf(state != "unknown", beastRankName(str(def["role"]))),
			"element":  nameIf(state != "unknown", elementName(str(def["element"]))),
			"tint":     def["tint"], "state": state, "stateName": beastStateName(state),
			"kills": kills, "places": places,
		})
	}
	sort.SliceStable(beasts, func(i, k int) bool {
		if beastStateRank(str(beasts[i]["state"])) != beastStateRank(str(beasts[k]["state"])) {
			return beastStateRank(str(beasts[i]["state"])) < beastStateRank(str(beasts[k]["state"]))
		}
		if num(beasts[i]["level"]) != num(beasts[k]["level"]) {
			return num(beasts[i]["level"]) < num(beasts[k]["level"])
		}
		return str(beasts[i]["id"]) < str(beasts[k]["id"])
	})
	companions := []M{}
	for _, def := range objects(a.data["pets"]) {
		pid := str(def["id"])
		known := f.pets[pid]
		role := petRole(def)
		companions = append(companions, M{
			"id": pid, "known": known,
			"name": nameIf(known, str(def["name"])),
			"role": role, "roleName": nameIf(known, roleLabel(role)),
			"rarityName": nameIf(known, rarityName(str(def["rarity"]))),
			"battle":     nameIf(known, str(def["battle"])),
			"work":       nameIf(known, str(def["work"])),
			"desc":       nameIf(known, str(def["desc"])),
		})
	}
	sort.SliceStable(companions, func(i, k int) bool {
		if truth(companions[i]["known"]) != truth(companions[k]["known"]) {
			return truth(companions[i]["known"])
		}
		return str(companions[i]["id"]) < str(companions[k]["id"])
	})
	lands := []M{}
	for _, def := range objects(a.data["maps"]) {
		mid := str(def["id"])
		lands = append(lands, M{
			"id": mid, "name": def["name"], "kind": def["kind"], "kindName": landKindName(str(def["kind"])),
			"unlockLevel": def["unlockLevel"], "visited": f.maps[mid], "blurb": def["desc"],
		})
	}
	return M{
		"summary": M{
			"deedsDone": doneN, "deeds": len(deeds),
			"seen": seenN, "felled": felledN, "beasts": len(beasts),
			"pets": f.petCount, "petTotal": len(companions),
			"maps": f.mapCount, "mapTotal": len(lands),
		},
		"deeds": deeds, "beasts": beasts, "companions": companions, "lands": lands,
	}
}

func (a *App) habitats() map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, m := range objects(a.data["maps"]) {
		name := str(m["name"])
		for _, spawn := range objects(m["spawns"]) {
			id := str(spawn["monsterId"])
			if id == "" || name == "" {
				continue
			}
			if seen[id] == nil {
				seen[id] = map[string]bool{}
			}
			if seen[id][name] {
				continue
			}
			seen[id][name] = true
			out[id] = append(out[id], name)
		}
	}
	return out
}

func nameIf(show bool, name string) string {
	if show {
		return name
	}
	return ""
}

func beastStateRank(state string) int {
	switch state {
	case "felled":
		return 0
	case "met":
		return 1
	default:
		return 2
	}
}

func beastStateName(state string) string {
	switch state {
	case "felled":
		return "击败过"
	case "met":
		return "遇见过"
	default:
		return "未曾遇见"
	}
}

func beastRankName(role string) string {
	switch role {
	case "elite":
		return "精英"
	case "boss":
		return "首领"
	case "world":
		return "稀有"
	default:
		return "寻常"
	}
}

func elementName(id string) string {
	switch id {
	case "fire":
		return "火"
	case "water":
		return "水"
	case "thunder":
		return "雷"
	case "wind":
		return "风"
	case "earth":
		return "土"
	default:
		return "物理"
	}
}

func rarityName(id string) string {
	switch id {
	case "uncommon":
		return "少见"
	case "rare":
		return "珍稀"
	case "epic":
		return "史诗"
	default:
		return "常见"
	}
}

func landKindName(kind string) string {
	switch kind {
	case "town":
		return "镇子"
	case "dungeon":
		return "副本"
	default:
		return "野外"
	}
}
