// 堂口公告、点卯和坐骑预览。捐献换级、聊天频道和副本进图仍走原来的函数。

package main

import (
	"math"
	"strings"
	"unicode/utf8"
)

// guildLevel 和捐献用同一条：500 经验一级，上限 10。点卯只往经验上加，不另写一条。
func guildLevel(exp float64) float64 {
	return math.Min(10, 1+math.Floor(exp/500))
}

// guildDay 按服务器毫秒取日。同一天只能点一次卯。
func guildDay(t float64) float64 {
	return math.Floor(t / 86400000)
}

// setGuildNotice 只让会长改公告。捐献、等级和人数不动。
func (a *App) setGuildNotice(id, text string) {
	r := a.one("SELECT guild_id, role FROM guild_members WHERE player_id=?", id)
	check(r != nil, "你不在公会里")
	check(str(r["role"]) == "leader", "只有会长能改公告")
	clean := strings.Join(strings.Fields(text), " ")
	n := utf8.RuneCountInString(clean)
	check(n >= 1 && n <= 40, "公告要在 1 到 40 个字")
	a.run("UPDATE guilds SET notice=? WHERE id=?", clean, r["guild_id"])
	a.toast(id, "公告改好了")
	a.notify(id)
}

// rollGuild 一天一次，给公会 20 经验。不加个人经验、金币，也不改战斗。
func (a *App) rollGuild(id string) {
	r := a.one("SELECT guild_id, roll_day FROM guild_members WHERE player_id=?", id)
	check(r != nil, "你不在公会里")
	day := guildDay(now())
	check(num(r["roll_day"]) != day, "今天已经点过卯")
	check(a.run("UPDATE guild_members SET roll_day=? WHERE player_id=? AND roll_day<>?", day, id, day) == 1, "今天已经点过卯")
	g := a.one("SELECT exp FROM guilds WHERE id=?", r["guild_id"])
	exp := num(g["exp"]) + 20
	a.run("UPDATE guilds SET exp=?,level=? WHERE id=?", exp, guildLevel(exp), r["guild_id"])
	a.toast(id, "点过卯了。公会经验 +20")
	a.notify(id)
}

// mountCatalog 是旅团自己的四匹预览。等级只决定能不能亮出来，不加快走路。
var mountCatalog = []M{
	{"id": "leaf_donkey", "name": "青叶驴", "level": 1, "blurb": "镇上短途的灰驴，鞍上缝了一片青叶。亮出来也还是原来的步子。"},
	{"id": "red_pony", "name": "赤岩驹", "level": 12, "blurb": "矿坑边上的小红马，鬃毛像晒过的岩土。"},
	{"id": "wind_yak", "name": "风铃牦", "level": 20, "blurb": "高地牦牛，颈上挂一枚风铃。铃不响，也不加快。"},
	{"id": "mist_deer", "name": "雾绒鹿", "level": 28, "blurb": "湖雾里的细腿鹿，角尖一点暖金。只是预览。"},
}

func mountByID(id string) M {
	for _, m := range mountCatalog {
		if str(m["id"]) == id {
			return m
		}
	}
	return nil
}

// setMountPreview 记下要亮出的坐骑。空着就是收起。速度、碰撞和战斗不读这个字段。
func (a *App) setMountPreview(id, mountID string) {
	mountID = strings.TrimSpace(mountID)
	var name string
	if mountID != "" {
		m := mountByID(mountID)
		check(m != nil, "没有这匹坐骑")
		level := num(a.player(id)["level"])
		if l := a.live[id]; l != nil {
			level = num(l.State["level"])
		}
		check(level >= num(m["level"]), "等级不够，还不能预览这匹")
		name = str(m["name"])
	}
	a.run("UPDATE players SET mount_preview=?,updated_at=? WHERE id=?", mountID, now(), id)
	if l := a.live[id]; l != nil {
		l.State["mount"] = mountID
	}
	if mountID == "" {
		a.toast(id, "收起了坐骑预览")
	} else {
		a.toast(id, "亮出了"+name+"。步子还是原来的")
	}
	a.notify(id)
}
