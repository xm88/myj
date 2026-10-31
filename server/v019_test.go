// V0.1.9：召唤兽出战进回合，宠物只携带；养成和战斗结算写回存档。

package main

import (
	"strings"
	"testing"
)

func TestBeastRollDeployAndAura(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.givePet(id, "forest_cat", false)
	cat := a.one("SELECT * FROM player_pets WHERE player_id=? AND template_id='forest_cat'", id)
	if cat == nil || num(cat["deployed"]) != 1 || num(cat["active"]) != 1 || num(cat["vigor"]) != 80 || num(cat["loyalty"]) != 82 {
		t.Fatalf("fresh beast %+v", cat)
	}
	roots := obj(parse(str(cat["roots"])))
	if num(roots["claw"]) < 30 || num(roots["step"]) < 30 || num(cat["growth"]) < 0.9 {
		t.Fatalf("roots %+v growth %v", roots, cat["growth"])
	}
	if len(arr(parse(str(cat["skills"])))) != 1 {
		t.Fatalf("innate skills %s", cat["skills"])
	}
	if a.countItem(id, "beast_grain") != 2 || a.countItem(id, "vigor_fruit") != 1 {
		t.Fatal("first beast rations missing")
	}
	bare := a.stats(id, 1, "sword_shield", []M{})
	if num(bare["critRate"]) < 0.12 {
		t.Fatalf("aura missing %v", bare["critRate"])
	}
	a.givePet(id, "mine_rat", false)
	rat := a.one("SELECT * FROM player_pets WHERE player_id=? AND template_id='mine_rat'", id)
	if num(rat["deployed"]) != 0 || num(rat["active"]) != 0 {
		t.Fatalf("pet should not take the battle slot %+v", rat)
	}
	view := a.profile(id)
	var beast, pet M
	for _, row := range objects(view["pets"]) {
		if str(row["templateId"]) == "forest_cat" {
			beast = row
		}
		if str(row["templateId"]) == "mine_rat" {
			pet = row
		}
	}
	if str(beast["roleName"]) != "召唤兽" || str(pet["roleName"]) != "宠物" || num(beast["panel"].(M)["hp"]) <= 0 {
		t.Fatalf("profile beast %+v pet %+v", beast, pet)
	}
}

func TestOldPetRowBackfill(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.run("INSERT INTO player_pets(id,player_id,template_id,level,exp,active,created_at) VALUES(?,?,?,3,4,1,?)", "oldpet", id, "forest_cat", now())
	a.backfillPets()
	row := a.one("SELECT * FROM player_pets WHERE id='oldpet'")
	if len(obj(parse(str(row["roots"])))) != 5 || num(row["loyalty"]) < 60 || num(row["deployed"]) != 1 || num(row["level"]) != 3 {
		t.Fatalf("backfill %+v", row)
	}
	roots := str(row["roots"])
	a.backfillPets()
	again := a.one("SELECT roots, loyalty FROM player_pets WHERE id='oldpet'")
	if str(again["roots"]) != roots || num(again["loyalty"]) != num(row["loyalty"]) {
		t.Fatal("backfill rerolled")
	}
}

func TestPetFeedTeachAndRefuse(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.givePet(id, "mine_rat", false)
	rat := str(a.one("SELECT id FROM player_pets WHERE template_id='mine_rat' AND player_id=?", id)["id"])
	if _, code := request(t, a, "/api/pets/deploy", token, M{"petId": rat}); code != 400 {
		t.Fatal("pet deployed into the battle slot")
	}
	a.givePet(id, "forest_cat", false)
	cat := a.one("SELECT * FROM player_pets WHERE template_id='forest_cat' AND player_id=?", id)
	cid := str(cat["id"])
	a.run("UPDATE player_pets SET vigor=0 WHERE id=?", cid)
	if _, code := request(t, a, "/api/pets/deploy", token, M{"petId": cid}); code != 400 {
		t.Fatal("spent beast deployed")
	}
	a.run("UPDATE player_pets SET vigor=80, loyalty=10 WHERE id=?", cid)
	if _, code := request(t, a, "/api/pets/deploy", token, M{"petId": cid}); code != 400 {
		t.Fatal("disloyal beast deployed")
	}
	a.run("UPDATE player_pets SET loyalty=82, deployed=0 WHERE id=?", cid)
	okRequest(t, a, "/api/pets/deploy", token, M{"petId": cid})
	if num(a.one("SELECT deployed FROM player_pets WHERE id=?", cid)["deployed"]) != 1 {
		t.Fatal("deploy did not stick")
	}
	okRequest(t, a, "/api/pets/rest", token, M{"petId": cid})
	if num(a.one("SELECT deployed FROM player_pets WHERE id=?", cid)["deployed"]) != 0 {
		t.Fatal("rest did not clear")
	}
	grain := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='beast_grain'", id)
	before := num(a.one("SELECT loyalty, exp FROM player_pets WHERE id=?", cid)["loyalty"])
	okRequest(t, a, "/api/pets/feed", token, M{"petId": cid, "itemId": grain["id"]})
	fed := a.one("SELECT loyalty, exp FROM player_pets WHERE id=?", cid)
	if num(fed["loyalty"]) != before+6 || num(fed["exp"]) < 14 {
		t.Fatalf("feed %+v", fed)
	}
	a.addItem(id, "beast_seal", 1, nil, "bag")
	seal := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='beast_seal'", id)
	okRequest(t, a, "/api/pets/teach", token, M{"petId": cid, "itemId": seal["id"]})
	if len(arr(parse(str(a.one("SELECT skills FROM player_pets WHERE id=?", cid)["skills"])))) != 2 {
		t.Fatal("teach did not add a skill")
	}
	if _, code := request(t, a, "/api/pets/teach", token, M{"petId": rat, "itemId": "missing"}); code != 400 {
		t.Fatal("pet accepted a seal")
	}
}

func TestBeastFightsAndWritesBack(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	okRequest(t, a, "/api/travel", token, M{"mapId": "wild_greenleaf"})
	a.givePet(id, "forest_cat", false)
	pet := a.one("SELECT * FROM player_pets WHERE player_id=?", id)
	placeOnSlime(t, a, id)
	a.engage(id)
	l := a.live[id]
	b := a.bouts[str(l.State["boutId"])]
	found := false
	for _, actor := range b.Order {
		if actor.Kind == "beast" && actor.ID == str(pet["id"]) {
			found = true
		}
	}
	if !found || b.Await != id {
		t.Fatalf("order %+v await %s", b.Order, b.Await)
	}
	before := 0.
	for mid := range b.Monsters {
		if foe := a.boutMonster(b, mid); foe != nil {
			before += num(foe["hp"])
		}
	}
	hit := false
	for i := 0; i < 6 && b.Phase == "choose" && !hit; i++ {
		if b.AwaitKind == "beast" {
			a.actBout(id, M{"action": "attack"})
			hit = true
			break
		}
		if b.Await == id {
			a.actBout(id, M{"action": "defend"})
			continue
		}
		t.Fatalf("stuck kind=%s await=%s log=%v", b.AwaitKind, b.Await, b.Log)
	}
	afterHP := 0.
	for mid := range b.Monsters {
		if foe := a.boutMonster(b, mid); foe != nil {
			afterHP += num(foe["hp"])
		}
	}
	if !hit || afterHP >= before {
		t.Fatalf("beast hit=%v hp %v -> %v log %v", hit, before, afterHP, b.Log)
	}
	if !strings.Contains(strings.Join(b.Log, " "), "森林猫") {
		t.Fatalf("log %v", b.Log)
	}
	for i := 0; i < 12 && b.Phase == "choose"; i++ {
		for mid := range b.Monsters {
			if foe := a.boutMonster(b, mid); foe != nil && truth(foe["alive"]) {
				foe["hp"] = 1
			}
		}
		if b.AwaitKind == "beast" || b.Await == id {
			a.actBout(id, M{"action": "attack"})
			continue
		}
		t.Fatalf("no actor kind=%s await=%s log=%v", b.AwaitKind, b.Await, b.Log)
	}
	if b.Phase != "victory" {
		t.Fatalf("phase %s log %v", b.Phase, b.Log)
	}
	after := a.one("SELECT * FROM player_pets WHERE id=?", pet["id"])
	grew := num(after["level"]) > num(pet["level"]) || num(after["exp"]) > num(pet["exp"])
	if !grew || num(after["vigor"]) != num(pet["vigor"])-1 || num(after["loyalty"]) < num(pet["loyalty"]) {
		t.Fatalf("writeback before %+v after %+v", pet, after)
	}
}

func TestCarriedPetStaysOffTheTurnBar(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	okRequest(t, a, "/api/travel", token, M{"mapId": "wild_greenleaf"})
	a.givePet(id, "mine_rat", false)
	placeOnSlime(t, a, id)
	a.engage(id)
	b := a.bouts[str(a.live[id].State["boutId"])]
	for _, actor := range b.Order {
		if actor.Kind == "beast" {
			t.Fatalf("pet entered combat %+v", b.Order)
		}
	}
	if len(b.Beasts) != 0 {
		t.Fatalf("beasts %+v", b.Beasts)
	}
}
