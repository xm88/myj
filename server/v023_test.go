// V0.2.3：私聊、驿站短信、一日三件事和称谓。金币、步速、伤害和门没有改。

package main

import "testing"

func TestDaybookAndTitleLeavePaceAlone(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	speed := num(a.live[id].Stats["speed"])
	atk := num(a.live[id].Stats["atk"])
	items := num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=?", id)["n"])
	if gold != a.constant("starterGold") || a.constant("marketTax") != 0.05 {
		t.Fatalf("economy constants gold=%v tax=%v", gold, a.constant("marketTax"))
	}
	expectFailure(t, func() { a.setTitle(id, "day_idle") })
	expectFailure(t, func() { a.setTitle(id, "grove_walker") })
	expectFailure(t, func() { a.setTitle(id, "sworn_kin") })
	expectFailure(t, func() { a.setTitle(id, "no_such") })
	expectFailure(t, func() { a.travel(id, "dungeon_ruins") })
	okRequest(t, a, "/api/title", token, M{"id": "newcomer"})
	if str(a.live[id].State["title"]) != "初来的旅人" || num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("title changed the pace")
	}
	a.postChat(id, "world", "今天风轻")
	a.live[id].State["x"], a.live[id].State["y"] = 480, 340
	a.dialog(id, "npc_elder")
	a.travel(id, "wild_greenleaf")
	if str(a.live[id].State["mapId"]) != "wild_greenleaf" {
		t.Fatal("travel moved wrong")
	}
	day := obj(a.profile(id)["day"])
	if !truth(day["speak"]) || !truth(day["stride"]) || !truth(day["hand"]) || !truth(day["done"]) || truth(day["kept"]) {
		t.Fatalf("day %v", day)
	}
	if len(objects(day["goals"])) != 3 {
		t.Fatal("goals")
	}
	expectFailure(t, func() { a.setTitle(id, "day_idle") })
	okRequest(t, a, "/api/day/claim", token, M{})
	expectFailure(t, func() { a.claimDay(id) })
	a.setTitle(id, "day_idle")
	if str(a.plateOf(a.live[id])) != "今日有闲" {
		t.Fatal("plate")
	}
	if num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp || num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != atk {
		t.Fatal("daily changed the player")
	}
	if num(a.one("SELECT COALESCE(SUM(count),0) AS n FROM player_items WHERE player_id=?", id)["n"]) != items {
		t.Fatal("daily changed the bag")
	}
	a.run("UPDATE players SET daybook=? WHERE id=?", js(M{"day": guildDay(now()) - 1, "speak": true, "stride": true, "hand": true, "kept": true}), id)
	a.refreshPlate(id)
	if str(a.live[id].State["title"]) != "" {
		t.Fatal("yesterday title stayed on the plate")
	}
	a.live[id].State["level"] = 10
	a.setTitle(id, "grove_walker")
	if str(a.live[id].State["title"]) != "林间行者" || num(a.live[id].Stats["speed"]) != speed {
		t.Fatal("level title changed speed")
	}
}

func TestWhisperNoteAndBond(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	token2, id2 := newGuest(t, a)
	_, id3 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.attach(id3, nil)
	name := str(a.player(id)["name"])
	name2 := str(a.player(id2)["name"])
	gold := num(a.player(id)["gold"])
	gold2 := num(a.player(id2)["gold"])
	speed := num(a.live[id].Stats["speed"])
	a.requestFriend(id, name2)
	row := a.one("SELECT id FROM friends WHERE from_id=? AND to_id=?", id, id2)
	a.removeFriend(id2, str(row["id"]))
	if a.one("SELECT id FROM friends WHERE from_id=? OR to_id=?", id, id) != nil {
		t.Fatal("declined request stayed")
	}
	expectFailure(t, func() { a.postChat(id, "friend:"+name2, "湖边见") })
	expectFailure(t, func() { a.sendNote(id, name2, "带一包干粮") })
	okRequest(t, a, "/api/friends/request", token, M{"name": name2})
	row = a.one("SELECT id FROM friends WHERE from_id=? AND to_id=? AND status='pending'", id, id2)
	okRequest(t, a, "/api/friends/accept", token2, M{"id": row["id"]})
	msg := a.postChat(id, "friend:"+name2, "湖边见")
	if str(msg["channel"]) != "dm:"+dmPair(id, id2) {
		t.Fatalf("channel %v", msg["channel"])
	}
	history := a.chatHistory(id2, "friend:"+name)
	if len(history) != 1 || str(history[0]["text"]) != "湖边见" || str(history[0]["channel"]) != str(msg["channel"]) {
		t.Fatalf("history %v", history)
	}
	if a.hears(id3, a.live[id3], str(msg["channel"])) || len(a.chatHistory(id3, "friend:"+name)) != 0 {
		t.Fatal("stranger heard the whisper")
	}
	if !a.hears(id, a.live[id], str(msg["channel"])) || !a.hears(id2, a.live[id2], str(msg["channel"])) {
		t.Fatal("friends missed the whisper")
	}
	okRequest(t, a, "/api/mail/send", token, M{"name": name2, "body": "带一包干粮"})
	letters := a.listMail(id2)
	var note M
	for _, letter := range letters {
		if str(letter["title"]) == "驿站短信" {
			note = letter
		}
	}
	if note == nil || str(note["from"]) != name || str(note["sender"]) != name || !truth(note["claimed"]) || truth(note["read"]) {
		t.Fatalf("note %v", note)
	}
	expectFailure(t, func() { a.claimMail(id2, str(note["id"])) })
	okRequest(t, a, "/api/mail/read", token2, M{"id": note["id"]})
	if num(a.profile(id2)["unreadMail"]) != 0 {
		t.Fatal("still unread")
	}
	a.mail(id, "登记", "驿站留了一句", nil)
	sys := a.listMail(id)[0]
	if str(sys["from"]) != "驿站" || str(sys["sender"]) != "" {
		t.Fatalf("system mail %v", sys)
	}
	if num(a.player(id)["gold"]) != gold || num(a.player(id2)["gold"]) != gold2 || num(a.live[id].Stats["speed"]) != speed {
		t.Fatal("social changed gold or speed")
	}
	a.pledgeFriend(id, name2)
	friends := a.friends(id2)
	if len(friends) != 1 || !truth(friends[0]["pledgeIn"]) || truth(friends[0]["bonded"]) {
		t.Fatalf("pledge %v", friends)
	}
	a.pledgeFriend(id2, name)
	if str(a.player(id)["bond_with"]) != id2 || str(a.player(id2)["bond_with"]) != id {
		t.Fatal("bond missing")
	}
	a.setTitle(id, "sworn_kin")
	if str(a.live[id].State["title"]) != "同行的人" || num(a.live[id].Stats["speed"]) != speed || num(a.live[id].Stats["atk"]) != num(a.stats(id, 1, "sword_shield", a.live[id].State["buffs"])["atk"]) {
		t.Fatal("bond title changed combat")
	}
	a.breakBond(id)
	if str(a.player(id)["bond_with"]) != "" || str(a.player(id)["title_id"]) != "" || str(a.live[id].State["title"]) != "" {
		t.Fatal("bond title stayed")
	}
	kept := a.one("SELECT id FROM friends WHERE status='accepted' AND ((from_id=? AND to_id=?) OR (from_id=? AND to_id=?))", id, id2, id2, id)
	a.removeFriend(id, str(kept["id"]))
	expectFailure(t, func() { a.postChat(id, "friend:"+name2, "还在吗") })
	if num(a.player(id)["gold"]) != gold || num(a.live[id].Stats["speed"]) != speed {
		t.Fatal("removing a friend changed gold or speed")
	}
}
