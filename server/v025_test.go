// V0.2.5：招牌货架、队伍目标、镇民好感和合剂。税、步速、伤害、掉落和门没有改。

package main

import (
	"strings"
	"testing"
)

func TestStreetConstantsStay(t *testing.T) {
	a := testApp(t)
	if a.constant("bagCap") != 60 || a.constant("bankCap") != 80 || a.constant("starterGold") != 80 || a.constant("marketTax") != 0.05 || a.constant("guildCost") != 200 || a.constant("starterOrbs") != 3 {
		t.Fatalf("caps bag=%v bank=%v gold=%v tax=%v guild=%v orbs=%v", a.constant("bagCap"), a.constant("bankCap"), a.constant("starterGold"), a.constant("marketTax"), a.constant("guildCost"), a.constant("starterOrbs"))
	}
	cfg := a.publicConfig()
	if str(cfg["version"]) != "0.2.6" || len(objects(cfg["blends"])) != 2 || len(objects(cfg["partyAims"])) != 4 || len(objects(cfg["stallShelves"])) != 4 {
		t.Fatalf("config version=%v blends=%v aims=%v shelves=%v", cfg["version"], len(objects(cfg["blends"])), len(objects(cfg["partyAims"])), len(objects(cfg["stallShelves"])))
	}
	// 合剂不写进原来的配方表。
	if a.def("recipes", "gel_pill") != nil || len(objects(a.data["recipes"])) != 16 {
		t.Fatal("blends leaked into the recipe book")
	}
}

func TestStallSignAndShelvesKeepTax(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	token2, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	expectFailure(t, func() { a.setStallSign(id, "凝胶") })
	a.openStall(id, "路边摊")
	expectFailure(t, func() { a.setStallSign(id, "这面招牌实在写得太长了呀多出来啦啊") })
	expectFailure(t, func() { a.setStallSign(id, "便宜!") })
	okRequest(t, a, "/api/stall/sign", token, M{"sign": "凝胶和药草"})
	if str(a.one("SELECT sign FROM stalls WHERE player_id=?", id)["sign"]) != "凝胶和药草" {
		t.Fatal("sign not stored")
	}
	okRequest(t, a, "/api/stall/sign", token, M{"sign": ""})
	if str(a.one("SELECT sign FROM stalls WHERE player_id=?", id)["sign"]) != "" {
		t.Fatal("sign did not clear")
	}
	a.setStallSign(id, "凝胶和药草")
	bread := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.addItem(id, "slime_gel", 4, nil, "bag")
	gel := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)
	a.listStall(id, str(bread["id"]), 1, 40)
	a.listStall(id, str(gel["id"]), 2, 20)
	detail := a.stallDetail(id, str(a.live[id].State["stallId"]))
	if str(detail["sign"]) != "凝胶和药草" {
		t.Fatalf("detail sign %v", detail["sign"])
	}
	shelves := map[string]bool{}
	for _, good := range objects(detail["goods"]) {
		shelves[str(good["shelf"])] = true
		if str(good["templateId"]) == "bread" && (str(good["shelf"]) != "tonic" || num(good["tax"]) != 2 || num(good["net"]) != 38) {
			t.Fatalf("bread shelf/tax %+v", good)
		}
		if str(good["templateId"]) == "slime_gel" && str(good["shelf"]) != "stock" {
			t.Fatalf("gel shelf %+v", good)
		}
	}
	if !shelves["tonic"] || !shelves["stock"] {
		t.Fatalf("shelves %v", shelves)
	}
	near := a.stallsNear(id2)
	if len(near) != 1 || str(near[0]["sign"]) != "凝胶和药草" || !strings.Contains(str(near[0]["shelves"]), "丹食架") || !strings.Contains(str(near[0]["shelves"]), "材料架") {
		t.Fatalf("near %+v", near)
	}
	seller := num(a.player(id)["gold"])
	buyer := num(a.player(id2)["gold"])
	breadGood := a.one("SELECT id FROM stall_goods WHERE stall_id=? AND template_id='bread'", a.live[id].State["stallId"])
	a.buyStall(id2, str(breadGood["id"]), 1)
	if num(a.player(id)["gold"]) != seller+38 || num(a.player(id2)["gold"]) != buyer-40 {
		t.Fatalf("tax seller %v buyer %v", a.player(id)["gold"], a.player(id2)["gold"])
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk || a.constant("marketTax") != 0.05 {
		t.Fatal("sign changed pace or tax")
	}
	_ = token2
}

func TestPartyAimIsAHintOnly(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	speed := num(a.live[id].Stats["speed"])
	speed2 := num(a.live[id2].Stats["speed"])
	interval := num(a.live[id2].Stats["attackInterval"])
	expectFailure(t, func() { a.setPartyAim(id, "hunt") })
	a.inviteParty(id, str(a.player(id2)["name"]))
	a.joinParty(id2)
	expectFailure(t, func() { a.setPartyAim(id2, "hunt") })
	expectFailure(t, func() { a.setPartyAim(id, "siege") })
	a.live[id].State["x"], a.live[id].State["y"] = 700, 400
	a.live[id2].State["x"], a.live[id2].State["y"] = 400, 400
	okRequest(t, a, "/api/party/aim", token, M{"aim": "hunt"})
	view := a.partyView(id2).(M)
	follow := obj(view["follow"])
	if str(view["aim"]) != "hunt" || str(view["aimName"]) != "打怪" || str(follow["compass"]) != "东" || !truth(follow["same"]) || !truth(follow["far"]) || num(follow["gap"]) != 300 {
		t.Fatalf("follow %+v", follow)
	}
	if !strings.Contains(str(follow["text"]), "打怪") || !strings.Contains(str(follow["text"]), "东") {
		t.Fatal(follow["text"])
	}
	lead := obj(a.partyView(id).(M)["follow"])
	if !truth(lead["lead"]) || !strings.Contains(str(lead["text"]), "打怪") {
		t.Fatalf("leader %+v", lead)
	}
	snap := a.snapshot(a.live[id2], now())
	if str(obj(snap["follow"])["compass"]) != "东" {
		t.Fatalf("snapshot %+v", snap["follow"])
	}
	if str(a.profile(id)["companionOrder"]) != "steady" {
		t.Fatal("aim rewrote the combat order")
	}
	ax, ay := num(a.live[id2].State["x"]), num(a.live[id2].State["y"])
	a.travel(id, "wild_greenleaf")
	follow = obj(a.partyView(id2).(M)["follow"])
	if truth(follow["same"]) || !strings.Contains(str(follow["text"]), "翠叶森林") || num(a.live[id2].State["x"]) != ax || num(a.live[id2].State["y"]) != ay {
		t.Fatalf("cross map follow %+v pos %v %v", follow, a.live[id2].State["x"], a.live[id2].State["y"])
	}
	if num(a.live[id].Stats["speed"]) != speed || num(a.live[id2].Stats["speed"]) != speed2 || num(a.live[id2].Stats["attackInterval"]) != interval {
		t.Fatal("aim changed the pace")
	}
}

func TestRegardIsCosmetic(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	a.live[id].State["x"], a.live[id].State["y"] = 860, 430
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	d := a.dialog(id, "npc_shop")
	regard := obj(d["regard"])
	if num(regard["n"]) != 1 || str(regard["tierName"]) != "见过" {
		t.Fatalf("first talk %+v", regard)
	}
	again := obj(a.dialog(id, "npc_shop")["regard"])
	if num(again["n"]) != 1 {
		t.Fatal("same day talk stacked")
	}
	a.run("UPDATE players SET regard=? WHERE id=?", js(M{"npc_shop": M{"n": 1, "talk": guildDay(now()) - 1}}), id)
	if num(obj(a.dialog(id, "npc_shop")["regard"])["n"]) != 2 {
		t.Fatal("next day did not move")
	}
	a.run("UPDATE players SET regard=? WHERE id=?", js(M{"npc_shop": M{"n": 8, "talk": guildDay(now()) - 1}}), id)
	known := a.dialog(id, "npc_shop")
	if num(obj(known["regard"])["n"]) != 9 || !strings.Contains(str(known["text"]), "熟客") {
		t.Fatalf("known %+v text %v", known["regard"], known["text"])
	}
	a.setTitle(id, "town_face")
	if str(a.plateOf(a.live[id])) != "镇上的熟脸" || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("title changed the pace")
	}
	a.setTitle(id, "")
	a.run("UPDATE players SET regard=? WHERE id=?", js(M{"npc_shop": M{"n": 2, "talk": guildDay(now()), "gift": guildDay(now()) - 1}}), id)
	expectFailure(t, func() { a.giveRegard(id, "npc_shop") })
	a.addItem(id, "slime_gel", 1, nil, "bag")
	okRequest(t, a, "/api/dialog", token, M{"npcId": "npc_shop", "action": "gift"})
	if a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='slime_gel'", id) != nil {
		t.Fatal("gift stayed in the bag")
	}
	if num(a.regardN(id, "npc_shop")) != 3 || num(a.player(id)["gold"]) != gold {
		t.Fatalf("gift n=%v gold=%v", a.regardN(id, "npc_shop"), a.player(id)["gold"])
	}
	a.addItem(id, "slime_gel", 1, nil, "bag")
	expectFailure(t, func() { a.giveRegard(id, "npc_shop") })
	if num(a.one("SELECT count FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)["count"]) != 1 {
		t.Fatal("second gift consumed the gel")
	}
	a.run("UPDATE players SET regard=? WHERE id=?", js(M{"npc_shop": M{"n": 9, "talk": guildDay(now())}}), id)
	a.addItem(id, "slime_gel", 1, nil, "bag")
	expectFailure(t, func() { a.giveRegard(id, "npc_shop") })
	a.setTitle(id, "town_face")
	if str(a.plateOf(a.live[id])) != "镇上的熟脸" {
		t.Fatal("plate")
	}
	a.live[id].State["x"], a.live[id].State["y"] = 960, 500
	plain := a.dialog(id, "npc_bank")
	if plain["regard"] != nil {
		t.Fatal("warehouse keeper gained regard")
	}
	if num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp || num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("regard changed gold, exp, or pace")
	}
}

func TestBlendUsesBagThenBank(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	a.addItem(id, "slime_gel", 2, nil, "bag")
	a.addItem(id, "slime_gel", 5, nil, "bank")
	if failed, status := request(t, a, "/api/craft", token, M{"recipeId": "recipe_potion_slime"}); status == 200 {
		t.Fatalf("craft used the warehouse: %v", failed)
	}
	if a.stockCount(id, "slime_gel") != 7 || num(a.player(id)["gold"]) != gold {
		t.Fatal("craft consumed bank or gold")
	}
	quests := a.rows("SELECT quest_id,status,progress FROM player_quests WHERE player_id=? ORDER BY quest_id", id)
	okRequest(t, a, "/api/blend", token, M{"id": "gel_pill"})
	bag := num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bag'", id)["n"])
	bank := num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='slime_gel' AND location='bank'", id)["n"])
	if bag != 0 || bank != 4 {
		t.Fatalf("blend bag=%v bank=%v", bag, bank)
	}
	if num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='potion_hp' AND location='bag'", id)["n"]) != 1 {
		t.Fatal("no pill")
	}
	if num(a.player(id)["gold"]) != gold-5 || num(a.player(id)["exp"]) != exp {
		t.Fatal("blend changed the wrong purse")
	}
	cook := a.track(id, "life:cook")
	if num(cook["level"]) != 1 || num(cook["exp"]) != 0 {
		t.Fatalf("blend raised cooking %+v", cook)
	}
	after := a.rows("SELECT quest_id,status,progress FROM player_quests WHERE player_id=? ORDER BY quest_id", id)
	if len(after) != len(quests) {
		t.Fatal("blend opened a quest")
	}
	for i := range quests {
		if str(quests[i]["quest_id"]) != str(after[i]["quest_id"]) || str(quests[i]["status"]) != str(after[i]["status"]) || str(quests[i]["progress"]) != str(after[i]["progress"]) {
			t.Fatalf("quest moved %v -> %v", quests[i], after[i])
		}
	}
	expectFailure(t, func() { a.blend(id, "nope") })
	a.addItem(id, "moon_dew", 1, nil, "bank")
	a.addItem(id, "herb_green", 2, nil, "bank")
	before := num(a.player(id)["gold"])
	a.blend(id, "dew_tea")
	if num(a.player(id)["gold"]) != before-8 || a.stockCount(id, "moon_dew") != 0 || a.stockCount(id, "herb_green") != 0 {
		t.Fatal("dew tea did not take the warehouse")
	}
	if num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='potion_mp' AND location='bag'", id)["n"]) != 1 {
		t.Fatal("no tea")
	}
	if num(a.track(id, "life:cook")["exp"]) != 0 || a.constant("marketTax") != 0.05 {
		t.Fatal("tea changed skill or tax")
	}
}
