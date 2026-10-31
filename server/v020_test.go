// V0.2.0：兽纹帖与覆纹、合灵、野捕，以及出手飘字。回合结算公式没有改。

package main

import (
	"math"
	"strings"
	"testing"
)

func TestSnareChanceBands(t *testing.T) {
	low := snareChance(0.1, 0.7)
	mid := snareChance(0.72, 0.7)
	high := snareChance(0.72, 0)
	if low != 0.05 || mid < 0.32 || mid > 0.33 || high <= mid || high > 0.46 {
		t.Fatalf("chance low=%v mid=%v high=%v", low, mid, high)
	}
}

func TestSlipCoverAndRefuse(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.givePet(id, "forest_cat", false)
	cid := str(a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id)["id"])
	a.addItem(id, "slip_shell", 1, nil, "bag")
	slip := str(a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slip_shell'", id)["id"])
	okRequest(t, a, "/api/pets/teach", token, M{"petId": cid, "itemId": slip})
	skills := arr(parse(str(a.one("SELECT skills FROM player_pets WHERE id=?", cid)["skills"])))
	if len(skills) != 2 || !skillListHas(skills, "beast_shell") {
		t.Fatalf("slip skills %v", skills)
	}
	a.addItem(id, "slip_shell", 1, nil, "bag")
	again := str(a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slip_shell'", id)["id"])
	if _, code := request(t, a, "/api/pets/teach", token, M{"petId": cid, "itemId": again}); code != 400 {
		t.Fatal("duplicate slip accepted")
	}
	if a.countItem(id, "slip_shell") != 1 {
		t.Fatal("duplicate slip was consumed")
	}
	a.run("UPDATE player_pets SET skills=? WHERE id=?", js([]any{"pet_cat", "beast_pounce", "beast_shell", "beast_lick"}), cid)
	a.addItem(id, "slip_ember", 1, nil, "bag")
	ember := str(a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slip_ember'", id)["id"])
	if _, code := request(t, a, "/api/pets/teach", token, M{"petId": cid, "itemId": ember}); code != 400 {
		t.Fatal("full bar accepted a slip without a cover")
	}
	okRequest(t, a, "/api/pets/teach", token, M{"petId": cid, "itemId": ember, "cover": "beast_lick"})
	covered := arr(parse(str(a.one("SELECT skills FROM player_pets WHERE id=?", cid)["skills"])))
	if len(covered) != 4 || skillListHas(covered, "beast_lick") || !skillListHas(covered, "beast_ember") {
		t.Fatalf("cover %v", covered)
	}
	view := a.profile(id)
	var mood string
	for _, row := range objects(view["pets"]) {
		if str(row["templateId"]) == "forest_cat" {
			mood = str(row["moodName"])
		}
	}
	if mood != "亲近" {
		t.Fatalf("fresh mood %s", mood)
	}
}

func TestFuseKeepsTheStrongerBones(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.givePet(id, "forest_cat", false)
	a.givePet(id, "fire_fox", false)
	a.givePet(id, "mine_rat", false)
	cat := str(a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id)["id"])
	fox := str(a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id='fire_fox'", id)["id"])
	rat := str(a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id='mine_rat'", id)["id"])
	bones := func(n float64) string {
		return js(M{"claw": n, "plate": n, "blood": n, "mind": n, "step": n})
	}
	a.run("UPDATE player_pets SET growth=?,roots=?,skills=?,loyalty=? WHERE id=?", 0.99, bones(31), js([]any{"pet_cat"}), 80, cat)
	a.run("UPDATE player_pets SET growth=?,roots=?,skills=?,loyalty=?,active=0,deployed=0 WHERE id=?", 1.05, bones(55), js([]any{"pet_fox"}), 40, fox)
	if _, code := request(t, a, "/api/pets/fuse", token, M{"keepId": cat, "offerId": rat}); code != 400 {
		t.Fatal("pet fused")
	}
	okRequest(t, a, "/api/pets/fuse", token, M{"keepId": cat, "offerId": fox})
	if a.one("SELECT id FROM player_pets WHERE id=?", fox) != nil {
		t.Fatal("offer still exists")
	}
	row := a.one("SELECT * FROM player_pets WHERE id=?", cat)
	roots := obj(parse(str(row["roots"])))
	skills := arr(parse(str(row["skills"])))
	if num(roots["claw"]) != 55 || math.Abs(num(row["growth"])-1.07) > 0.001 || num(row["loyalty"]) != 60 || !skillListHas(skills, "pet_fox") {
		t.Fatalf("fused %+v roots %+v skills %v", row, roots, skills)
	}
	view := a.profile(id)
	for _, pet := range objects(view["pets"]) {
		if str(pet["id"]) == cat && str(pet["moodName"]) != "寻常" {
			t.Fatalf("fused mood %+v", pet["moodName"])
		}
	}
}

func TestSnareRejectsAndWildTemper(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	a.travel(id, "wild_greenleaf")
	mob := placeOnTemplate(t, a, id, "forest_cat")
	mob["hp"] = num(mob["maxHp"]) * 0.4
	if !panicked(func() { a.snare(id) }) {
		t.Fatal("snare without a rope")
	}
	a.addItem(id, "wild_snare", 1, nil, "bag")
	mob["hp"] = num(mob["maxHp"])
	if !panicked(func() { a.snare(id) }) || a.countItem(id, "wild_snare") != 1 {
		t.Fatal("healthy snare should fail before consuming the rope")
	}
	rat := placeOnTemplate(t, a, id, "forest_rat")
	rat["hp"] = num(rat["maxHp"]) * 0.2
	if !panicked(func() { a.snare(id) }) || a.countItem(id, "wild_snare") != 1 {
		t.Fatal("uncapturable target accepted a snare")
	}
	got := false
	for i := 0; i < 24 && !got; i++ {
		mob["alive"], mob["hp"] = true, num(mob["maxHp"])*0.15
		l := a.live[id]
		l.State["x"], l.State["y"], l.State["targetId"] = mob["x"], mob["y"], mob["id"]
		a.addItem(id, "wild_snare", 1, nil, "bag")
		_ = panicked(func() { a.snare(id) })
		if a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id) != nil {
			got = true
		}
	}
	if !got {
		t.Fatal("snare never landed")
	}
	row := a.one("SELECT loyalty, vigor, deployed FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id)
	if num(row["loyalty"]) != 52 || num(row["vigor"]) != 60 || num(row["deployed"]) != 1 {
		t.Fatalf("wild temper %+v", row)
	}
	if str(a.viewPet(a.one("SELECT * FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id))["moodName"]) != "寻常" {
		t.Fatal("wild mood")
	}
}

func TestCastFlareDoesNotChangeTheHit(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	a.travel(id, "wild_greenleaf")
	placeOnSlime(t, a, id)
	a.engage(id)
	l := a.live[id]
	l.Events = []M{}
	b := a.bouts[str(l.State["boutId"])]
	before := 0.
	for mid := range b.Monsters {
		if foe := a.boutMonster(b, mid); foe != nil {
			before += num(foe["hp"])
		}
	}
	a.actBout(id, M{"action": "skill", "slot": 1})
	after := 0.
	for mid := range b.Monsters {
		if foe := a.boutMonster(b, mid); foe != nil {
			after += num(foe["hp"])
		}
	}
	if after >= before {
		t.Fatalf("skill did not land %v -> %v", before, after)
	}
	found := false
	for _, ev := range l.Events {
		if str(ev["kind"]) == "cast" && str(ev["text"]) != "" && str(ev["entityId"]) == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("no cast flare %v", l.Events)
	}
	log := strings.Join(b.Log, " ")
	if !strings.Contains(log, "施放") {
		t.Fatalf("log %s", log)
	}
}

func placeOnTemplate(t *testing.T, a *App, id, template string) M {
	t.Helper()
	l := a.live[id]
	i := a.instances[l.InstanceKey]
	var mob M
	for _, m := range i.Monsters {
		if str(m["templateId"]) == template && truth(m["alive"]) {
			mob = m
			break
		}
	}
	if mob == nil {
		t.Fatal("no " + template)
	}
	l.State["x"], l.State["y"], l.State["targetId"] = mob["x"], mob["y"], mob["id"]
	return mob
}

func skillListHas(skills []any, id string) bool {
	for _, s := range skills {
		if str(s) == id {
			return true
		}
	}
	return false
}

func panicked(fn func()) (hit bool) {
	defer func() {
		hit = recover() != nil
	}()
	fn()
	return false
}
