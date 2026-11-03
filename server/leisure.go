// 引路、抛竿、衣橱和招呼。都是街上的轻量事。
// 不开金币、经验、伤害、掉落、税、门和步速。采集熟练也不涨。

package main

import (
	"strings"
)

// trailBondMax 路谊到这里就停。再传一句也只是说话，不加属性。
const trailBondMax = 6

// trailTitleAt 够这点路谊才能挂「认得路的人」。
const trailTitleAt = 3

// castWait 抛一次竿之后要歇这么久。不跟地图上的鱼点共用冷却。
const castWait = 90000.0

// emoteMs 招呼在别人眼里留一会儿。emoteGap 是下一次招呼的间隔。
const emoteMs = 2600.0
const emoteGap = 1200.0

// trailGap 邀请和收回要隔一会儿，免得来回刷对方的提示。
const trailGap = 1200.0

// lookCap 衣橱最多留三套。换上只改衣色、绦带和坐骑预览。
const lookCap = 3

// sashCatalog 是胸前一条绦。素带就是原来的衣服，空着不写库。
var sashCatalog = []M{
	{"id": "plain", "name": "素带", "color": "", "blurb": "原来的衣襟，不另加绦。"},
	{"id": "reed", "name": "苇穗", "color": "#e6c56a", "blurb": "胸前一道苇黄色。不加防御。"},
	{"id": "tide", "name": "潮线", "color": "#7eb8c9", "blurb": "湖边那种浅青绦。"},
	{"id": "ember", "name": "余烬", "color": "#c4513a", "blurb": "细细一条暖红。步子不变。"},
	{"id": "mist", "name": "雾绦", "color": "#c9c4e4", "blurb": "夜里看不清的浅紫。"},
}

// emoteCatalog 三个招呼。旁边的人看见字，动效由网页决定。
var emoteCatalog = []M{
	{"id": "wave", "name": "招手", "text": "招了招手"},
	{"id": "bow", "name": "作揖", "text": "拱手作了个揖"},
	{"id": "cheer", "name": "喝彩", "text": "抬手喝了声彩"},
}

// trailLines 引路人和随行人各三句。不能自己编，免得和聊天混在一起。
var trailLines = []M{
	{"id": "kit", "role": "lead", "text": "先把伤药放在顺手的格子。"},
	{"id": "gate", "role": "lead", "text": "门在地图边上，走近就能过去。"},
	{"id": "feed", "role": "lead", "text": "灵兽饿了，喂一口就肯跟着。"},
	{"id": "heard", "role": "walk", "text": "记下了，我先去试试。"},
	{"id": "ask", "role": "walk", "text": "下一张图，门在哪一边？"},
	{"id": "thanks", "role": "walk", "text": "今天跟着走了一段。"},
}

func sashByID(id string) M {
	for _, row := range sashCatalog {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func emoteByID(id string) M {
	for _, row := range emoteCatalog {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func trailLine(role, id string) M {
	for _, row := range trailLines {
		if str(row["role"]) == role && str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func trailRoleName(role string) string {
	if role == "lead" {
		return "引路人"
	}
	if role == "walk" {
		return "随行人"
	}
	return ""
}

func trailMark(role string) string {
	if role == "lead" {
		return "引"
	}
	if role == "walk" {
		return "随"
	}
	return ""
}

// trailRow 读路谊。过了日的「今天说过」只在这份拷贝里清掉，写回去时才落库。
func (a *App) trailRow(id string) M {
	r := a.player(id)
	b := obj(parse(str(r["trail"])))
	if num(b["day"]) != guildDay(now()) {
		b["said"] = false
	}
	return b
}

func (a *App) writeTrail(id string, row M) {
	a.run("UPDATE players SET trail=?,updated_at=? WHERE id=?", js(row), now(), id)
	a.syncTrailMark(id)
}

func (a *App) syncTrailMark(id string) {
	l := a.live[id]
	if l == nil {
		return
	}
	row := obj(parse(str(a.player(id)["trail"])))
	mark := ""
	if str(row["peer"]) != "" {
		mark = trailMark(str(row["role"]))
	}
	l.State["trail"] = mark
}

func (a *App) trailPeerName(id string) string {
	if id == "" {
		return ""
	}
	if p := a.one("SELECT name FROM players WHERE id=?", id); p != nil {
		return str(p["name"])
	}
	return ""
}

// askTrail 邀请结一段路谊。role 是自己想当的：lead 引路，walk 跟着学。
func (a *App) askTrail(id, name, role string) {
	role = strings.TrimSpace(role)
	check(role == "lead" || role == "walk", "只能请人引路，或请人跟着")
	other := a.one("SELECT id FROM players WHERE name=?", strings.TrimSpace(name))
	check(other != nil, "找不到这个旅人")
	oid := str(other["id"])
	check(oid != id, "不能和自己结路谊")
	me := a.trailRow(id)
	them := a.trailRow(oid)
	check(str(me["peer"]) == "", "你已经有路谊了")
	check(str(them["peer"]) == "", "对方已经有路谊了")
	check(str(me["out"]) == "" && str(me["in"]) == "", "你还有没处理的邀请")
	check(str(them["out"]) == "" && str(them["in"]) == "", "对方还有没处理的邀请")
	mine, theirs := a.playerLevel(id), a.playerLevel(oid)
	if role == "lead" {
		check(mine > theirs, "引路的人得先走一步")
	} else {
		check(theirs > mine, "引路的人得先走一步")
	}
	check(now() >= num(me["askAt"]), "路谊的邀请太密了")
	theirRole := "walk"
	if role == "walk" {
		theirRole = "lead"
	}
	me["out"], me["outAs"] = oid, role
	me["askAt"] = now() + trailGap
	them["in"], them["inAs"] = id, theirRole
	a.writeTrail(id, me)
	a.writeTrail(oid, them)
	a.toast(oid, str(a.player(id)["name"])+" 想和你结一段路谊")
	a.toast(id, "路谊的邀请送出去了")
	a.notify(id)
	a.notify(oid)
}

func (a *App) acceptTrail(id string) {
	me := a.trailRow(id)
	from := str(me["in"])
	check(from != "", "没有路谊邀请")
	them := a.trailRow(from)
	check(str(them["out"]) == id, "邀请已经不在了")
	check(str(me["peer"]) == "" && str(them["peer"]) == "", "已经有路谊了")
	myRole, theirRole := str(me["inAs"]), str(them["outAs"])
	check((myRole == "lead" && theirRole == "walk") || (myRole == "walk" && theirRole == "lead"), "邀请对不上")
	if myRole == "lead" {
		check(a.playerLevel(id) > a.playerLevel(from), "引路的人得先走一步")
	} else {
		check(a.playerLevel(from) > a.playerLevel(id), "引路的人得先走一步")
	}
	a.writeTrail(id, M{"peer": from, "role": myRole, "bond": 0})
	a.writeTrail(from, M{"peer": id, "role": theirRole, "bond": 0})
	a.toast(id, "路谊结下了。一天可以互传一句，不加属性")
	a.toast(from, str(a.player(id)["name"])+" 答应了结路谊")
	a.notify(id)
	a.notify(from)
}

func (a *App) declineTrail(id string) {
	me := a.trailRow(id)
	from := str(me["in"])
	check(from != "", "没有要回的邀请")
	them := a.trailRow(from)
	me["in"], me["inAs"] = "", ""
	if str(them["out"]) == id {
		them["out"], them["outAs"] = "", ""
	}
	a.writeTrail(id, me)
	a.writeTrail(from, them)
	a.toast(from, str(a.player(id)["name"])+" 没有结这段路谊")
	a.toast(id, "回绝了路谊")
	a.notify(id)
	a.notify(from)
}

func (a *App) cancelTrail(id string) {
	me := a.trailRow(id)
	oid := str(me["out"])
	check(oid != "", "没有要收回的邀请")
	check(now() >= num(me["askAt"]), "路谊的邀请太密了")
	them := a.trailRow(oid)
	me["askAt"] = now() + trailGap
	me["out"], me["outAs"] = "", ""
	if str(them["in"]) == id {
		them["in"], them["inAs"] = "", ""
	}
	a.writeTrail(id, me)
	a.writeTrail(oid, them)
	a.toast(oid, str(a.player(id)["name"])+" 收回了路谊邀请")
	a.toast(id, "收回了邀请")
	a.notify(id)
	a.notify(oid)
}

func (a *App) leaveTrail(id string) {
	me := a.trailRow(id)
	peer := str(me["peer"])
	check(peer != "", "还没有路谊")
	them := a.trailRow(peer)
	a.writeTrail(id, M{})
	if str(them["peer"]) == id {
		a.writeTrail(peer, M{})
		a.refreshPlate(peer)
		a.toast(peer, str(a.player(id)["name"])+" 散了路谊")
		a.notify(peer)
	}
	a.refreshPlate(id)
	a.toast(id, "路谊散了。称谓如果挂着，也会卸下显示")
	a.notify(id)
}

// sayTrail 一天一句。两个人都传过，路谊 +1，最高 6。不加金币和经验。
func (a *App) sayTrail(id, lineID string) {
	me := a.trailRow(id)
	peer := str(me["peer"])
	check(peer != "", "还没有路谊")
	role := str(me["role"])
	line := trailLine(role, strings.TrimSpace(lineID))
	check(line != nil, "这句不是你能传的")
	check(!truth(me["said"]), "今天已经传过一句")
	them := a.trailRow(peer)
	check(str(them["peer"]) == id, "路谊已经散了")
	day := guildDay(now())
	me["said"] = true
	me["day"] = day
	// line 留下今天这句的 id。对方不在线时，打开引路面板仍能读到原文。
	me["line"] = str(line["id"])
	if truth(them["said"]) {
		next := num(me["bond"]) + 1
		if next > trailBondMax {
			next = trailBondMax
		}
		me["bond"] = next
		them["bond"] = next
		a.writeTrail(peer, them)
		a.writeTrail(id, me)
		a.refreshPlate(id)
		a.refreshPlate(peer)
		if next >= trailTitleAt && next-1 < trailTitleAt {
			a.toast(id, "路谊够了。可以挂「认得路的人」，不加属性")
			a.toast(peer, "路谊够了。可以挂「认得路的人」，不加属性")
		} else if next >= trailBondMax && num(them["bond"]) >= trailBondMax {
			a.toast(id, "路谊已经满了。这句话还是送到了")
			a.toast(peer, "路谊已经满了。今天这句也齐了")
		} else {
			a.toast(id, "两句都到了，路谊往前走了一步")
			a.toast(peer, "两句都到了，路谊往前走了一步")
		}
		a.notify(peer)
	} else {
		a.writeTrail(id, me)
		a.toast(id, "送到了。等对方也传一句，路谊才会往前")
		a.notify(peer)
	}
	a.toast(peer, str(a.player(id)["name"])+" 传了一句："+str(line["text"]))
	a.notify(id)
}

func (a *App) trailView(id string) M {
	me := a.trailRow(id)
	role := str(me["role"])
	peer := str(me["peer"])
	lines := []M{}
	if peer != "" {
		for _, row := range trailLines {
			if str(row["role"]) == role {
				lines = append(lines, M{"id": row["id"], "text": row["text"]})
			}
		}
	}
	var incoming, outgoing any
	if from := str(me["in"]); from != "" && peer == "" {
		incoming = M{"name": a.trailPeerName(from), "role": str(me["inAs"]), "roleName": trailRoleName(str(me["inAs"]))}
	}
	if to := str(me["out"]); to != "" && peer == "" {
		outgoing = M{"name": a.trailPeerName(to), "role": str(me["outAs"]), "roleName": trailRoleName(str(me["outAs"]))}
	}
	themSaid := false
	peerLine := ""
	if peer != "" {
		them := a.trailRow(peer)
		if str(them["peer"]) == id && truth(them["said"]) {
			themSaid = true
			if row := trailLine(str(them["role"]), str(them["line"])); row != nil {
				peerLine = str(row["text"])
			}
		}
	}
	bond := 0.
	if peer != "" {
		bond = num(me["bond"])
	}
	return M{
		"role": role, "roleName": trailRoleName(role), "peerName": a.trailPeerName(peer),
		"bond": bond, "bondMax": trailBondMax, "titleAt": trailTitleAt,
		"said": peer != "" && truth(me["said"]), "peerSaid": themSaid, "peerLine": peerLine,
		"incoming": incoming, "outgoing": outgoing, "lines": lines,
	}
}

// castLine 在星落镇或月光湖抛一次竿。背包能放下就进背包，满了就寄到邮箱。不涨采集熟练，也不动鱼点。
func (a *App) castLine(id string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能抛竿")
	check(str(l.State["boutId"]) == "", "战斗里不能抛竿")
	mapID := str(l.State["mapId"])
	check(mapID == "town_starfall" || mapID == "wild_moonlake", "得在星落镇或月光湖边上")
	t := now()
	until := num(l.State["castUntil"])
	if _, ok := l.State["castUntil"]; !ok {
		until = num(a.player(id)["cast_at"])
	}
	check(t >= until, "竿还没歇过来")
	stacked := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id=? AND location='bag' AND extra IS NULL", id, "fish_silver") != nil
	mailed := !stacked && a.slots(id, "bag") >= a.constant("bagCap")
	a.addItem(id, "fish_silver", 1, nil, "bag")
	next := t + castWait
	a.run("UPDATE players SET cast_at=?,updated_at=? WHERE id=?", next, now(), id)
	l.State["castUntil"] = next
	if mailed {
		a.toast(id, "背包满了，银鳞鱼寄到邮箱了。采集熟练没涨")
	} else {
		a.toast(id, "抛竿钓上银鳞鱼 ×1。采集熟练没涨")
	}
	a.notify(id)
}

func (a *App) setSash(id, sashID string) {
	sashID = strings.TrimSpace(sashID)
	label := "素带"
	if sashID == "plain" {
		sashID = ""
	}
	if sashID != "" {
		row := sashByID(sashID)
		check(row != nil, "没有这条绦")
		label = str(row["name"])
	}
	a.run("UPDATE players SET sash_id=?,updated_at=? WHERE id=?", sashID, now(), id)
	if l := a.live[id]; l != nil {
		l.State["sash"] = sashID
	}
	if sashID == "" {
		a.toast(id, "收成了素带。属性和步子没变")
	} else {
		a.toast(id, "系上了"+label+"。属性和步子没变")
	}
	a.notify(id)
}

func (a *App) looksOf(id string) []M {
	out := []M{}
	for _, row := range objects(parse(str(a.player(id)["looks"]))) {
		if str(row["id"]) == "" {
			continue
		}
		out = append(out, M{
			"id": row["id"], "name": row["name"],
			"dye": str(row["dye"]), "mount": str(row["mount"]), "sash": str(row["sash"]),
		})
	}
	return out
}

func (a *App) saveLooks(id string, rows []M) {
	if rows == nil {
		rows = []M{}
	}
	a.run("UPDATE players SET looks=?,updated_at=? WHERE id=?", js(rows), now(), id)
}

func (a *App) currentLook(id string) (string, string, string) {
	r := a.player(id)
	dye, mount, sash := str(r["dye_id"]), str(r["mount_preview"]), str(r["sash_id"])
	if l := a.live[id]; l != nil {
		if l.State["dye"] != nil {
			dye = str(l.State["dye"])
		}
		if l.State["mount"] != nil {
			mount = str(l.State["mount"])
		}
		if l.State["sash"] != nil {
			sash = str(l.State["sash"])
		}
	}
	return dye, mount, sash
}

func (a *App) saveLook(id, name string) {
	name = strings.TrimSpace(name)
	check(validName(name, 1, 6), "样子要有 1 到 6 个字，可用中文、字母和数字")
	rows := a.looksOf(id)
	check(len(rows) < lookCap, "衣橱最多留三套")
	dye, mount, sash := a.currentLook(id)
	rows = append(rows, M{"id": uid(), "name": name, "dye": dye, "mount": mount, "sash": sash})
	a.saveLooks(id, rows)
	a.toast(id, "记下了「"+name+"」。只是这一身的样子")
	a.notify(id)
}

func lookLabel(dye, mount, sash string) string {
	dyeName := "暖茶"
	if dye != "" {
		if row := dyeByID(dye); row != nil {
			dyeName = str(row["name"])
		}
	}
	mountName := "未亮坐骑"
	if mount != "" {
		if row := mountByID(mount); row != nil {
			mountName = str(row["name"])
		}
	}
	sashName := "素带"
	if sash != "" {
		if row := sashByID(sash); row != nil {
			sashName = str(row["name"])
		}
	}
	return dyeName + " · " + sashName + " · " + mountName
}

// wearLook 换上保存的衣色、绦带和坐骑预览。坐骑等级不够就留着原来的预览。
func (a *App) wearLook(id, lookID string) {
	var hit M
	for _, row := range a.looksOf(id) {
		if str(row["id"]) == lookID {
			hit = row
			break
		}
	}
	check(hit != nil, "衣橱里没有这一套")
	dye, mount, sash := str(hit["dye"]), str(hit["mount"]), str(hit["sash"])
	if dye == "tea" {
		dye = ""
	}
	if dye != "" {
		check(dyeByID(dye) != nil, "这套衣色已经不在了")
	}
	if sash == "plain" {
		sash = ""
	}
	if sash != "" {
		check(sashByID(sash) != nil, "这条绦已经不在了")
	}
	note := ""
	if mount != "" {
		m := mountByID(mount)
		check(m != nil, "这匹坐骑已经不在了")
		if a.playerLevel(id) < num(m["level"]) {
			_, mount, _ = a.currentLook(id)
			note = "坐骑等级不够，衣色和绦带先换上"
		}
	}
	a.run("UPDATE players SET dye_id=?,mount_preview=?,sash_id=?,updated_at=? WHERE id=?", dye, mount, sash, now(), id)
	if l := a.live[id]; l != nil {
		l.State["dye"] = dye
		l.State["mount"] = mount
		l.State["sash"] = sash
	}
	if note == "" {
		a.toast(id, "换上了「"+str(hit["name"])+"」。属性和步子没变")
	} else {
		a.toast(id, note)
	}
	a.notify(id)
}

func (a *App) dropLook(id, lookID string) {
	rows := a.looksOf(id)
	next := []M{}
	found := false
	for _, row := range rows {
		if str(row["id"]) == lookID {
			found = true
			continue
		}
		next = append(next, row)
	}
	check(found, "衣橱里没有这一套")
	a.saveLooks(id, next)
	a.toast(id, "从衣橱里拿掉了")
	a.notify(id)
}

func (a *App) lookViews(id string) []M {
	out := []M{}
	for _, row := range a.looksOf(id) {
		out = append(out, M{
			"id": row["id"], "name": row["name"],
			"dye": row["dye"], "mount": row["mount"], "sash": row["sash"],
			"label": lookLabel(str(row["dye"]), str(row["mount"]), str(row["sash"])),
		})
	}
	return out
}

// playEmote 在当前地图上打个招呼。不改坐标，也不进战斗结算。
func (a *App) playEmote(id, emoteID string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能招呼")
	check(str(l.State["boutId"]) == "", "战斗里不能招呼")
	row := emoteByID(strings.TrimSpace(emoteID))
	check(row != nil, "没有这个招呼")
	t := now()
	check(t >= num(l.State["emoteAt"]), "招呼太密了")
	l.State["emote"] = str(row["id"])
	l.State["emoteUntil"] = t + emoteMs
	l.State["emoteAt"] = t + emoteGap
	a.toast(id, str(row["text"]))
}

func shownEmote(s M) string {
	if s == nil {
		return ""
	}
	if num(s["emoteUntil"]) > now() {
		return str(s["emote"])
	}
	return ""
}

// fillLeisure 把路谊、抛竿和衣橱放进档案。只给界面看。
func (a *App) fillLeisure(s M, id string) {
	r := a.player(id)
	sashID := str(r["sash_id"])
	until := num(r["cast_at"])
	if l := a.live[id]; l != nil {
		if l.State["sash"] != nil {
			sashID = str(l.State["sash"])
		}
		if l.State["castUntil"] != nil {
			until = num(l.State["castUntil"])
		}
	}
	s["sash"] = sashID
	s["sashes"] = sashCatalog
	s["looks"] = a.lookViews(id)
	s["castUntil"] = until
	s["castWait"] = castWait
	s["castReady"] = now() >= until
	s["trail"] = a.trailView(id)
	s["emotes"] = emoteCatalog
}

func (a *App) trailBond(id string) float64 {
	row := a.trailRow(id)
	if str(row["peer"]) == "" {
		return 0
	}
	return num(row["bond"])
}
