// 名帖、一日三件事、驿站短信和同行。称谓只挂在名字上。
// 不开金币、经验、伤害、掉落、税、门和步速。

package main

import (
	"strings"
	"unicode/utf8"
)

// titleCatalog 是旅团自己的显示称谓。等级、堂口、同行和今日小事都能解锁，都不进属性。
var titleCatalog = []M{
	{"id": "newcomer", "name": "初来的旅人", "need": "落脚即可", "kind": "level", "level": 1},
	{"id": "grove_walker", "name": "林间行者", "need": "到 10 级", "kind": "level", "level": 10},
	{"id": "lake_crosser", "name": "渡湖的人", "need": "到 20 级", "kind": "level", "level": 20},
	{"id": "gate_watcher", "name": "遗迹门前", "need": "到 25 级", "kind": "level", "level": 25},
	{"id": "hall_hand", "name": "堂口里的人", "need": "加入一个公会", "kind": "guild"},
	{"id": "sworn_kin", "name": "同行的人", "need": "和一位好友互相同行", "kind": "bond"},
	{"id": "day_idle", "name": "今日有闲", "need": "做完今天三件小事", "kind": "day"},
}

func titleByID(id string) M {
	for _, t := range titleCatalog {
		if str(t["id"]) == id {
			return t
		}
	}
	return nil
}

func titleLabel(id string) string {
	if t := titleByID(id); t != nil {
		return str(t["name"])
	}
	return ""
}

// bookRow 读今天的小事。过了日就当没做，不改库，等下一次记下再写。
func (a *App) bookRow(id string) M {
	r := a.one("SELECT daybook FROM players WHERE id=?", id)
	if r == nil {
		return nil
	}
	b := obj(parse(str(r["daybook"])))
	if num(b["day"]) != guildDay(now()) {
		return M{"day": guildDay(now())}
	}
	return b
}

func (a *App) writeBook(id string, b M) {
	b["day"] = guildDay(now())
	a.run("UPDATE players SET daybook=?,updated_at=? WHERE id=?", js(b), now(), id)
}

// noteDay 记下开口、换地或手头。已经记过的不再提示。不加金币和经验。
func (a *App) noteDay(id, key string) {
	if key != "speak" && key != "stride" && key != "hand" {
		return
	}
	b := a.bookRow(id)
	if b == nil || truth(b[key]) {
		return
	}
	b[key] = true
	a.writeBook(id, b)
	label := map[string]string{"speak": "开口", "stride": "换地", "hand": "手头"}[key]
	a.toast(id, "日常记下了："+label)
	if truth(b["speak"]) && truth(b["stride"]) && truth(b["hand"]) && !truth(b["kept"]) {
		a.toast(id, "三件都做了。名帖上可以收下「今日有闲」")
	}
	a.notify(id)
}

// claimDay 三件做完才收下今日称谓。不发金币、经验和物品。
func (a *App) claimDay(id string) {
	b := a.bookRow(id)
	check(b != nil, "还没有角色")
	check(truth(b["speak"]) && truth(b["stride"]) && truth(b["hand"]), "今天的三件还没做完")
	check(!truth(b["kept"]), "今天已经收下了")
	b["kept"] = true
	a.writeBook(id, b)
	a.refreshPlate(id)
	a.toast(id, "收下了。称谓「今日有闲」可以挂上，不加金币")
	a.notify(id)
}

func (a *App) playerLevel(id string) float64 {
	if l := a.live[id]; l != nil {
		return num(l.State["level"])
	}
	r := a.one("SELECT level FROM players WHERE id=?", id)
	if r == nil {
		return 0
	}
	return num(r["level"])
}

func (a *App) titleOpen(id, tid string) bool {
	t := titleByID(tid)
	if t == nil {
		return false
	}
	switch str(t["kind"]) {
	case "level":
		return a.playerLevel(id) >= num(t["level"])
	case "guild":
		return a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id) != nil
	case "bond":
		return str(a.player(id)["bond_with"]) != ""
	case "day":
		b := a.bookRow(id)
		return b != nil && truth(b["kept"])
	default:
		return false
	}
}

// shownTitle 是名牌上那一行。没解锁、过了日、散了同行，就空着。
func (a *App) shownTitle(id string) string {
	r := a.one("SELECT title_id FROM players WHERE id=?", id)
	if r == nil {
		return ""
	}
	tid := str(r["title_id"])
	if tid == "" || !a.titleOpen(id, tid) {
		return ""
	}
	return titleLabel(tid)
}

func (a *App) refreshPlate(id string) {
	l := a.live[id]
	if l == nil {
		return
	}
	l.State["title"] = a.shownTitle(id)
	l.State["plateDay"] = guildDay(now())
}

// plateOf 一天最多重算一次名牌，避免每个快照都读库。
func (a *App) plateOf(l *Live) string {
	if l == nil {
		return ""
	}
	if num(l.State["plateDay"]) != guildDay(now()) {
		a.refreshPlate(str(l.State["id"]))
	}
	return str(l.State["title"])
}

// setTitle 挂上或收起称谓。空字符串就是收起。不改属性。
func (a *App) setTitle(id, tid string) {
	tid = strings.TrimSpace(tid)
	if tid != "" {
		check(titleByID(tid) != nil, "没有这个称谓")
		check(a.titleOpen(id, tid), "这个称谓还没解锁")
	}
	a.run("UPDATE players SET title_id=?,updated_at=? WHERE id=?", tid, now(), id)
	a.refreshPlate(id)
	if tid == "" {
		a.toast(id, "收起了称谓")
	} else {
		a.toast(id, "挂上了"+titleLabel(tid)+"。属性没变")
	}
	a.notify(id)
}

func (a *App) dayView(id string) M {
	b := a.bookRow(id)
	if b == nil {
		b = M{}
	}
	speak, stride, hand := truth(b["speak"]), truth(b["stride"]), truth(b["hand"])
	goals := []M{
		{"id": "speak", "name": "开口", "hint": "在频道里说一句，私聊也算", "done": speak},
		{"id": "stride", "name": "换地", "hint": "走进另一张地图", "done": stride},
		{"id": "hand", "name": "手头", "hint": "喂一次灵兽、采集、做一件，或和镇民说上话", "done": hand},
	}
	return M{"speak": speak, "stride": stride, "hand": hand, "kept": truth(b["kept"]), "done": speak && stride && hand, "goals": goals}
}

func (a *App) titleList(id string) []M {
	worn := str(a.player(id)["title_id"])
	out := []M{}
	for _, t := range titleCatalog {
		tid := str(t["id"])
		open := a.titleOpen(id, tid)
		out = append(out, M{"id": tid, "name": t["name"], "need": t["need"], "open": open, "on": open && worn == tid})
	}
	return out
}

// fillNamecard 把称谓和今日小事放进档案。只给界面看。
func (a *App) fillNamecard(s M, id string) {
	s["titleName"] = a.shownTitle(id)
	s["title"] = ""
	if r := a.one("SELECT title_id,bond_with FROM players WHERE id=?", id); r != nil {
		s["title"] = str(r["title_id"])
		if bond := str(r["bond_with"]); bond != "" {
			if other := a.one("SELECT name FROM players WHERE id=?", bond); other != nil {
				s["bondName"] = str(other["name"])
			}
		}
	}
	s["titles"] = a.titleList(id)
	s["day"] = a.dayView(id)
}

func (a *App) acceptedFriend(id, other string) bool {
	if id == "" || other == "" || id == other {
		return false
	}
	return a.one("SELECT id FROM friends WHERE status='accepted' AND ((from_id=? AND to_id=?) OR (from_id=? AND to_id=?))", id, other, other, id) != nil
}

func dmPair(aID, bID string) string {
	if aID > bID {
		aID, bID = bID, aID
	}
	return aID + ":" + bID
}

// friendChannel 把「friend:名字」收成两个人共用的私聊频道。旁人听不见。
func (a *App) friendChannel(id, ch string, posting bool) string {
	if ch == "friend" {
		check(!posting, "先点一位好友")
		return ""
	}
	name := strings.TrimSpace(strings.TrimPrefix(ch, "friend:"))
	other := a.one("SELECT id FROM players WHERE name=?", name)
	if other == nil {
		check(!posting, "找不到这个旅人")
		return ""
	}
	oid := str(other["id"])
	if oid == id {
		check(!posting, "不能和自己私聊")
		return ""
	}
	if !a.acceptedFriend(id, oid) {
		check(!posting, "先成为好友才能私聊")
		return ""
	}
	return "dm:" + dmPair(id, oid)
}

// sendNote 给好友寄一封没有附件的短信。不扣金币，也不发东西。
func (a *App) sendNote(id, name, body string) {
	body = strings.Join(strings.Fields(body), " ")
	n := utf8.RuneCountInString(body)
	check(n >= 1 && n <= 60, "短信要在 1 到 60 个字")
	p := a.one("SELECT id,name FROM players WHERE name=?", strings.TrimSpace(name))
	check(p != nil, "找不到这个旅人")
	other := str(p["id"])
	check(other != id, "不能给自己写信")
	check(a.acceptedFriend(id, other), "先成为好友才能寄短信")
	if l := a.live[id]; l != nil {
		check(now()-num(l.State["noteAt"]) >= 2000, "寄得太快了")
		l.State["noteAt"] = now()
	}
	sender := str(a.player(id)["name"])
	a.run("INSERT INTO mail(id,player_id,title,body,attachments,is_read,claimed,created_at,sender) VALUES(?,?,?,?,NULL,0,1,?,?)", uid(), other, "驿站短信", body, now(), sender)
	a.toast(other, sender+" 寄来一封驿站短信")
	a.toast(id, "短信寄出了")
	a.notify(other)
	a.notify(id)
}

// readMail 把信标成已读。不发附件，领取仍走原来的领取。
func (a *App) readMail(id, mailID string) {
	r := a.one("SELECT id FROM mail WHERE id=? AND player_id=?", mailID, id)
	check(r != nil, "没有这封信")
	a.run("UPDATE mail SET is_read=1 WHERE id=? AND player_id=?", mailID, id)
	a.notify(id)
}

func (a *App) clearPledge(id, other string) {
	if str(a.player(id)["pledge_to"]) == other {
		a.run("UPDATE players SET pledge_to='',updated_at=? WHERE id=?", now(), id)
	}
}

// unbind 解开两个人的同行。挂着「同行的人」就一起卸下。
func (a *App) unbind(id, other string) {
	for _, pid := range []string{id, other} {
		row := a.player(pid)
		partner := other
		if pid == other {
			partner = id
		}
		if str(row["bond_with"]) != partner {
			continue
		}
		if str(row["title_id"]) == "sworn_kin" {
			a.run("UPDATE players SET bond_with='',title_id='',updated_at=? WHERE id=?", now(), pid)
		} else {
			a.run("UPDATE players SET bond_with='',updated_at=? WHERE id=?", now(), pid)
		}
		a.refreshPlate(pid)
	}
}

// removeFriend 撤回申请、回绝或删除好友。同行会一起散掉。
func (a *App) removeFriend(id, request string) {
	r := a.one("SELECT * FROM friends WHERE id=?", request)
	check(r != nil, "没有这条关系")
	from, to := str(r["from_id"]), str(r["to_id"])
	check(from == id || to == id, "没有这条关系")
	if str(r["status"]) == "pending" && from != id && to != id {
		fail("没有这条关系")
	}
	other := from
	if other == id {
		other = to
	}
	a.run("DELETE FROM friends WHERE id=?", request)
	a.clearPledge(id, other)
	a.clearPledge(other, id)
	a.unbind(id, other)
	a.toast(id, "关系放下了")
	a.notify(id)
	a.notify(other)
}

// pledgeFriend 邀一位好友同行。对方也邀过，就两边都记下。不加属性。
func (a *App) pledgeFriend(id, name string) {
	p := a.one("SELECT id FROM players WHERE name=?", strings.TrimSpace(name))
	check(p != nil, "找不到这个旅人")
	other := str(p["id"])
	check(other != id, "不能和自己同行")
	check(a.acceptedFriend(id, other), "先成为好友")
	me := a.player(id)
	them := a.player(other)
	if str(me["bond_with"]) == other {
		fail("已经是同行的人了")
	}
	check(str(me["bond_with"]) == "", "已经有同行的人了")
	check(str(them["bond_with"]) == "", "对方已经有同行的人了")
	mine := str(me["name"])
	if str(them["pledge_to"]) == id {
		a.run("UPDATE players SET bond_with=?,pledge_to='',updated_at=? WHERE id=?", other, now(), id)
		a.run("UPDATE players SET bond_with=?,pledge_to='',updated_at=? WHERE id=?", id, now(), other)
		a.refreshPlate(id)
		a.refreshPlate(other)
		a.toast(id, "你们同行了。称谓「同行的人」可以挂上")
		a.toast(other, mine+" 和你同行了")
		a.notify(id)
		a.notify(other)
		return
	}
	check(str(me["pledge_to"]) != other, "已经邀过了")
	a.run("UPDATE players SET pledge_to=?,updated_at=? WHERE id=?", other, now(), id)
	a.toast(other, mine+" 想和你同行")
	a.toast(id, "邀了同行，等对方答应")
	a.notify(id)
	a.notify(other)
}

// breakBond 自己这一侧散掉同行。
func (a *App) breakBond(id string) {
	other := str(a.player(id)["bond_with"])
	check(other != "", "还没有同行的人")
	a.unbind(id, other)
	a.toast(id, "同行散了")
	if name := str(a.player(id)["name"]); name != "" {
		a.toast(other, name+" 散了同行")
	}
	a.notify(id)
	a.notify(other)
}
