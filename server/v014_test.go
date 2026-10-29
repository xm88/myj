// V0.1.4：自由镜头的连续地块、双向门、同伴 AI、摆摊旅税。

package main

import (
	"math"
	"strings"
	"testing"
)

func TestPairedGatesShareWorld(t *testing.T) {
	a := testApp(t)
	pairs := [][2]string{
		{"town_starfall", "wild_greenleaf"},
		{"town_starfall", "wild_redrock"},
		{"town_starfall", "wild_moonlake"},
		{"town_starfall", "wild_thunder"},
		{"wild_thunder", "wild_windhigh"},
		{"wild_greenleaf", "wild_moonlake"},
	}
	for _, pair := range pairs {
		src, dst, ok := a.link(pair[0], pair[1])
		if !ok {
			t.Fatalf("one-way link %s -> %s", pair[0], pair[1])
		}
		ax, ay := a.worldPos(pair[0], num(src["x"]), num(src["y"]))
		bx, by := a.worldPos(pair[1], num(dst["x"]), num(dst["y"]))
		if math.Hypot(ax-bx, ay-by) > 0.01 {
			t.Fatalf("%s/%s gates differ (%v,%v) vs (%v,%v)", pair[0], pair[1], ax, ay, bx, by)
		}
	}
	tox, toy, _ := a.originOf("wild_thunder")
	wox, woy, _ := a.originOf("wild_windhigh")
	th := num(a.def("maps", "wild_thunder")["height"])
	wh := num(a.def("maps", "wild_windhigh")["height"])
	if tox == 0 && toy == -th {
		t.Fatal("thunder is still the unlinked fallback north of town")
	}
	if wox == 0 && woy == -wh {
		t.Fatal("wind is still the unlinked fallback north of town")
	}
	if woy >= toy {
		t.Fatalf("wind should sit north of thunder, origins %v and %v", woy, toy)
	}
	fx0, _, _ := a.originOf("wild_greenleaf")
	fw := num(a.def("maps", "wild_greenleaf")["width"])
	if fx0 >= 0 || fx0+fw > 160 {
		t.Fatalf("forest should tile west of town, origin %v width %v", fx0, fw)
	}
}

func TestFreeCameraChunks(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	l := a.live[id]
	l.State["x"], l.State["y"] = 36, 400
	snap := a.snapshot(l, now())
	if num(snap["camera"]) != cameraSpan {
		t.Fatalf("camera span %v", snap["camera"])
	}
	ids := map[string]M{}
	for _, c := range objects(snap["chunks"]) {
		ids[str(c["id"])] = c
	}
	for _, want := range []string{"town_starfall", "wild_greenleaf", "wild_redrock", "wild_moonlake", "wild_thunder"} {
		if ids[want] == nil {
			t.Fatalf("missing neighbor %s in %v", want, ids)
		}
	}
	if ids["wild_windhigh"] != nil || ids["dungeon_ruins"] != nil {
		t.Fatalf("camera included a non-neighbor: %v", ids)
	}
	fx0 := num(ids["wild_greenleaf"]["originX"])
	half := cameraSpan / 2
	left := math.Max(fx0, 36-half)
	right := math.Min(0, 36+half)
	if right-left < 40 {
		t.Fatalf("west-gate camera does not show forest outside town: %v..%v", left, right)
	}
	self := obj(snap["self"])
	if math.Hypot(num(self["worldX"])-36, num(self["worldY"])-400) > 1 {
		t.Fatalf("world position %v %v", self["worldX"], self["worldY"])
	}

	l.State["level"] = 30
	a.enter(l, "dungeon_ruins", true)
	snap = a.snapshot(l, now())
	chunks := objects(snap["chunks"])
	if len(chunks) != 1 || str(chunks[0]["id"]) != "dungeon_ruins" {
		t.Fatalf("dungeon should stay a single chunk: %v", chunks)
	}
	if num(snap["camera"]) != cameraSpan {
		t.Fatal("dungeon camera missing")
	}
}

func TestThunderWindRoundTrip(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	l := a.live[id]
	l.State["level"] = 30
	l.State["x"], l.State["y"] = 600, 20
	a.maybeCross(l)
	if str(l.State["mapId"]) != "wild_thunder" {
		t.Fatalf("north gate stayed on %v", l.State["mapId"])
	}
	src, dst, ok := a.link("town_starfall", "wild_thunder")
	if !ok {
		t.Fatal("thunder return gate missing")
	}
	if math.Hypot(num(l.State["x"])-num(dst["x"]), num(l.State["y"])-num(dst["y"])) > 80 {
		t.Fatalf("thunder arrival %v,%v", l.State["x"], l.State["y"])
	}
	wx, wy := a.worldPos("wild_thunder", num(l.State["x"]), num(l.State["y"]))
	gx, gy := a.worldPos("town_starfall", num(src["x"]), num(src["y"]))
	if math.Hypot(wx-gx, wy-gy) > 80 {
		t.Fatalf("thunder seam (%v,%v) vs (%v,%v)", wx, wy, gx, gy)
	}

	l.State["crossUntil"] = 0
	north, _, ok := a.link("wild_thunder", "wild_windhigh")
	if !ok {
		t.Fatal("wind return gate missing")
	}
	l.State["x"], l.State["y"] = num(north["x"]), 20
	a.maybeCross(l)
	if str(l.State["mapId"]) != "wild_windhigh" {
		t.Fatalf("wind gate stayed on %v at %v,%v", l.State["mapId"], l.State["x"], l.State["y"])
	}
	l.State["crossUntil"] = 0
	back, _, _ := a.link("wild_windhigh", "wild_thunder")
	l.State["x"] = num(back["x"])
	l.State["y"] = num(a.def("maps", "wild_windhigh")["height"]) - 16
	a.maybeCross(l)
	if str(l.State["mapId"]) != "wild_thunder" {
		t.Fatalf("return from wind %v", l.State["mapId"])
	}
	l.State["crossUntil"] = 0
	townGate, _, _ := a.link("wild_thunder", "town_starfall")
	l.State["x"] = num(townGate["x"])
	l.State["y"] = num(a.def("maps", "wild_thunder")["height"]) - 16
	a.maybeCross(l)
	if str(l.State["mapId"]) != "town_starfall" {
		t.Fatalf("return from thunder %v", l.State["mapId"])
	}
}

func TestCompanionChooses(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	l := a.live[id]
	msg := a.companionAction(nil, id)
	if str(msg["action"]) != "skill" || num(msg["slot"]) != 3 {
		t.Fatalf("healthy choice %+v", msg)
	}
	l.State["mp"] = 0
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "attack" {
		t.Fatalf("no mana %+v", msg)
	}
	l.State["mp"] = num(l.Stats["maxMp"])
	l.State["hp"] = 10
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "item" || str(msg["itemId"]) == "" {
		t.Fatalf("potion %+v", msg)
	}
	a.run("DELETE FROM player_items WHERE player_id=? AND template_id='bread'", id)
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "skill" || num(msg["slot"]) != 4 {
		t.Fatalf("heal %+v", msg)
	}
	l.State["mp"] = 0
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "defend" {
		t.Fatalf("defend %+v", msg)
	}
}

func TestCompanionActsForMate(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.inviteParty(id, str(a.player(id2)["name"]))
	a.joinParty(id2)
	a.enter(a.live[id], "wild_greenleaf", false)
	a.enter(a.live[id2], "wild_greenleaf", false)
	if a.live[id].InstanceKey != a.live[id2].InstanceKey {
		t.Fatal("party split across lines")
	}
	mob := placeOnSlime(t, a, id)
	a.live[id2].State["x"], a.live[id2].State["y"] = mob["x"], mob["y"]
	for _, m := range a.instances[a.live[id].InstanceKey].Monsters {
		m["hp"], m["maxHp"] = 8000, 8000
	}
	a.live[id].State["hp"], a.live[id].Stats["maxHp"] = 50000, 50000
	a.live[id2].State["hp"], a.live[id2].Stats["maxHp"] = 50000, 50000
	a.live[id].Stats["speed"] = 20
	a.live[id2].Stats["speed"] = 5000
	before := num(a.live[id2].State["mp"])
	a.engage(id)
	b := a.bouts[str(a.live[id].State["boutId"])]
	if b == nil || b.Leader != id || b.Await != id {
		t.Fatalf("leader should still choose: leader %v await %v", b.Leader, b)
	}
	if !b.Players[id2] {
		t.Fatal("mate was not pulled in")
	}
	if num(a.live[id2].State["mp"]) >= before {
		t.Fatalf("mate did not cast, log %v", b.Log)
	}
	if !strings.Contains(strings.Join(b.Log, "\n"), "盾牌猛击") {
		t.Fatalf("mate log %v", b.Log)
	}
}

func TestCompanionTimeoutUsesAI(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	a.enter(a.live[id], "wild_greenleaf", false)
	placeOnSlime(t, a, id)
	for _, m := range a.instances[a.live[id].InstanceKey].Monsters {
		m["hp"], m["maxHp"] = 8000, 8000
	}
	lhp := a.live[id]
	lhp.State["hp"], lhp.Stats["maxHp"] = 50000, 50000
	a.engage(id)
	l := a.live[id]
	b := a.bouts[str(l.State["boutId"])]
	b.WaitSince = now() - 21000
	before := num(l.State["mp"])
	a.tickBouts(now())
	if num(l.State["mp"]) >= before || !strings.Contains(strings.Join(b.Log, "\n"), "盾牌猛击") {
		t.Fatalf("timeout still basic-attacked, mp %v log %v", l.State["mp"], b.Log)
	}
}

func TestStallTaxMatchesAuction(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.addGold(id2, 400)
	a.live[id2].State["x"], a.live[id2].State["y"] = a.live[id].State["x"], a.live[id].State["y"]
	bread := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag'", id)
	a.openStall(id, "税摊")
	a.listStall(id, str(bread["id"]), 1, 200)
	goods := a.one("SELECT id FROM stall_goods")
	rate := a.constant("marketTax")
	tax := math.Floor(200 * rate)
	if tax != math.Floor(200*0.05) || tax <= 0 {
		t.Fatalf("tax %v rate %v", tax, rate)
	}
	seller := num(a.player(id)["gold"])
	buyer := num(a.player(id2)["gold"])
	a.buyStall(id2, str(goods["id"]), 1)
	if num(a.player(id2)["gold"]) != buyer-200 {
		t.Fatalf("buyer paid %v", a.player(id2)["gold"])
	}
	if num(a.player(id)["gold"]) != seller+200-tax {
		t.Fatalf("seller received %v, want %v", a.player(id)["gold"], seller+200-tax)
	}

	a.addItem(id, "bread", 3, nil, "bag")
	row := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='bread' AND location='bag' AND count>=2", id)
	a.listStall(id, str(row["id"]), 2, 80)
	goods = a.one("SELECT id, count, price FROM stall_goods WHERE count=2")
	seller = num(a.player(id)["gold"])
	buyer = num(a.player(id2)["gold"])
	a.buyStall(id2, str(goods["id"]), 1)
	pay := math.Floor(80 * 1 / 2)
	tax = math.Floor(pay * rate)
	if num(a.player(id2)["gold"]) != buyer-pay {
		t.Fatalf("partial buyer %v pay %v", a.player(id2)["gold"], pay)
	}
	if num(a.player(id)["gold"]) != seller+pay-tax {
		t.Fatalf("partial seller %v want %v", a.player(id)["gold"], seller+pay-tax)
	}
	left := a.one("SELECT count, price FROM stall_goods WHERE id=?", goods["id"])
	if num(left["count"]) != 1 || num(left["price"]) != 80-pay {
		t.Fatalf("remainder %v", left)
	}
}
