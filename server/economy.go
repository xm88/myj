// 城镇业务：制造、商店、拍卖、公会、邮件、好友、队伍与聊天。经济写入由调用方事务保护。

package main

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// craft 按配方扣金币与材料并发放产物，随后推进生活熟练和制造任务。
func (a *App) craft(id, recipe string) {
	r := a.def("recipes", recipe)
	check(r != nil, "没有这个配方")
	life := str(r["life"])
	check(num(a.track(id, "life:"+life)["level"]) >= num(r["skill"]), "生活技能等级不够")
	output := obj(r["output"])
	tpl := a.def("items", str(output["itemId"]))
	check(tpl != nil, "产物不存在")
	a.addGold(id, -num(r["gold"]))
	for _, i := range objects(r["inputs"]) {
		a.consumeTemplate(id, str(i["itemId"]), num(i["count"]))
	}
	var ex M
	if str(tpl["kind"]) == "equipment" {
		ex = a.equipmentExtra(fallback(output["quality"], "green"))
	}
	a.addItem(id, str(output["itemId"]), num(output["count"]), ex, "bag")
	gain := 12 * num(r["skill"])
	a.trackExp(id, "life:"+life, gain, false)
	a.emit(id, M{"type": "craft", "recipeId": recipe})
	lifeLabel := "制造"
	if life == "cook" {
		lifeLabel = "料理"
	}
	made := str(tpl["name"])
	if str(tpl["kind"]) == "equipment" {
		a.toast(id, fmt.Sprintf("做完了：%s（%s）。%s熟练 +%g", made, qualityName(str(fallback(output["quality"], "white"))), lifeLabel, gain))
	} else {
		a.toast(id, fmt.Sprintf("做完了：%s ×%g。%s熟练 +%g", made, num(output["count"]), lifeLabel, gain))
	}
	a.touchAlbum(id, nil)
	a.notify(id)
}

// buyShop 校验商店货物与数量，扣除金币后发放对应物品。
func (a *App) buyShop(id, npc, item string, count float64) {
	check(count >= 1 && count <= 99, "数量不对")
	var good M
	for _, g := range objects(a.def("npcs", npc)["shop"]) {
		if str(g["itemId"]) == item {
			good = g
		}
	}
	check(good != nil, "这里不卖这个")
	a.addGold(id, -num(good["price"])*count)
	a.addItem(id, item, count, nil, "bag")
	a.emit(id, M{"type": "buy_shop"})
	a.notify(id)
}

// sellShop 校验物品归属、位置和可售标记，移除出售数量并发放金币。
func (a *App) sellShop(id, item string, count float64) {
	r := a.takeItem(id, item)
	check(str(r["location"]) == "bag", "只能卖背包里的东西")
	tpl := a.def("items", str(r["template_id"]))
	check(tpl != nil && !truth(tpl["bound"]) && num(tpl["sell"]) > 0, "这件不能卖给商人")
	n := math.Min(count, num(r["count"]))
	if str(tpl["kind"]) == "equipment" {
		n = 1
	}
	check(n > 0, "数量不对")
	a.consume(r, n)
	a.addGold(id, num(tpl["sell"])*n)
	a.notify(id)
}

// market 读取开放订单，合并卖家名称与物品模板后输出拍卖列表。
func (a *App) market() []M {
	out := []M{}
	for _, r := range a.rows("SELECT o.*,p.name AS seller_name FROM market_orders o LEFT JOIN players p ON p.id=o.seller_id WHERE o.status='open' ORDER BY o.created_at DESC LIMIT 80") {
		s := obj(parse(str(r["snapshot"])))
		tpl := a.def("items", str(s["templateId"]))
		out = append(out, M{"id": r["id"], "sellerId": r["seller_id"], "sellerName": fallback(r["seller_name"], "旅人"), "price": r["price"], "templateId": s["templateId"], "name": tpl["name"], "count": s["count"], "extra": s["extra"], "kind": tpl["kind"], "createdAt": r["created_at"]})
	}
	return out
}

// listOrder 将可交易的背包物品转为订单快照，上架数量从原背包扣除。
func (a *App) listOrder(id, item string, count, price float64) {
	check(price >= 1 && price <= 1000000 && price == math.Floor(price), "标价不合法")
	r := a.takeItem(id, item)
	check(str(r["location"]) == "bag", "只能上架背包里的东西")
	tpl := a.def("items", str(r["template_id"]))
	check(tpl != nil && !truth(tpl["bound"]) && str(tpl["kind"]) != "quest", "这件不能交易")
	n := math.Min(count, num(r["count"]))
	if str(tpl["kind"]) == "equipment" || r["extra"] != nil {
		n = 1
	}
	check(n >= 1, "数量不对")
	var ex any
	if r["extra"] != nil {
		ex = parse(str(r["extra"]))
	}
	snapshot := M{"templateId": r["template_id"], "count": n, "extra": ex}
	a.consume(r, n)
	t := now()
	a.run("INSERT INTO market_orders(id,seller_id,buyer_id,snapshot,price,status,created_at,updated_at) VALUES(?,?,NULL,?,?,'open',?,?)", uid(), id, js(snapshot), price, t, t)
	a.emit(id, M{"type": "market_list"})
	a.notify(id)
}

// buyOrder 独占变更开放订单，扣买家金币、发货并结算卖家税后收入。
func (a *App) buyOrder(id, order string) {
	r := a.one("SELECT * FROM market_orders WHERE id=?", order)
	check(str(r["status"]) == "open", "已经被买走了")
	seller := str(r["seller_id"])
	check(seller != id, "不能买自己的东西")
	// 条件更新只接受 open 状态，配合业务锁和事务防止两人重复成交。
	check(a.run("UPDATE market_orders SET status='sold',buyer_id=?,updated_at=? WHERE id=? AND status='open'", id, now(), order) == 1, "已经被买走了")
	price := num(r["price"])
	a.addGold(id, -price)
	// 税率来自配置，税额向下取整，卖家收入即时入账而非等待领取邮件。
	tax := math.Floor(price * a.constant("marketTax"))
	a.addGold(seller, price-tax)
	s := obj(parse(str(r["snapshot"])))
	var ex M
	if s["extra"] != nil {
		ex = obj(s["extra"])
	}
	a.addItem(id, str(s["templateId"]), num(s["count"]), ex, "bag")
	a.mail(seller, "拍卖成交", fmt.Sprintf("%s 买下了 %s x%g。你得到 %g 金币（税 %g）。", str(a.player(id)["name"]), str(a.def("items", str(s["templateId"]))["name"]), num(s["count"]), price-tax, tax), nil)
	a.emit(id, M{"type": "market_buy"})
	a.notify(id)
	a.notify(seller)
}

// cancelOrder 校验订单归属并关闭订单，将快照中的物品归还卖家。
func (a *App) cancelOrder(id, order string) {
	r := a.one("SELECT * FROM market_orders WHERE id=?", order)
	check(str(r["status"]) == "open", "订单不在")
	check(str(r["seller_id"]) == id, "不是你的订单")
	check(a.run("UPDATE market_orders SET status='cancelled',updated_at=? WHERE id=? AND status='open'", now(), order) == 1, "订单不在")
	s := obj(parse(str(r["snapshot"])))
	var ex M
	if s["extra"] != nil {
		ex = obj(s["extra"])
	}
	a.addItem(id, str(s["templateId"]), num(s["count"]), ex, "bag")
	a.notify(id)
}

// guilds 输出公会列表，并补充当前人数和会长名称。
func (a *App) guilds() []M {
	out := a.rows("SELECT id,name,level,exp,notice,leader_id FROM guilds ORDER BY level DESC,exp DESC LIMIT 40")
	for _, g := range out {
		g["members"] = num(a.one("SELECT COUNT(*) AS n FROM guild_members WHERE guild_id=?", g["id"])["n"])
		g["leaderName"] = a.player(str(g["leader_id"]))["name"]
	}
	return out
}

// createGuild 校验名称与成员关系，收取创建费用并建立会长记录。
func (a *App) createGuild(id, name string) {
	check(validName(name, 2, 8), "公会名需要 2 到 8 个字")
	check(a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id) == nil, "你已经在公会里")
	check(a.one("SELECT id FROM guilds WHERE name=?", name) == nil, "已经有这个公会了")
	a.addGold(id, -a.constant("guildCost"))
	gid := uid()
	a.run("INSERT INTO guilds(id,name,leader_id,level,exp,notice,created_at) VALUES(?,?,?,1,0,'欢迎加入。',?)", gid, name, id, now())
	a.run("INSERT INTO guild_members(guild_id,player_id,role,joined_at) VALUES(?,?,'leader',?)", gid, id, now())
	a.emit(id, M{"type": "join_guild"})
	a.touchAlbum(id, nil)
	a.notify(id)
}

// joinGuild 检查公会存在、人数上限和已有成员关系，再写入成员记录。
func (a *App) joinGuild(id, gid string) {
	check(a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id) == nil, "你已经在公会里")
	check(a.one("SELECT id FROM guilds WHERE id=?", gid) != nil, "没有这个公会")
	check(num(a.one("SELECT COUNT(*) AS n FROM guild_members WHERE guild_id=?", gid)["n"]) < 30, "公会满了")
	a.run("INSERT INTO guild_members(guild_id,player_id,role,joined_at) VALUES(?,?,'member',?)", gid, id, now())
	a.emit(id, M{"type": "join_guild"})
	a.touchAlbum(id, nil)
	a.notify(id)
}

// leaveGuild 移除成员；有其他成员时会长不能离开，空公会会被清理。
func (a *App) leaveGuild(id string) {
	r := a.one("SELECT * FROM guild_members WHERE player_id=?", id)
	check(r != nil, "你不在公会里")
	n := num(a.one("SELECT COUNT(*) AS n FROM guild_members WHERE guild_id=? AND player_id<>?", r["guild_id"], id)["n"])
	check(str(r["role"]) != "leader" || n == 0, "还有成员时会长不能离开")
	a.run("DELETE FROM guild_members WHERE player_id=?", id)
	if n == 0 {
		a.run("DELETE FROM guilds WHERE id=?", r["guild_id"])
	}
	a.notify(id)
}

// donateGuild 扣除捐赠金币并增加公会经验，按累计经验更新公会等级。
func (a *App) donateGuild(id string, gold float64) {
	check(gold >= 50 && gold == math.Floor(gold), "至少捐 50 金币")
	r := a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id)
	check(r != nil, "你不在公会里")
	a.addGold(id, -gold)
	g := a.one("SELECT exp FROM guilds WHERE id=?", r["guild_id"])
	exp := num(g["exp"]) + gold
	a.run("UPDATE guilds SET exp=?,level=? WHERE id=?", exp, math.Min(10, 1+math.Floor(exp/500)), r["guild_id"])
	a.emit(id, M{"type": "donate"})
	a.notify(id)
}

// listMail 将邮件存档转换为客户端列表，包含附件、已读和领取状态。
func (a *App) listMail(id string) []M {
	out := []M{}
	for _, r := range a.rows("SELECT * FROM mail WHERE player_id=? ORDER BY created_at DESC LIMIT 40", id) {
		out = append(out, M{"id": r["id"], "title": r["title"], "body": r["body"], "attachments": parse(str(r["attachments"])), "read": truth(r["is_read"]), "claimed": truth(r["claimed"]), "createdAt": r["created_at"]})
	}
	return out
}

// claimMail 只允许领取所属邮件一次；标记领取后发放金币和物品附件。
func (a *App) claimMail(id, mail string) {
	r := a.one("SELECT * FROM mail WHERE id=? AND player_id=?", mail, id)
	check(r != nil, "没有这封信")
	check(!truth(r["claimed"]), "已经领过了")
	check(a.run("UPDATE mail SET claimed=1,is_read=1 WHERE id=? AND claimed=0", mail) == 1, "已经领过了")
	att := obj(parse(str(r["attachments"])))
	a.addGold(id, num(att["gold"]))
	for _, i := range objects(att["items"]) {
		var ex M
		if i["extra"] != nil {
			ex = obj(i["extra"])
		}
		a.addItem(id, str(i["templateId"]), num(i["count"]), ex, "bag")
	}
	a.notify(id)
}

// requestFriend 按角色名查找目标，排除自己与重复关系后发送好友申请。
func (a *App) requestFriend(id, name string) {
	p := a.one("SELECT id FROM players WHERE name=?", name)
	check(p != nil, "找不到这个旅人")
	other := str(p["id"])
	check(other != id, "不能加自己")
	check(a.one("SELECT id FROM friends WHERE (from_id=? AND to_id=?) OR (from_id=? AND to_id=?)", id, other, other, id) == nil, "申请或好友关系已经在了")
	a.run("INSERT INTO friends(id,from_id,to_id,status,created_at) VALUES(?,?,?,'pending',?)", uid(), id, other, now())
	a.toast(other, str(a.player(id)["name"])+" 想加你为好友")
	a.notify(other)
}

// acceptFriend 只允许收件人接受待处理的好友申请。
func (a *App) acceptFriend(id, request string) {
	check(a.run("UPDATE friends SET status='accepted' WHERE id=? AND to_id=? AND status='pending'", request, id) == 1, "没有这条申请")
	a.notify(id)
}

// friends 读取双向好友关系，补充对方名字、在线状态和申请方向。
func (a *App) friends(id string) []M {
	out := []M{}
	for _, r := range a.rows("SELECT * FROM friends WHERE from_id=? OR to_id=?", id, id) {
		other := str(r["from_id"])
		if other == id {
			other = str(r["to_id"])
		}
		out = append(out, M{"id": r["id"], "status": r["status"], "incoming": str(r["to_id"]) == id && str(r["status"]) == "pending", "name": a.player(other)["name"], "online": a.live[other] != nil})
	}
	return out
}

// channel 将公开频道名转换成世界、地图或公会的实际存储频道。
func (a *App) channel(id, ch string, posting bool) string {
	switch ch {
	case "world":
		return ch
	case "map":
		if l := a.live[id]; l != nil {
			return "map:" + str(l.State["mapId"])
		}
		check(!posting, "先进入世界才能用地图频道")
		return "map:" + str(a.player(id)["map_id"])
	case "guild":
		g := a.one("SELECT guild_id FROM guild_members WHERE player_id=?", id)
		if g == nil {
			check(!posting, "你不在公会里")
			return ""
		}
		return "guild:" + str(g["guild_id"])
	case "party":
		party := a.playerParty[id]
		if party == "" {
			check(!posting, "你不在队伍里")
			return ""
		}
		return "party:" + party
	}
	check(!posting, "没有这个频道")
	return "world"
}

// chatHistory 按频道读取最近聊天记录，并按时间从早到晚返回。
func (a *App) chatHistory(id, ch string) []M {
	stored := a.channel(id, ch, false)
	if stored == "" {
		return []M{}
	}
	r := a.rows("SELECT id,player_name,text,created_at FROM chat_messages WHERE channel=? ORDER BY created_at DESC LIMIT 30", stored)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

// postChat 整理文本、校验长度与发送频率，持久化后发给频道内的在线角色。
func (a *App) postChat(id, ch, text string) M {
	clean := strings.Join(strings.Fields(text), " ")
	check(clean != "" && utf8.RuneCountInString(clean) <= 80, "消息太长或是空的")
	stored := a.channel(id, ch, true)
	t := now()
	if l := a.live[id]; l != nil {
		check(t-num(l.State["chatAt"]) >= 700, "说得太快了")
		l.State["chatAt"] = t
	}
	mid := uid()
	name := a.player(id)["name"]
	a.run("INSERT INTO chat_messages(id,channel,player_id,player_name,text,created_at) VALUES(?,?,?,?,?,?)", mid, stored, id, name, clean, t)
	message := M{"type": "chat", "id": mid, "channel": stored, "name": name, "text": clean, "t": t}
	for pid, l := range a.live {
		if a.hears(pid, l, stored) {
			a.send(l, message)
		}
	}
	return message
}

// hears 判断在线角色是否该收到这条频道消息。
func (a *App) hears(pid string, l *Live, stored string) bool {
	switch {
	case stored == "world":
		return true
	case stored == "map:"+str(l.State["mapId"]):
		return true
	case strings.HasPrefix(stored, "guild:"):
		return a.one("SELECT guild_id FROM guild_members WHERE player_id=? AND guild_id=?", pid, strings.TrimPrefix(stored, "guild:")) != nil
	case strings.HasPrefix(stored, "party:"):
		return a.playerParty[pid] == strings.TrimPrefix(stored, "party:")
	default:
		return false
	}
}

// partyView 输出当前队伍的队长和成员名称，未组队时返回空值。
func (a *App) partyView(id string) any {
	p := a.parties[a.playerParty[id]]
	if p == nil {
		return nil
	}
	members := []M{}
	for _, v := range arr(p["memberIds"]) {
		pid := str(v)
		members = append(members, M{"id": pid, "name": a.player(pid)["name"]})
	}
	return M{"id": p["id"], "leaderId": p["leaderId"], "members": members}
}

// inviteParty 必要时创建队伍，再向在线目标发送限时组队邀请。
func (a *App) inviteParty(id, name string) {
	target := a.one("SELECT id FROM players WHERE name=?", name)
	check(target != nil, "找不到这个旅人")
	pid := str(target["id"])
	check(pid != id, "不能邀请自己")
	check(a.live[pid] != nil, "对方不在线")
	party := a.playerParty[id]
	if party == "" {
		party = uid()
		a.parties[party] = M{"id": party, "leaderId": id, "memberIds": []any{id}}
		a.playerParty[id] = party
	}
	check(len(arr(a.parties[party]["memberIds"])) < 4, "队伍满了")
	a.invites[pid] = M{"fromId": id, "fromName": a.player(id)["name"], "until": now() + 60000}
	a.toast(pid, str(a.player(id)["name"])+" 邀请你组队")
}

// joinParty 验证邀请期限与队伍容量，加入目标队伍并清理邀请。
func (a *App) joinParty(id string) {
	i := a.invites[id]
	check(i != nil && num(i["until"]) >= now(), "邀请过期了")
	p := a.parties[a.playerParty[str(i["fromId"])]]
	check(p != nil, "队伍已经散了")
	members := arr(p["memberIds"])
	check(len(members) < 4 || contains(members, id), "队伍满了")
	if old := a.playerParty[id]; old != "" && old != str(p["id"]) {
		a.leaveParty(id)
	}
	if !contains(members, id) {
		p["memberIds"] = append(members, id)
	}
	a.playerParty[id] = str(p["id"])
	delete(a.invites, id)
}

// leaveParty 从队伍移除角色；队长离开时转交队长，最后一人离开时解散。
func (a *App) leaveParty(id string) {
	pid := a.playerParty[id]
	delete(a.playerParty, id)
	p := a.parties[pid]
	if p == nil {
		return
	}
	members := []any{}
	for _, v := range arr(p["memberIds"]) {
		if str(v) != id {
			members = append(members, v)
		}
	}
	p["memberIds"] = members
	if len(members) == 0 {
		delete(a.parties, pid)
	} else if str(p["leaderId"]) == id {
		p["leaderId"] = members[0]
	}
}

// partyKey 以队长或单人角色 ID 标识副本归属，让同队成员进入同一副本。
func (a *App) partyKey(id string) string {
	if p := a.parties[a.playerParty[id]]; p != nil {
		return str(p["leaderId"])
	}
	return id
}

// setLoadout 校验四个互不重复的当前武器技能，并同步存档与在线技能栏。
func (a *App) setLoadout(id string, ids any) {
	values := arr(ids)
	check(len(values) == 4, "技能栏是 4 个")
	seen := map[string]bool{}
	weapon := str(a.player(id)["weapon_type"])
	for _, v := range values {
		sid := str(v)
		check(!seen[sid], "技能不能重复")
		seen[sid] = true
		check(str(a.def("skills", sid)["weapon"]) == weapon, "这不是当前武器的技能")
	}
	a.run("UPDATE players SET skill_loadout=?,updated_at=? WHERE id=?", js(values), now(), id)
	if l := a.live[id]; l != nil {
		l.State["loadout"] = values
	}
	a.notify(id)
}
