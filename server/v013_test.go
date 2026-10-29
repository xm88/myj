// V0.1.3：回合战斗、摆摊、当面交易、以及走过地图边界时的连续落点。

package main

import (
	"math"
	"strings"
	"testing"
)

func placeOnSlime(t *testing.T, a *App, id string) M {
	t.Helper()
	l := a.live[id]
	i := a.instances[l.InstanceKey]
	var mob M
	for _, m := range i.Monsters {
		if str(m["templateId"]) == "slime" && truth(m["alive"]) {
			mob = m
			break
		}
	}
	if mob == nil {
		t.Fatal("no slime")
	}
	l.State["x"], l.State["y"], l.State["targetId"] = mob["x"], mob["y"], mob["id"]
	return mob
}

func TestBoutRounds(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	okRequest(t, a, "/api/travel", token, M{"mapId": "wild_greenleaf"})
	mob := placeOnSlime(t, a, id)
	a.engage(id)
	l := a.live[id]
	b := a.bouts[str(l.State["boutId"])]
	if b == nil || b.Phase != "choose" || b.Round != 1 || b.Await != id {
		t.Fatalf("bout open: %+v", b)
	}
	if len(b.Order) < 2 || b.Order[0].Kind != "player" {
		t.Fatalf("turn order: %+v", b.Order)
	}
	hp := num(mob["hp"])
	a.actBout(id, M{"action": "defend"})
	if num(mob["hp"]) != hp {
		t.Fatalf("defend still struck: %v -> %v", hp, mob["hp"])
	}
	if b.Round != 2 || !strings.Contains(strings.Join(b.Log, " "), "守势") {
		t.Fatalf("round after defend: %d %v", b.Round, b.Log)
	}

	a.addEffect(b, str(mob["id"]), RoundEffect{ID: "poison", Name: "中毒", Rounds: 2, DotPct: 0.1})
	hp = num(mob["hp"])
	a.actBout(id, M{"action": "defend"})
	dot := math.Max(1, math.Round(num(mob["maxHp"])*0.1))
	if num(mob["hp"]) != hp-dot {
		t.Fatalf("poison dot %v, hp %v -> %v", dot, hp, mob["hp"])
	}
	left := 0
	for _, ef := range b.Effects[str(mob["id"])] {
		if ef.ID == "poison" {
			left = ef.Rounds
		}
	}
	if left != 1 {
		t.Fatalf("poison rounds left %d", left)
	}

	l.State["hp"] = 40
	before := num(l.State["hp"])
	mob["hp"] = 400
	bread := a.one("SELECT id,count FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.actBout(id, M{"action": "item", "itemId": bread["id"]})
	if num(l.State["hp"]) <= before {
		t.Fatalf("item did not heal: %v -> %v", before, l.State["hp"])
	}
	leftBread := a.one("SELECT count FROM player_items WHERE id=?", bread["id"])
	if leftBread == nil || num(leftBread["count"]) != num(bread["count"])-1 {
		t.Fatalf("bread not consumed: %v", leftBread)
	}

	mob["hp"] = 400
	a.actBout(id, M{"action": "skill", "slot": 1})
	wound := 0
	for mid := range b.Monsters {
		for _, ef := range b.Effects[mid] {
			if ef.ID == "wound" && ef.Rounds > wound {
				wound = ef.Rounds
			}
		}
	}
	if wound < 1 || b.Phase != "choose" {
		t.Fatalf("wound %d phase %s log %v", wound, b.Phase, b.Log)
	}

	gold := num(l.State["gold"])
	var last M
	for mid := range b.Monsters {
		m := a.boutMonster(b, mid)
		if m == nil {
			continue
		}
		if last == nil {
			last = m
			m["hp"] = 1
			continue
		}
		m["alive"], m["hp"] = false, 0
	}
	a.actBout(id, M{"action": "attack"})
	if b.Phase != "victory" {
		t.Fatalf("victory phase %s hp %v", b.Phase, last["hp"])
	}
	if num(l.State["gold"]) <= gold {
		t.Fatal("victory did not grant gold")
	}
	a.dismiss(b)

	mob = placeOnSlime(t, a, id)
	if !truth(mob["alive"]) {
		for _, m := range a.instances[l.InstanceKey].Monsters {
			if str(m["templateId"]) == "slime" && truth(m["alive"]) {
				mob = m
				l.State["x"], l.State["y"], l.State["targetId"] = m["x"], m["y"], m["id"]
			}
		}
	}
	a.engage(id)
	b = a.bouts[str(l.State["boutId"])]
	a.actBout(id, M{"action": "flee"})
	if b.Phase != "fled" {
		t.Fatalf("flee phase %s", b.Phase)
	}
}

func TestStallListAndBuy(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	token2, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	bread := a.one("SELECT id,count FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.openStall(id, "路边摊")
	a.listStall(id, str(bread["id"]), 1, 10)
	a.unlistStall(id, str(a.one("SELECT id FROM stall_goods WHERE stall_id=?", a.live[id].State["stallId"])["id"]))
	back := a.one("SELECT count FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	if num(back["count"]) != num(bread["count"]) {
		t.Fatalf("unlist did not return bread: %v", back)
	}
	bread = a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.listStall(id, str(bread["id"]), 1, 10)
	goods := a.one("SELECT id,price,count FROM stall_goods WHERE stall_id=?", a.live[id].State["stallId"])
	if num(goods["price"]) != 10 || num(goods["count"]) != 1 {
		t.Fatalf("listing: %v", goods)
	}
	sellerGold := num(a.player(id)["gold"])
	buyerGold := num(a.player(id2)["gold"])
	a.buyStall(id2, str(goods["id"]), 1)
	if a.one("SELECT id FROM stall_goods WHERE id=?", goods["id"]) != nil {
		t.Fatal("goods still listed")
	}
	if num(a.player(id)["gold"]) != sellerGold+10 || num(a.player(id2)["gold"]) != buyerGold-10 {
		t.Fatalf("gold seller %v buyer %v", a.player(id)["gold"], a.player(id2)["gold"])
	}
	if num(a.one("SELECT count FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id2)["count"]) != 3 {
		t.Fatal("buyer did not receive bread")
	}
	expectFailure(t, func() { a.buyStall(id, "missing", 1) })
	a.closeStall(id)
	if a.live[id].State["stallId"] != "" {
		t.Fatal("stall still open")
	}
	_ = token
	_ = token2
}

func TestTradeConfirm(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	name2 := str(a.player(id2)["name"])
	a.inviteTrade(id, name2)
	if a.tradeView(id) == nil || a.tradeView(id2) == nil {
		t.Fatal("trade window missing")
	}
	bread := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.offerTrade(id, M{"itemId": bread["id"], "count": 1})
	a.offerTrade(id2, M{"gold": 15})
	view := a.tradeView(id).(M)
	you := obj(view["you"])
	them := obj(view["them"])
	if num(you["gold"]) != 0 || len(objects(you["items"])) != 1 || num(them["gold"]) != 15 {
		t.Fatalf("panes: %v", view)
	}
	a.live[id2].State["x"] = 900
	expectFailure(t, func() { a.lockTrade(id) })
	a.live[id2].State["x"] = num(a.live[id].State["x"])
	a.lockTrade(id)
	a.lockTrade(id2)
	a.confirmTrade(id)
	if a.trades[a.tradeBy[id]] == nil {
		t.Fatal("confirm from one side settled early")
	}
	gold := num(a.player(id)["gold"])
	gold2 := num(a.player(id2)["gold"])
	a.confirmTrade(id2)
	if a.tradeBy[id] != "" || a.tradeBy[id2] != "" {
		t.Fatal("trade still open")
	}
	if num(a.player(id)["gold"]) != gold+15 || num(a.player(id2)["gold"]) != gold2-15 {
		t.Fatalf("gold %v %v", a.player(id)["gold"], a.player(id2)["gold"])
	}
	if num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)["n"]) != 1 {
		t.Fatal("seller bread")
	}
	if num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id2)["n"]) != 3 {
		t.Fatal("buyer bread")
	}

	a.inviteTrade(id, name2)
	a.cancelTrade(id2)
	if a.tradeBy[id] != "" {
		t.Fatal("cancel left the trade")
	}
}

func TestChunkCrossAndSeam(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	l := a.live[id]
	l.State["x"], l.State["y"] = 20, 400
	a.maybeCross(l)
	if str(l.State["mapId"]) != "wild_greenleaf" {
		t.Fatalf("map %v", l.State["mapId"])
	}
	spawn := obj(a.def("maps", "wild_greenleaf")["spawn"])
	if num(l.State["x"]) == num(spawn["x"]) && num(l.State["y"]) == num(spawn["y"]) {
		t.Fatal("crossed onto the map spawn")
	}
	_, dst, linked := a.link("town_starfall", "wild_greenleaf")
	if !linked {
		t.Fatal("town and forest are not paired")
	}
	if math.Hypot(num(l.State["x"])-num(dst["x"]), num(l.State["y"])-num(dst["y"])) > 80 {
		t.Fatalf("arrival not inside the far gate: %v,%v gate %v,%v", l.State["x"], l.State["y"], dst["x"], dst["y"])
	}
	wx, wy := a.worldPos("wild_greenleaf", num(l.State["x"]), num(l.State["y"]))
	gx, gy := a.worldPos("town_starfall", 36, 400)
	if math.Hypot(wx-gx, wy-gy) > 80 {
		t.Fatalf("world seam jumped: (%v,%v) vs gate (%v,%v)", wx, wy, gx, gy)
	}

	seller := a.live[id2]
	a.enter(seller, "wild_greenleaf", false)
	seller.State["x"], seller.State["y"] = num(dst["x"])-40, num(dst["y"])
	a.openStall(id2, "界边摊")
	l.State["mapId"] = "town_starfall"
	l.State["x"], l.State["y"] = 50, 400
	// 人已经在森林分线里，先走回镇的分线再站到门口。
	a.enter(l, "town_starfall", false)
	l.State["x"], l.State["y"] = 50, 400
	if npc := a.nearNPC(id, "npc_verge"); str(npc["name"]) != "阿禾" {
		t.Fatalf("seam npc: %v", npc)
	}
	near := a.stallsNear(id)
	found := false
	for _, s := range near {
		if str(s["title"]) == "界边摊" && truth(s["seam"]) {
			found = true
		}
	}
	if !found {
		t.Fatalf("seam stalls: %v", near)
	}
	goods := a.one("SELECT id FROM stall_goods")
	// 摊上还没有货时先上架再隔门买。
	bread := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id2)
	a.listStall(id2, str(bread["id"]), 1, 8)
	goods = a.one("SELECT id FROM stall_goods WHERE stall_id=?", seller.State["stallId"])
	before := num(a.player(id)["gold"])
	a.buyStall(id, str(goods["id"]), 1)
	if num(a.player(id)["gold"]) != before-8 {
		t.Fatalf("cross-chunk buy gold %v", a.player(id)["gold"])
	}

	inst := a.instances[seller.InstanceKey]
	var mob M
	for _, m := range inst.Monsters {
		if str(m["templateId"]) == "slime" && truth(m["alive"]) {
			mob = m
			break
		}
	}
	if mob == nil {
		t.Fatal("no slime on the far side")
	}
	mob["x"], mob["y"] = num(dst["x"])-24, num(dst["y"])
	l.State["targetId"] = mob["id"]
	a.engage(id)
	b := a.bouts[str(l.State["boutId"])]
	if b == nil || !b.Monsters[str(mob["id"])] {
		t.Fatal("seam engage did not include the far monster")
	}
	a.actBout(id, M{"action": "flee"})
}
