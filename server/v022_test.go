// V0.2.2：公会点卯和公告、坐骑预览。捐献升级、步速和遗迹门槛没有改。

package main

import "testing"

func TestHallRollNoticeAndDonateLadder(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	_, id2 := newGuest(t, a)
	a.addGold(id, 1000)
	a.addGold(id2, 200)
	okRequest(t, a, "/api/guilds", token, M{"name": "青叶堂"})
	gid := str(a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id)["guild_id"])
	a.joinGuild(id2, gid)
	expectFailure(t, func() { a.setGuildNotice(id2, "成员改的") })
	okRequest(t, a, "/api/guilds/notice", token, M{"text": "今晚点卯，遗迹门口见。"})
	view := a.profile(id)
	guild := obj(view["guild"])
	if str(guild["notice"]) != "今晚点卯，遗迹门口见。" || str(guild["role"]) != "leader" {
		t.Fatalf("notice %v", guild)
	}
	gold := num(a.player(id)["gold"])
	exp := num(a.player(id)["exp"])
	okRequest(t, a, "/api/guilds/roll", token, M{})
	if num(a.player(id)["gold"]) != gold || num(a.player(id)["exp"]) != exp {
		t.Fatal("roll changed the player")
	}
	expectFailure(t, func() { a.rollGuild(id) })
	a.donateGuild(id, 50)
	a.donateGuild(id, 450)
	after := obj(a.profile(id)["guild"])
	if num(after["level"]) != 2 || num(after["exp"]) != 520 || num(after["contrib"]) != 500 {
		t.Fatalf("ladder %v", after)
	}
	var memberContrib float64
	for _, m := range objects(after["members"]) {
		if str(m["id"]) == id2 {
			memberContrib = num(m["contrib"])
		}
	}
	if memberContrib != 0 || len(objects(after["members"])) != 2 {
		t.Fatalf("roster %v", after["members"])
	}
	msg := a.postChat(id, "guild", "点卯了")
	if str(msg["channel"]) != "guild:"+gid {
		t.Fatal(msg)
	}
	history := a.chatHistory(id2, "guild")
	if len(history) != 1 || str(history[0]["text"]) != "点卯了" {
		t.Fatalf("guild chat %v", history)
	}
}

func TestMountPreviewDoesNotChangePaceOrGate(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	speed := num(a.live[id].Stats["speed"])
	expectFailure(t, func() { a.setMountPreview(id, "mist_deer") })
	expectFailure(t, func() { a.setMountPreview(id, "no_such") })
	a.setMountPreview(id, "leaf_donkey")
	if str(a.player(id)["mount_preview"]) != "leaf_donkey" || str(a.live[id].State["mount"]) != "leaf_donkey" {
		t.Fatal("preview not stored")
	}
	if num(a.live[id].Stats["speed"]) != speed {
		t.Fatal("preview changed speed")
	}
	expectFailure(t, func() { a.travel(id, "dungeon_ruins") })
	if str(a.live[id].State["mapId"]) != "town_starfall" {
		t.Fatal("gate moved")
	}
	a.live[id].State["level"] = 28
	a.setMountPreview(id, "mist_deer")
	if num(a.live[id].Stats["speed"]) != speed {
		t.Fatal("high preview changed speed")
	}
	a.setMountPreview(id, "")
	if str(a.player(id)["mount_preview"]) != "" || str(a.live[id].State["mount"]) != "" {
		t.Fatal("preview stayed up")
	}
}
