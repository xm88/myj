// V0.2.4：分仓、染色、航标旗和心法册。格子、金币、步速、伤害和航标传送没有改。

package main

import "testing"

func TestEconomyConstantsStay(t *testing.T) {
	a := testApp(t)
	if a.constant("bagCap") != 60 || a.constant("bankCap") != 80 || a.constant("starterGold") != 80 || a.constant("marketTax") != 0.05 || a.constant("guildCost") != 200 || a.constant("starterOrbs") != 3 {
		t.Fatalf("caps bag=%v bank=%v gold=%v tax=%v guild=%v orbs=%v", a.constant("bagCap"), a.constant("bankCap"), a.constant("starterGold"), a.constant("marketTax"), a.constant("guildCost"), a.constant("starterOrbs"))
	}
}

func TestShelvesShareTheOldCap(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	gold := num(a.player(id)["gold"])
	if gold != a.constant("starterGold") {
		t.Fatal("starter gold")
	}
	a.addItem(id, "slime_gel", 5, nil, "bag")
	a.addItem(id, "map_fragment", 1, nil, "bag")
	gel := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)
	bound := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='map_fragment' AND location='bag'", id)
	expectFailure(t, func() {
		a.storage(id, str(bound["id"]), "deposit", 1, "stock")
	})
	if a.slots(id, "bag") < 2 {
		t.Fatal("bound item left the bag")
	}
	out := okRequest(t, a, "/api/storage", token, M{"itemId": gel["id"], "direction": "deposit", "count": 2, "bin": "stock"})
	stock, sundry := 0.0, 0.0
	for _, item := range objects(out["bank"]) {
		if str(item["templateId"]) != "slime_gel" {
			continue
		}
		if str(item["bin"]) == "stock" {
			stock += num(item["count"])
		}
		if str(item["bin"]) == "sundry" {
			sundry += num(item["count"])
		}
	}
	if stock != 2 || sundry != 0 || num(a.one("SELECT count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)["count"]) != 3 {
		t.Fatalf("split stock=%v sundry=%v", stock, sundry)
	}
	rest := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)
	okRequest(t, a, "/api/storage", token, M{"itemId": rest["id"], "direction": "deposit", "count": 3, "bin": "sundry"})
	rows := a.rows("SELECT bin,count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank' ORDER BY bin", id)
	if len(rows) != 2 {
		t.Fatalf("shelves merged: %v", rows)
	}
	// 旧档 bin 是空的，再存进杂物仓要并进同一摞。
	a.run("UPDATE player_items SET bin='' WHERE player_id=? AND template_id='slime_gel' AND location='bank' AND bin='sundry'", id)
	a.addItem(id, "slime_gel", 4, nil, "bag")
	again := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)
	a.storage(id, str(again["id"]), "deposit", 4, "")
	merged := a.rows("SELECT bin,count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank'", id)
	sundryN := 0.0
	for _, row := range merged {
		if shownShelf(str(row["bin"])) == "sundry" {
			sundryN += num(row["count"])
		}
	}
	if sundryN != 7 || len(merged) != 2 {
		t.Fatalf("old shelf %v", merged)
	}
	kept := a.one("SELECT id,count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank' AND bin='stock'", id)
	a.storage(id, str(kept["id"]), "shift", num(kept["count"]), "keeps")
	if a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank' AND bin='keeps'", id) == nil {
		t.Fatal("shift missed")
	}
	if num(a.player(id)["gold"]) != gold {
		t.Fatal("shelves charged gold")
	}
	expectFailure(t, func() { a.storage(id, str(kept["id"]), "shift", 1, "nope") })
	expectFailure(t, func() { a.storage(id, str(kept["id"]), "shift", 1, "") })

	// 凝胶两摞还在，再塞满到 80 格后，拆出新的一摞要被拒绝；整摞换仓不占新格。
	for guard := 0; a.slots(id, "bank") < a.constant("bankCap") && guard < 100; guard++ {
		before := a.slots(id, "bank")
		a.addItem(id, "wpn_sword_1", 1, nil, "bank")
		if a.slots(id, "bank") == before {
			t.Fatal("could not fill the bank")
		}
	}
	if a.slots(id, "bank") != 80 {
		t.Fatalf("bank slots %v", a.slots(id, "bank"))
	}
	piece := a.one("SELECT id,count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank' AND (bin='' OR bin='sundry')", id)
	expectFailure(t, func() { a.storage(id, str(piece["id"]), "shift", 1, "stock") })
	a.storage(id, str(piece["id"]), "shift", num(piece["count"]), "stock")
	if a.slots(id, "bank") != 80 || num(a.player(id)["gold"]) != gold {
		t.Fatal("full-shelf move changed capacity or gold")
	}
	sword := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='wpn_sword_1' AND location='bank'", id)
	a.storage(id, str(sword["id"]), "withdraw", 1, "")
	if str(a.one("SELECT location,bin FROM player_items WHERE id=?", sword["id"])["location"]) != "bag" {
		t.Fatal("withdraw")
	}
}

func TestDyeAndMarksLeavePaceAlone(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	gold := num(a.player(id)["gold"])
	expectFailure(t, func() { a.dropMark(id, "镇口") })
	a.attach(id, nil)
	a.attach(id2, nil)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	interval := num(a.live[id].Stats["attackInterval"])
	expectFailure(t, func() { a.setDye(id, "no_such") })
	okRequest(t, a, "/api/dye", token, M{"id": "lake"})
	if str(a.player(id)["dye_id"]) != "lake" || str(a.live[id].State["dye"]) != "lake" {
		t.Fatal("dye not stored")
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || num(a.live[id].Stats["attackInterval"]) != interval {
		t.Fatal("dye changed the pace")
	}
	self := obj(a.snapshot(a.live[id], now())["self"])
	if str(self["dye"]) != "lake" || num(self["speed"]) != speed {
		t.Fatal("snapshot dye")
	}
	seen := false
	for _, entity := range objects(a.snapshot(a.live[id2], now())["entities"]) {
		if str(entity["id"]) == id && str(entity["dye"]) == "lake" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("other traveler missed the dye")
	}
	okRequest(t, a, "/api/dye", token, M{"id": "tea"})
	if str(a.player(id)["dye_id"]) != "" || str(a.live[id].State["dye"]) != "" {
		t.Fatal("tea should stay the default")
	}
	expectFailure(t, func() { a.dropMark(id, "") })
	expectFailure(t, func() { a.dropMark(id, "一二三四五六七") })
	okRequest(t, a, "/api/mark", token, M{"name": "镇 口"})
	a.dropMark(id, "井边")
	a.dropMark(id, "门口")
	expectFailure(t, func() { a.dropMark(id, "第四") })
	marks := a.marksOf(id)
	if len(marks) != 3 || str(marks[0]["name"]) != "镇口" || str(marks[0]["mapId"]) != "town_starfall" {
		t.Fatalf("marks %v", marks)
	}
	hereX := num(a.live[id].State["x"])
	okRequest(t, a, "/api/mark/forget", token, M{"id": marks[2]["id"]})
	if len(a.marksOf(id)) != 2 {
		t.Fatal("forget")
	}
	expectFailure(t, func() { a.forgetMark(id, "missing") })
	a.travel(id, "wild_greenleaf")
	if str(a.live[id].State["mapId"]) != "wild_greenleaf" || str(a.marksOf(id)[0]["mapId"]) != "town_starfall" {
		t.Fatal("mark hijacked travel")
	}
	if num(a.live[id].State["x"]) == hereX && str(a.marksOf(id)[0]["mapId"]) == str(a.live[id].State["mapId"]) {
		t.Fatal("travel did not leave the flag")
	}
	if num(a.player(id)["gold"]) != gold || num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("marks or dye changed the player")
	}
}

func TestVerseStudyLeavesFormulas(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	if verseLane(M{"tauntMs": 4000}) != "guard" || verseLane(M{"buff": M{"stat": "defPct"}}) != "guard" || verseLane(M{"healPct": 0.1}) != "guard" {
		t.Fatal("guard lane")
	}
	if verseLane(M{"aoe": true}) != "edge" || verseLane(M{"forceCrit": true}) != "edge" {
		t.Fatal("edge lane")
	}
	if verseLane(M{"forceCrit": false, "coefficient": 1.2}) != "root" || verseLane(M{"coefficient": 1.65}) != "root" {
		t.Fatal("root lane")
	}
	if verseLane(a.def("skills", "ss_bash")) != "guard" || verseLane(a.def("skills", "gs_spin")) != "edge" || verseLane(a.def("skills", "gs_slash")) != "root" {
		t.Fatal("catalog lanes")
	}
	coef := num(a.def("skills", "ss_bash")["coefficient"])
	load := str(a.player(id)["skill_loadout"])
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	gold := num(a.player(id)["gold"])
	expectFailure(t, func() { a.studyVerse(id, "gs_spin") })
	expectFailure(t, func() { a.studyVerse(id, "no_such") })
	out := okRequest(t, a, "/api/manual", token, M{"id": "ss_bash"})
	manual := obj(out["manual"])
	if str(manual["focus"]) != "ss_bash" || len(arr(manual["studied"])) != 1 {
		t.Fatalf("manual %v", manual)
	}
	a.studyVerse(id, "ss_bash")
	if len(arr(a.manualOf(id)["studied"])) != 1 {
		t.Fatal("study duplicated")
	}
	if num(a.def("skills", "ss_bash")["coefficient"]) != coef || str(a.player(id)["skill_loadout"]) != load {
		t.Fatal("study changed the bar or the coefficient")
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || num(a.player(id)["gold"]) != gold {
		t.Fatal("study changed the player")
	}
	cfg := okRequest(t, a, "/api/config", token, nil)
	if len(objects(cfg["dyes"])) != 6 || len(objects(cfg["shelves"])) != 3 || str(cfg["version"]) != "0.2.5" {
		t.Fatalf("config %v %v %v", len(objects(cfg["dyes"])), len(objects(cfg["shelves"])), cfg["version"])
	}
}
