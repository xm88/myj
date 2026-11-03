// V0.2.6：引路、抛竿、衣橱和招呼。税、步速、伤害、掉落和门没有改。

package main

import (
	"strings"
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
	relaxTrail(a, id)
	okRequest(t, a, "/api/trail/cancel", token, M{})
	expectFailure(t, func() { a.acceptTrail(id2) })
	relaxTrail(a, id)
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

func TestTrailLastLineStays(t *testing.T) {
	a := testApp(t)
	_, id, _, id2 := bondTrail(t, a)
	speed := num(a.live[id].Stats["speed"])
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	ear := listen(a, id2)
	a.sayTrail(id, "heard")
	if !hasToast(ear, "记下了，我先去试试。") {
		t.Fatal("partner missed the first line")
	}
	if str(obj(a.profile(id2)["trail"])["peerLine"]) != "记下了，我先去试试。" || num(obj(a.profile(id)["trail"])["bond"]) != 0 {
		t.Fatalf("first line id=%+v id2=%+v", a.profile(id)["trail"], a.profile(id2)["trail"])
	}
	if str(obj(parse(str(a.player(id)["trail"])))["line"]) != "heard" {
		t.Fatal("first line id was not stored")
	}
	a.live[id].Peer = nil
	a.sayTrail(id2, "kit")
	view := obj(a.profile(id)["trail"])
	if str(view["peerLine"]) != "先把伤药放在顺手的格子。" || num(view["bond"]) != 1 || num(obj(a.profile(id2)["trail"])["bond"]) != 1 {
		t.Fatalf("completing line %+v", view)
	}
	if str(obj(parse(str(a.player(id2)["trail"])))["line"]) != "kit" {
		t.Fatal("completing line id was not stored")
	}
	for n := 0; n < 8 && num(obj(a.profile(id)["trail"])["bond"]) < trailBondMax; n++ {
		ageTrail(a, id)
		ageTrail(a, id2)
		a.sayTrail(id, "thanks")
		a.sayTrail(id2, "feed")
	}
	if num(obj(a.profile(id)["trail"])["bond"]) != trailBondMax || num(obj(a.profile(id2)["trail"])["bond"]) != trailBondMax {
		t.Fatalf("cap %+v", a.profile(id)["trail"])
	}
	ageTrail(a, id)
	ageTrail(a, id2)
	a.sayTrail(id, "heard")
	if num(obj(a.profile(id)["trail"])["bond"]) != trailBondMax {
		t.Fatal("one side moved a full bond")
	}
	ageTrail(a, id)
	ageTrail(a, id2)
	ear = listen(a, id2)
	a.sayTrail(id2, "feed")
	a.sayTrail(id, "thanks")
	if num(obj(a.profile(id)["trail"])["bond"]) != trailBondMax {
		t.Fatal("bond grew past the cap")
	}
	if str(obj(a.profile(id2)["trail"])["peerLine"]) != "今天跟着走了一段。" || str(obj(a.profile(id)["trail"])["peerLine"]) != "灵兽饿了，喂一口就肯跟着。" {
		t.Fatalf("capped lines id=%+v id2=%+v", a.profile(id)["trail"], a.profile(id2)["trail"])
	}
	if !hasToast(ear, "今天跟着走了一段。") {
		t.Fatal("capped line was not delivered")
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp || a.constant("marketTax") != 0.05 {
		t.Fatal("trail lines changed pace, gold, exp, or tax")
	}
}

func TestTrailInvitePace(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.live[id2].State["level"] = 4
	a.run("UPDATE players SET level=4 WHERE id=?", id2)
	name2 := str(a.player(id2)["name"])
	ear := listen(a, id2)
	okRequest(t, a, "/api/trail/ask", token, M{"name": name2, "role": "walk"})
	left := num(obj(parse(str(a.player(id)["trail"])))["askAt"]) - now()
	if left < trailGap-500 || left > trailGap+500 {
		t.Fatalf("invite gap %v", left)
	}
	if !hasToast(ear, "想和你结一段路谊") {
		t.Fatal("invite did not reach the partner")
	}
	expectMessage(t, "路谊的邀请太密了", func() { a.cancelTrail(id) })
	if str(obj(a.profile(id)["trail"])["outgoing"].(M)["name"]) != name2 {
		t.Fatal("paced cancel dropped the invite")
	}
	if len(toastTexts(ear)) != 0 {
		t.Fatal("paced cancel still toasted the partner")
	}
	relaxTrail(a, id)
	okRequest(t, a, "/api/trail/cancel", token, M{})
	if !hasToast(ear, "收回了路谊邀请") {
		t.Fatal("cancel did not reach the partner")
	}
	expectMessage(t, "路谊的邀请太密了", func() { a.askTrail(id, name2, "walk") })
	if obj(a.profile(id)["trail"])["outgoing"] != nil || len(toastTexts(ear)) != 0 {
		t.Fatal("paced invite went out again")
	}
	relaxTrail(a, id)
	okRequest(t, a, "/api/trail/ask", token, M{"name": name2, "role": "walk"})
	if str(obj(a.profile(id2)["trail"])["incoming"].(M)["name"]) == "" || !hasToast(ear, "想和你结一段路谊") {
		t.Fatal("invite did not resume after the gap")
	}
}

func TestCastMailsWhenBagIsFull(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	ear := listen(a, id)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	gold := num(a.player(id)["gold"])
	x, y := num(a.live[id].State["x"]), num(a.live[id].State["y"])
	a.live[id].State["mapId"] = "town_starfall"
	for a.slots(id, "bag") < a.constant("bagCap") {
		a.run("INSERT INTO player_items(id,player_id,template_id,count,location,created_at) VALUES(?,?,?,1,'bag',?)", uid(), id, "pad_"+uid(), now())
	}
	before := a.countItem(id, "fish_silver")
	mails := len(a.listMail(id))
	okRequest(t, a, "/api/cast", token, M{})
	if a.countItem(id, "fish_silver") != before || a.slots(id, "bag") != a.constant("bagCap") {
		t.Fatalf("bag fish=%v slots=%v", a.countItem(id, "fish_silver"), a.slots(id, "bag"))
	}
	if len(a.listMail(id)) != mails+1 || mailedFish(a, id) != 1 {
		t.Fatalf("mail count=%d fish=%d", len(a.listMail(id)), mailedFish(a, id))
	}
	text := strings.Join(toastTexts(ear), "\n")
	if !strings.Contains(text, "邮箱") || strings.Contains(text, "钓上") {
		t.Fatalf("toast %q", text)
	}
	expectFailure(t, func() { a.castLine(id) })
	if a.countItem(id, "fish_silver") != before || mailedFish(a, id) != 1 {
		t.Fatal("cooldown cast duplicated the fish")
	}
	pad := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id LIKE 'pad_%' LIMIT 1", id)
	a.run("DELETE FROM player_items WHERE id=?", pad["id"])
	a.run("INSERT INTO player_items(id,player_id,template_id,count,location,created_at) VALUES(?,?,?,1,'bag',?)", uid(), id, "fish_silver", now())
	a.live[id].State["castUntil"] = 0
	a.run("UPDATE players SET cast_at=0 WHERE id=?", id)
	okRequest(t, a, "/api/cast", token, M{})
	text = strings.Join(toastTexts(ear), "\n")
	if a.countItem(id, "fish_silver") != before+2 || mailedFish(a, id) != 1 || !strings.Contains(text, "钓上") || strings.Contains(text, "邮箱") {
		t.Fatalf("stack fish=%v mail=%d toast=%q", a.countItem(id, "fish_silver"), mailedFish(a, id), text)
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || num(a.player(id)["gold"]) != gold || num(a.live[id].State["x"]) != x || num(a.live[id].State["y"]) != y || a.constant("marketTax") != 0.05 {
		t.Fatal("full bag cast changed pace, gold, place, or tax")
	}
}

func bondTrail(t *testing.T, a *App) (string, string, string, string) {
	t.Helper()
	token, id := newGuest(t, a)
	token2, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.live[id2].State["level"] = 4
	a.run("UPDATE players SET level=4 WHERE id=?", id2)
	a.askTrail(id, str(a.player(id2)["name"]), "walk")
	a.acceptTrail(id2)
	return token, id, token2, id2
}

func relaxTrail(a *App, id string) {
	row := a.trailRow(id)
	row["askAt"] = 0
	a.writeTrail(id, row)
}

func listen(a *App, id string) *Peer {
	p := &Peer{out: make(chan []byte, 64), done: make(chan struct{})}
	a.live[id].Peer = p
	return p
}

func toastTexts(p *Peer) []string {
	out := []string{}
	for {
		select {
		case b := <-p.out:
			msg := obj(parse(string(b)))
			if str(msg["type"]) == "toast" {
				out = append(out, str(msg["text"]))
			}
		default:
			return out
		}
	}
}

func hasToast(p *Peer, part string) bool {
	for _, text := range toastTexts(p) {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

func expectMessage(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		e := recover()
		if e == nil {
			t.Fatal("expected failure")
		}
		ge, ok := e.(GameError)
		if !ok {
			panic(e)
		}
		if ge.Message != want {
			t.Fatalf("got %q want %q", ge.Message, want)
		}
	}()
	fn()
}

func mailedFish(a *App, id string) int {
	n := 0
	for _, row := range a.listMail(id) {
		for _, item := range objects(obj(row["attachments"])["items"]) {
			if str(item["templateId"]) == "fish_silver" {
				n += int(num(item["count"]))
			}
		}
	}
	return n
}
