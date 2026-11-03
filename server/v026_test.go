// V0.2.6：引路、抛竿、衣橱和招呼。税、步速、伤害、掉落和门没有改。

package main

import (
	"testing"
)

func TestLeisureConstantsStay(t *testing.T) {
	a := testApp(t)
	if a.constant("marketTax") != 0.05 || a.constant("bagCap") != 60 || a.constant("bankCap") != 80 {
		t.Fatalf("tax=%v bag=%v bank=%v", a.constant("marketTax"), a.constant("bagCap"), a.constant("bankCap"))
	}
	health, _ := request(t, a, "/api/health", "", nil)
	cfg := a.publicConfig()
	if str(health["version"]) != "0.2.6" || str(cfg["version"]) != "0.2.6" {
		t.Fatalf("version health=%v config=%v", health["version"], cfg["version"])
	}
	if len(objects(cfg["sashes"])) != 5 || len(objects(cfg["emotes"])) != 3 || len(objects(cfg["blends"])) != 2 {
		t.Fatalf("catalogs sash=%v emote=%v blend=%v", len(objects(cfg["sashes"])), len(objects(cfg["emotes"])), len(objects(cfg["blends"])))
	}
}

func TestTrailBondIsCosmetic(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	token2, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	name := str(a.player(id)["name"])
	name2 := str(a.player(id2)["name"])
	expectFailure(t, func() { a.askTrail(id, name, "lead") })
	expectFailure(t, func() { a.askTrail(id, name2, "lead") })
	expectFailure(t, func() { a.askTrail(id, name2, "walk") })
	expectFailure(t, func() { a.askTrail(id, name2, "teacher") })
	a.live[id2].State["level"] = 4
	a.run("UPDATE players SET level=4 WHERE id=?", id2)
	okRequest(t, a, "/api/trail/ask", token, M{"name": name2, "role": "walk"})
	if str(obj(a.profile(id2)["trail"])["incoming"].(M)["role"]) != "lead" {
		t.Fatalf("incoming %+v", a.profile(id2)["trail"])
	}
	okRequest(t, a, "/api/trail/cancel", token, M{})
	expectFailure(t, func() { a.acceptTrail(id2) })
	a.askTrail(id, name2, "walk")
	okRequest(t, a, "/api/trail/accept", token2, M{})
	view := obj(a.profile(id)["trail"])
	if str(view["role"]) != "walk" || str(view["peerName"]) != name2 || num(view["bond"]) != 0 {
		t.Fatalf("bonded %+v", view)
	}
	expectFailure(t, func() { a.setTitle(id, "path_known") })
	okRequest(t, a, "/api/trail/line", token, M{"id": "heard"})
	if num(obj(a.profile(id)["trail"])["bond"]) != 0 || !truth(obj(a.profile(id)["trail"])["said"]) {
		t.Fatal("first line moved the bond")
	}
	expectFailure(t, func() { a.sayTrail(id, "heard") })
	expectFailure(t, func() { a.sayTrail(id, "kit") })
	okRequest(t, a, "/api/trail/line", token2, M{"id": "kit"})
	if num(obj(a.profile(id)["trail"])["bond"]) != 1 || num(obj(a.profile(id2)["trail"])["bond"]) != 1 {
		t.Fatalf("bond id=%v id2=%v", a.profile(id)["trail"], a.profile(id2)["trail"])
	}
	for n := 1.; n < trailTitleAt; n++ {
		ageTrail(a, id)
		ageTrail(a, id2)
		a.sayTrail(id, "thanks")
		a.sayTrail(id2, "feed")
	}
	if num(obj(a.profile(id)["trail"])["bond"]) != trailTitleAt {
		t.Fatalf("bond after daily %+v", a.profile(id)["trail"])
	}
	okRequest(t, a, "/api/title", token, M{"id": "path_known"})
	if str(a.plateOf(a.live[id])) != "认得路的人" {
		t.Fatal("title did not show")
	}
	okRequest(t, a, "/api/trail/leave", token, M{})
	if str(obj(a.profile(id)["trail"])["peerName"]) != "" || str(a.plateOf(a.live[id])) != "" {
		t.Fatalf("leave trail=%+v plate=%s", a.profile(id)["trail"], a.plateOf(a.live[id]))
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp {
		t.Fatal("trail changed pace, gold, or exp")
	}
}

func ageTrail(a *App, id string) {
	row := a.trailRow(id)
	row["day"] = guildDay(now()) - 1
	row["said"] = true
	a.writeTrail(id, row)
}

func TestCastLineYieldsFishOnly(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	x, y := num(a.live[id].State["x"]), num(a.live[id].State["y"])
	before := a.countItem(id, "fish_silver")
	a.live[id].State["mapId"] = "wild_greenleaf"
	expectFailure(t, func() { a.castLine(id) })
	a.live[id].State["mapId"] = "town_starfall"
	okRequest(t, a, "/api/cast", token, M{})
	if a.countItem(id, "fish_silver") != before+1 {
		t.Fatalf("fish %v", a.countItem(id, "fish_silver"))
	}
	expectFailure(t, func() { a.castLine(id) })
	if a.countItem(id, "fish_silver") != before+1 {
		t.Fatal("cooldown still granted a fish")
	}
	track := a.track(id, "life:gather")
	if num(track["level"]) != 1 || num(track["exp"]) != 0 {
		t.Fatalf("gather track %+v", track)
	}
	if truth(a.bookRow(id)["hand"]) {
		t.Fatal("casting counted as the daily hand")
	}
	a.live[id].State["safe"] = true
	a.live[id].State["castUntil"] = 0
	a.run("UPDATE players SET cast_at=0 WHERE id=?", id)
	expectFailure(t, func() { a.castLine(id) })
	a.live[id].State["safe"] = false
	a.live[id].State["boutId"] = "bout"
	expectFailure(t, func() { a.castLine(id) })
	a.live[id].State["boutId"] = ""
	okRequest(t, a, "/api/cast", token, M{})
	if a.countItem(id, "fish_silver") != before+2 {
		t.Fatal("second cast after the wait did not land")
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp {
		t.Fatal("casting changed pace, gold, or exp")
	}
	if num(a.live[id].State["x"]) != x || num(a.live[id].State["y"]) != y || a.constant("marketTax") != 0.05 {
		t.Fatal("casting moved the player or the tax")
	}
}

func TestWardrobeSwapsLooksOnly(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	okRequest(t, a, "/api/dye", token, M{"id": "lake"})
	okRequest(t, a, "/api/mount", token, M{"id": "leaf_donkey"})
	okRequest(t, a, "/api/sash", token, M{"id": "tide"})
	expectFailure(t, func() { a.saveLook(id, "") })
	expectFailure(t, func() { a.saveLook(id, "这套名字太长了") })
	okRequest(t, a, "/api/look/save", token, M{"name": "湖边"})
	okRequest(t, a, "/api/dye", token, M{"id": "tea"})
	okRequest(t, a, "/api/mount", token, M{"id": ""})
	okRequest(t, a, "/api/sash", token, M{"id": "plain"})
	if str(a.player(id)["dye_id"]) != "" || str(a.player(id)["sash_id"]) != "" || str(a.player(id)["mount_preview"]) != "" {
		t.Fatal("clearing the look did not stick")
	}
	lookID := str(objects(a.profile(id)["looks"])[0]["id"])
	okRequest(t, a, "/api/look/wear", token, M{"id": lookID})
	if str(a.player(id)["dye_id"]) != "lake" || str(a.player(id)["sash_id"]) != "tide" || str(a.player(id)["mount_preview"]) != "leaf_donkey" {
		t.Fatalf("worn dye=%s sash=%s mount=%s", a.player(id)["dye_id"], a.player(id)["sash_id"], a.player(id)["mount_preview"])
	}
	if str(a.live[id].State["dye"]) != "lake" || str(a.live[id].State["sash"]) != "tide" || str(a.snapshot(a.live[id], now())["self"].(M)["sash"]) != "tide" {
		t.Fatal("snapshot missed the sash")
	}
	a.saveLook(id, "第二")
	a.saveLook(id, "第三")
	expectFailure(t, func() { a.saveLook(id, "第四") })
	a.dropLook(id, lookID)
	if len(a.looksOf(id)) != 2 {
		t.Fatal("drop did not free a slot")
	}
	a.run("UPDATE players SET looks=? WHERE id=?", js([]M{{"id": "high", "name": "高阶", "dye": "pine", "mount": "mist_deer", "sash": "mist"}}), id)
	okRequest(t, a, "/api/look/wear", token, M{"id": "high"})
	if str(a.player(id)["dye_id"]) != "pine" || str(a.player(id)["sash_id"]) != "mist" || str(a.player(id)["mount_preview"]) != "leaf_donkey" {
		t.Fatalf("locked mount dye=%s sash=%s mount=%s", a.player(id)["dye_id"], a.player(id)["sash_id"], a.player(id)["mount_preview"])
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("wardrobe changed the pace")
	}
}

func TestEmoteIsAWave(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	speed := num(a.live[id].Stats["speed"])
	x, y := num(a.live[id].State["x"]), num(a.live[id].State["y"])
	expectFailure(t, func() { a.playEmote(id, "dance") })
	okRequest(t, a, "/api/emote", token, M{"id": "wave"})
	expectFailure(t, func() { a.playEmote(id, "bow") })
	self := obj(a.snapshot(a.live[id], now())["self"])
	if str(self["emote"]) != "wave" || num(self["x"]) != x || num(self["y"]) != y {
		t.Fatalf("self emote %+v", self)
	}
	seen := false
	for _, entity := range objects(a.snapshot(a.live[id2], now())["entities"]) {
		if str(entity["id"]) == id && str(entity["emote"]) == "wave" && str(entity["kind"]) == "player" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("other traveler missed the wave")
	}
	a.live[id].State["emoteAt"] = 0
	a.live[id].State["boutId"] = "bout"
	expectFailure(t, func() { a.playEmote(id, "cheer") })
	a.live[id].State["boutId"] = ""
	a.live[id].State["safe"] = true
	expectFailure(t, func() { a.playEmote(id, "cheer") })
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].State["x"]) != x || num(a.live[id].State["y"]) != y {
		t.Fatal("emote changed pace or place")
	}
}
