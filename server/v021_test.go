// V0.2.1：旅册只记见闻。击败仍按原来的经验结算。

package main

import "testing"

func TestAlbumOpensOnTheTown(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	health, _ := request(t, a, "/api/health", "", nil)
	if str(health["version"]) != "0.2.1" {
		t.Fatalf("version %v", health["version"])
	}
	book := okRequest(t, a, "/api/album", token, nil)
	if num(obj(book["summary"])["deedsDone"]) != 1 || num(obj(book["summary"])["maps"]) != 1 {
		t.Fatalf("summary %+v", book["summary"])
	}
	if !truth(albumDeed(t, book, "first_town")["done"]) || truth(albumDeed(t, book, "into_woods")["done"]) {
		t.Fatal("town deed")
	}
	if num(albumDeed(t, book, "rank_five")["current"]) != 1 || num(albumDeed(t, book, "hand_craft")["current"]) != 0 {
		t.Fatal("starting progress")
	}
	slime := albumBeast(t, book, "slime")
	if str(slime["state"]) != "unknown" || str(slime["name"]) != "" || num(slime["kills"]) != 0 {
		t.Fatalf("fresh slime %+v", slime)
	}
	a.setFlag(id, "hidden_mine")
	flags := obj(parse(str(a.player(id)["flags"])))
	if !truth(flags["hidden_mine"]) || !journalAny(journalBag(obj(flags["journal"]), "maps")) {
		t.Fatalf("flags %+v", flags)
	}
}

func TestAlbumFollowsTheTrail(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	a.travel(id, "wild_greenleaf")
	book := a.album(id)
	if !truth(albumDeed(t, book, "into_woods")["done"]) || num(albumDeed(t, book, "three_lands")["current"]) != 2 {
		t.Fatalf("woods %+v", albumDeed(t, book, "three_lands"))
	}
	exp := num(a.player(id)["exp"])
	a.markKill(id, "not_a_beast")
	a.markKill(id, "slime")
	if num(a.player(id)["exp"]) != exp {
		t.Fatal("noting a kill paid exp")
	}
	book = a.album(id)
	slime := albumBeast(t, book, "slime")
	if str(slime["state"]) != "felled" || num(slime["kills"]) != 1 || str(slime["name"]) != "史莱姆" {
		t.Fatalf("felled %+v", slime)
	}
	places := ""
	for _, place := range arr(slime["places"]) {
		places += str(place) + " "
	}
	if !containsText(places, "翠叶森林") {
		t.Fatalf("places %s", places)
	}
	if !truth(albumDeed(t, book, "first_blow")["done"]) || truth(albumDeed(t, book, "ten_hunts")["done"]) {
		t.Fatal("first blow")
	}
	for i := 0; i < 9; i++ {
		a.markKill(id, "slime")
	}
	a.markKill(id, "boss_mushroom")
	book = a.album(id)
	if num(albumBeast(t, book, "slime")["kills"]) != 10 || !truth(albumDeed(t, book, "ten_hunts")["done"]) || !truth(albumDeed(t, book, "boss_mark")["done"]) {
		t.Fatal("hunt tally")
	}
	a.givePet(id, "forest_cat", false)
	a.givePet(id, "forest_cat", false)
	book = a.album(id)
	if num(obj(book["summary"])["pets"]) != 1 || !truth(albumDeed(t, book, "first_bond")["done"]) || truth(albumDeed(t, book, "two_bonds")["done"]) {
		t.Fatalf("one pet %+v", book["summary"])
	}
	a.givePet(id, "fire_fox", false)
	a.run("UPDATE players SET level=5 WHERE id=?", id)
	a.run("UPDATE player_tracks SET level=2 WHERE player_id=? AND track='life:craft'", id)
	a.addGold(id, 200)
	a.createGuild(id, "浮岛会")
	book = a.album(id)
	for _, deed := range []string{"two_bonds", "rank_five", "hand_craft", "joined_guild"} {
		if !truth(albumDeed(t, book, deed)["done"]) {
			t.Fatal(deed)
		}
	}
	fox := albumPet(t, book, "forest_cat")
	if !truth(fox["known"]) || str(fox["name"]) != "森林猫" {
		t.Fatalf("cat %+v", fox)
	}
	rat := albumPet(t, book, "mine_rat")
	if truth(rat["known"]) || str(rat["name"]) != "" {
		t.Fatalf("unknown pet %+v", rat)
	}
}

func TestMeetingDoesNotCountAsAKill(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	a.travel(id, "wild_greenleaf")
	mob := placeOnTemplate(t, a, id, "slime")
	exp := num(a.player(id)["exp"])
	a.engage(id)
	book := a.album(id)
	slime := albumBeast(t, book, "slime")
	if str(slime["state"]) != "met" || num(slime["kills"]) != 0 || str(slime["name"]) != "史莱姆" {
		t.Fatalf("met %+v", slime)
	}
	if num(a.player(id)["exp"]) != exp {
		t.Fatal("meeting paid exp")
	}
	a.killMonster(a.instances[a.live[id].InstanceKey], a.live[id], mob)
	if num(a.player(id)["exp"]) != exp+num(a.def("monsters", "slime")["exp"]) {
		t.Fatalf("exp %v", a.player(id)["exp"])
	}
	book = a.album(id)
	if num(albumBeast(t, book, "slime")["kills"]) != 1 {
		t.Fatal("bout kill was not counted once")
	}
}

func albumDeed(t *testing.T, book M, id string) M {
	t.Helper()
	for _, row := range objects(book["deeds"]) {
		if str(row["id"]) == id {
			return row
		}
	}
	t.Fatalf("missing deed %s", id)
	return nil
}

func albumBeast(t *testing.T, book M, id string) M {
	t.Helper()
	for _, row := range objects(book["beasts"]) {
		if str(row["id"]) == id {
			return row
		}
	}
	t.Fatalf("missing beast %s", id)
	return nil
}

func albumPet(t *testing.T, book M, id string) M {
	t.Helper()
	for _, row := range objects(book["companions"]) {
		if str(row["id"]) == id {
			return row
		}
	}
	t.Fatalf("missing pet %s", id)
	return nil
}

func containsText(text, piece string) bool {
	return len(piece) > 0 && len(text) >= len(piece) && (text == piece || len(text) > 0 && stringIndex(text, piece) >= 0)
}

func stringIndex(text, piece string) int {
	for i := 0; i+len(piece) <= len(text); i++ {
		if text[i:i+len(piece)] == piece {
			return i
		}
	}
	return -1
}
