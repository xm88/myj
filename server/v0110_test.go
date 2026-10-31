// V0.1.10：装备淬炼、制造提示、同伴号令，以及天气下一候。

package main

import (
	"math"
	"strings"
	"testing"
)

func TestTemperMath(t *testing.T) {
	if g, gold := temperNeed(0); g != 1 || gold != 8 {
		t.Fatalf("first temper cost %v %v", g, gold)
	}
	if g, gold := temperNeed(2); g != 2 || gold != 20 {
		t.Fatalf("third temper cost %v %v", g, gold)
	}
	if g, gold := temperNeed(5); g != 3 || gold != 38 {
		t.Fatalf("last temper cost %v %v", g, gold)
	}
	if math.Abs(temperChance(0, 1)-0.9) > 1e-9 || math.Abs(temperChance(5, 1)-0.4) > 1e-9 {
		t.Fatalf("chance %v %v", temperChance(0, 1), temperChance(5, 1))
	}
	if math.Abs(temperChance(5, 10)-0.58) > 1e-9 {
		t.Fatalf("craft chance %v", temperChance(5, 10))
	}
	if temperSteps(0, 6, false, true) != 0 || temperSteps(0, 6, true, false) != 1 {
		t.Fatal("plain steps")
	}
	if temperSteps(4, 6, true, true) != 2 || temperSteps(5, 6, true, true) != 1 || temperSteps(6, 6, true, false) != 0 {
		t.Fatal("spark steps")
	}
}

func TestTemperAndCraft(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	p := a.profile(id)
	weapon := obj(obj(p["equipment"])["weapon"])
	wid := str(weapon["id"])
	if wid == "" || num(weapon["temper"]) != 0 {
		t.Fatalf("starter weapon %+v", weapon)
	}
	gold := num(p["gold"])
	if _, code := request(t, a, "/api/temper", token, M{"itemId": wid}); code != 400 || num(a.profile(id)["gold"]) != gold {
		t.Fatal("missing grit should not spend gold")
	}
	bread := ""
	for _, item := range objects(a.profile(id)["bag"]) {
		if str(item["templateId"]) == "bread" {
			bread = str(item["id"])
		}
	}
	if _, code := request(t, a, "/api/temper", token, M{"itemId": bread}); code != 400 {
		t.Fatal("bread tempered")
	}

	a.addItem(id, "temper_grit", 40, nil, "bag")
	a.addGold(id, 400)
	before := num(a.live[id].Stats["atk"])
	raised := false
	for i := 0; i < 16 && !raised; i++ {
		res, code := request(t, a, "/api/temper", token, M{"itemId": wid})
		if code != 200 {
			t.Fatalf("temper %d: %v", i, res)
		}
		if num(obj(obj(res["equipment"])["weapon"])["temper"]) >= 1 {
			raised = true
		}
	}
	if !raised {
		t.Fatal("temper never landed")
	}
	if num(a.live[id].Stats["atk"]) <= before {
		t.Fatal("equipped temper did not raise attack")
	}
	row := a.one("SELECT extra FROM player_items WHERE id=?", wid)
	ex := extra(row["extra"])
	ex["temper"] = temperCap
	a.run("UPDATE player_items SET extra=? WHERE id=?", js(ex), wid)
	grit := a.countItem(id, "temper_grit")
	if _, code := request(t, a, "/api/temper", token, M{"itemId": wid}); code != 400 || a.countItem(id, "temper_grit") != grit {
		t.Fatal("full temper consumed grit")
	}
	ex["temper"] = 3
	a.run("UPDATE player_items SET extra=? WHERE id=?", js(ex), wid)
	view := a.viewItem(a.takeItem(id, wid), 1)
	atk := 0.0
	for _, line := range objects(view["lines"]) {
		if str(line["stat"]) == "atk" {
			atk = num(line["value"])
		}
	}
	if atk != 7.4 || num(view["temperChance"]) != 60 {
		t.Fatalf("temper preview atk %v chance %v", atk, view["temperChance"])
	}

	a.addItem(id, "wood", 1, nil, "bag")
	a.addItem(id, "herb_green", 1, nil, "bag")
	cooked := okRequest(t, a, "/api/craft", token, M{"recipeId": "recipe_bread"})
	breads := 0.0
	for _, item := range objects(cooked["bag"]) {
		if str(item["templateId"]) == "bread" {
			breads += num(item["count"])
		}
	}
	if breads < 4 || num(obj(obj(cooked["tracks"])["cook"])["exp"]) < 12 {
		t.Fatalf("craft feedback %+v bread %v", cooked["tracks"], breads)
	}
}

func TestCompanionOrdersAndSky(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	l := a.live[id]
	maxHp := num(l.Stats["maxHp"])
	l.State["hp"] = maxHp * 0.30
	msg := a.companionAction(nil, id)
	if str(msg["action"]) != "item" || str(msg["order"]) != "steady" {
		t.Fatalf("steady %+v", msg)
	}
	if _, code := request(t, a, "/api/party/order", token, M{"order": "飞"}); code != 400 {
		t.Fatal("bad order")
	}
	pressed := okRequest(t, a, "/api/party/order", token, M{"order": "press"})
	if str(pressed["companionOrder"]) != "press" || str(pressed["companionOrderName"]) != "猛攻" {
		t.Fatalf("saved order %+v", pressed["companionOrderName"])
	}
	msg = a.companionAction(nil, id)
	if str(msg["action"]) == "item" || str(msg["action"]) == "defend" {
		t.Fatalf("press at 30%% %+v", msg)
	}
	l.State["hp"] = maxHp
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "skill" || num(msg["slot"]) != 3 {
		t.Fatalf("press still casts %+v", msg)
	}
	a.setCompanionOrder(id, "cover")
	l.State["hp"] = maxHp * 0.60
	msg = a.companionAction(nil, id)
	if str(msg["action"]) != "item" {
		t.Fatalf("cover drinks early %+v", msg)
	}
	a.setCompanionOrder(id, "focus")
	l.State["hp"] = maxHp
	l.State["targetId"] = "mob-b"
	msg = a.companionAction(&Bout{Leader: id}, id)
	if str(msg["focusId"]) != "mob-b" || str(msg["orderName"]) != "集火" {
		t.Fatalf("focus %+v", msg)
	}

	home := "line-test"
	a.instances[home] = &Instance{Key: home, Monsters: map[string]M{
		"mob-a": {"id": "mob-a", "alive": true, "name": "甲"},
		"mob-b": {"id": "mob-b", "alive": false, "name": "乙"},
	}}
	b := &Bout{Home: map[string]string{"mob-a": home, "mob-b": home}, Order: []BoutActor{{ID: "mob-a", Kind: "monster"}, {ID: "mob-b", Kind: "monster"}}}
	if _, got := a.pickFoe(b, "mob-b"); got != "mob-a" {
		t.Fatalf("dead focus fell through to %s", got)
	}
	a.instances[home].Monsters["mob-b"]["alive"] = true
	if _, got := a.pickFoe(b, "mob-b"); got != "mob-b" {
		t.Fatalf("focus target %s", got)
	}

	sky := a.profile(id)
	quests := objects(sky["quests"])
	if len(quests) == 0 || !strings.Contains(str(quests[0]["reward"]), "经验") {
		t.Fatalf("quest reward %+v", quests)
	}
	if str(sky["weatherNext"]) == "" || num(sky["weatherUntil"]) <= now() {
		t.Fatalf("sky %+v %+v", sky["weatherNext"], sky["weatherUntil"])
	}
}
