// 摆摊：人在地图上摆开一个摊位，标价上架，附近的人来买。摊位写在数据库里，收摊或下架把东西退回背包。

package main

import (
	"fmt"
	"math"
)

// openStall 在脚下开摊。副本、战斗和交易里不能摆。
func (a *App) openStall(id, title string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能摆摊")
	check(str(l.State["stallId"]) == "", "你已经在摆摊")
	check(str(l.State["boutId"]) == "", "战斗里不能摆摊")
	check(str(l.State["tradeId"]) == "", "交易里不能摆摊")
	m := a.def("maps", str(l.State["mapId"]))
	check(m != nil && str(m["kind"]) != "dungeon", "这里不能摆摊")
	check(validName(title, 2, 8), "摊名需要 2 到 8 个字")
	check(a.one("SELECT id FROM stalls WHERE player_id=? AND open=1", id) == nil, "你已经在摆摊")
	sid := uid()
	a.run("INSERT INTO stalls(id,player_id,map_id,x,y,title,open,created_at) VALUES(?,?,?,?,?,?,1,?)", sid, id, l.State["mapId"], num(l.State["x"]), num(l.State["y"]), title, now())
	l.State["stallId"] = sid
	halt(l, true)
	a.toast(id, "摊位开了："+title)
	a.notify(id)
}

// listStall 把背包里的东西按标价放到自己的摊上。
func (a *App) listStall(id, item string, count, price float64) {
	check(price >= 1 && price <= 1000000 && price == math.Floor(price), "标价不合法")
	stall := a.ownStall(id)
	r := a.takeItem(id, item)
	check(str(r["location"]) == "bag", "只能上架背包里的东西")
	tpl := a.def("items", str(r["template_id"]))
	check(tpl != nil && !truth(tpl["bound"]) && str(tpl["kind"]) != "quest", "这件不能摆出来")
	n := math.Min(math.Floor(count), num(r["count"]))
	var ex any
	if str(tpl["kind"]) == "equipment" || r["extra"] != nil {
		n = 1
		if r["extra"] != nil {
			ex = parse(str(r["extra"]))
		}
	}
	check(n >= 1, "数量不对")
	a.consume(r, n)
	extra := any(nil)
	if ex != nil {
		extra = js(ex)
	}
	a.run("INSERT INTO stall_goods(id,stall_id,template_id,count,price,extra,created_at) VALUES(?,?,?,?,?,?,?)", uid(), stall["id"], r["template_id"], n, price, extra, now())
	a.toast(id, "上架了 "+str(tpl["name"]))
	a.notify(id)
}

// unlistStall 把一件货物退回背包。
func (a *App) unlistStall(id, goods string) {
	stall := a.ownStall(id)
	g := a.one("SELECT * FROM stall_goods WHERE id=? AND stall_id=?", goods, stall["id"])
	check(g != nil, "摊上没有这件")
	a.returnGoods(id, g)
	a.run("DELETE FROM stall_goods WHERE id=?", goods)
	a.notify(id)
}

// closeStall 收摊，剩下的货物都退回背包。
func (a *App) closeStall(id string) {
	stall := a.ownStall(id)
	for _, g := range a.rows("SELECT * FROM stall_goods WHERE stall_id=?", stall["id"]) {
		a.returnGoods(id, g)
	}
	a.run("DELETE FROM stall_goods WHERE stall_id=?", stall["id"])
	a.run("DELETE FROM stalls WHERE id=?", stall["id"])
	if l := a.live[id]; l != nil {
		l.State["stallId"] = ""
	}
	a.toast(id, "摊收起来了")
	a.notify(id)
}

func (a *App) ownStall(id string) M {
	stall := a.one("SELECT * FROM stalls WHERE player_id=? AND open=1", id)
	check(stall != nil, "你还没有摆摊")
	return stall
}

func (a *App) returnGoods(id string, g M) {
	var ex M
	if g["extra"] != nil && str(g["extra"]) != "" {
		ex = obj(parse(str(g["extra"])))
	}
	a.addItem(id, str(g["template_id"]), num(g["count"]), ex, "bag")
}

// buyStall 购买别人摊上的货物。买家要站在摊位旁边，同一条缝也算。
func (a *App) buyStall(id, goods string, count float64) {
	g := a.one("SELECT * FROM stall_goods WHERE id=?", goods)
	check(g != nil, "已经被买走了")
	stall := a.one("SELECT * FROM stalls WHERE id=? AND open=1", g["stall_id"])
	check(stall != nil, "摊已经收了")
	seller := str(stall["player_id"])
	check(seller != id, "不能买自己的东西")
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能买")
	gap, ok := a.bridgeGap(str(l.State["mapId"]), l.State, str(stall["map_id"]), stall)
	check(ok && gap <= 96, "离摊位太远")
	n := math.Floor(count)
	if n < 1 {
		n = 1
	}
	check(n <= num(g["count"]), "数量不够")
	price := num(g["price"]) / num(g["count"]) * n
	if num(g["count"]) == n {
		price = num(g["price"])
	} else {
		price = math.Floor(num(g["price"]) * n / num(g["count"]))
		check(price >= 1, "标价不合法")
	}
	if n == num(g["count"]) {
		check(a.run("DELETE FROM stall_goods WHERE id=? AND count=?", goods, n) == 1, "已经被买走了")
	} else {
		check(a.run("UPDATE stall_goods SET count=count-? WHERE id=? AND count>=?", n, goods, n) == 1, "已经被买走了")
		left := num(g["price"]) - price
		a.run("UPDATE stall_goods SET price=? WHERE id=?", left, goods)
	}
	tax := math.Floor(price * a.constant("marketTax"))
	if tax < 0 {
		tax = 0
	}
	if tax > price {
		tax = price
	}
	a.addGold(id, -price)
	a.addGold(seller, price-tax)
	var ex M
	if g["extra"] != nil && str(g["extra"]) != "" {
		ex = obj(parse(str(g["extra"])))
	}
	a.addItem(id, str(g["template_id"]), n, ex, "bag")
	name := str(a.def("items", str(g["template_id"]))["name"])
	a.toast(id, "买到了 "+name)
	if tax > 0 {
		a.toast(seller, fmt.Sprintf("%s 买走了 %s。旅税 %g，到手 %g。", str(a.player(id)["name"]), name, tax, price-tax))
	} else {
		a.toast(seller, str(a.player(id)["name"])+" 买走了 "+name)
	}
	a.notify(id)
	a.notify(seller)
}

// stallsNear 当前角色看得见的摊，包括缝隙对面靠近门的那些。
func (a *App) stallsNear(id string) []M {
	l := a.live[id]
	if l == nil {
		return []M{}
	}
	out := []M{}
	seen := map[string]bool{}
	add := func(row M, seam bool) {
		sid := str(row["id"])
		if seen[sid] {
			return
		}
		gap, ok := a.bridgeGap(str(l.State["mapId"]), l.State, str(row["map_id"]), row)
		if !ok || gap > 420 {
			return
		}
		seen[sid] = true
		out = append(out, M{"id": sid, "title": row["title"], "owner": row["owner_name"], "ownerId": row["player_id"], "mapId": row["map_id"], "x": row["x"], "y": row["y"], "mine": str(row["player_id"]) == id, "seam": seam, "distance": math.Round(gap)})
	}
	for _, row := range a.rows("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.open=1 AND s.map_id=?", l.State["mapId"]) {
		add(row, false)
	}
	for _, ent := range a.seamEntities(l) {
		if str(ent["kind"]) != "stall" {
			continue
		}
		if row := a.one("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.id=? AND s.open=1", ent["stallId"]); row != nil {
			add(row, true)
		}
	}
	return out
}

// stallDetail 摊位上的货物。
func (a *App) stallDetail(id, stallID string) M {
	stall := a.one("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.id=? AND s.open=1", stallID)
	check(stall != nil, "摊已经收了")
	goods := []M{}
	for _, g := range a.rows("SELECT * FROM stall_goods WHERE stall_id=? ORDER BY created_at", stallID) {
		tpl := a.def("items", str(g["template_id"]))
		price := num(g["price"])
		tax := math.Floor(price * a.constant("marketTax"))
		goods = append(goods, M{"id": g["id"], "templateId": g["template_id"], "name": tpl["name"], "kind": tpl["kind"], "count": g["count"], "price": g["price"], "tax": tax, "net": price - tax, "desc": tpl["desc"]})
	}
	return M{"id": stall["id"], "title": stall["title"], "owner": stall["owner_name"], "ownerId": stall["player_id"], "mine": str(stall["player_id"]) == id, "mapId": stall["map_id"], "x": stall["x"], "y": stall["y"], "goods": goods}
}

// myStallView 档案里附带自己的摊，没有就空。
func (a *App) myStallView(id string) any {
	stall := a.one("SELECT id FROM stalls WHERE player_id=? AND open=1", id)
	if stall == nil {
		return nil
	}
	return a.stallDetail(id, str(stall["id"]))
}

// resumeStall 重连时把人放回自己的摊位，避免人走了摊还开着却对不上。
func (a *App) resumeStall(l *Live) {
	id := str(l.State["id"])
	stall := a.one("SELECT * FROM stalls WHERE player_id=? AND open=1", id)
	if stall == nil {
		l.State["stallId"] = ""
		return
	}
	if str(l.State["mapId"]) != str(stall["map_id"]) {
		a.enter(l, str(stall["map_id"]), false)
	}
	l.State["x"], l.State["y"] = stall["x"], stall["y"]
	l.State["stallId"] = stall["id"]
	halt(l, true)
}
