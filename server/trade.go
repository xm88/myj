// 当面交易：两个靠近的人打开双栏窗口，放入物品和金币，锁定后再确认。
// 东西在成交前仍留在背包里，服务器在成交时再校验距离、归属和数量。

package main

import "encoding/json"

// TradeOffer 是一侧栏里准备交出去的金币和物品。
type TradeOffer struct {
	Gold  float64 `json:"gold"`
	Items []M     `json:"items"`
}

// Trade 一场还没写进数据库的当面交易。成交时才动背包和金币。
type Trade struct {
	ID      string                `json:"id"`
	A       string                `json:"a"`
	B       string                `json:"b"`
	Offer   map[string]TradeOffer `json:"offer"`
	Lock    map[string]bool       `json:"lock"`
	Confirm map[string]bool       `json:"confirm"`
}

func cloneTrade(t *Trade) *Trade {
	if t == nil {
		return nil
	}
	var c Trade
	must(json.Unmarshal([]byte(js(t)), &c))
	if c.Offer == nil {
		c.Offer = map[string]TradeOffer{}
	}
	if c.Lock == nil {
		c.Lock = map[string]bool{}
	}
	if c.Confirm == nil {
		c.Confirm = map[string]bool{}
	}
	return &c
}

func (a *App) newTrade(aID, bID string) *Trade {
	t := &Trade{
		ID: uid(), A: aID, B: bID,
		Offer:   map[string]TradeOffer{aID: {Items: []M{}}, bID: {Items: []M{}}},
		Lock:    map[string]bool{},
		Confirm: map[string]bool{},
	}
	a.trades[t.ID] = t
	a.tradeBy[aID] = t.ID
	a.tradeBy[bID] = t.ID
	if la := a.live[aID]; la != nil {
		la.State["tradeId"] = t.ID
		halt(la, true)
	}
	if lb := a.live[bID]; lb != nil {
		lb.State["tradeId"] = t.ID
		halt(lb, true)
	}
	return t
}

// inviteTrade 靠近对方时直接打开交易窗。
func (a *App) inviteTrade(id, name string) {
	l := a.mustLive(id)
	check(!truth(l.State["safe"]), "切回冒险才能交易")
	check(str(l.State["stallId"]) == "", "收摊之后才能交易")
	check(str(l.State["boutId"]) == "", "战斗里不能交易")
	check(a.tradeBy[id] == "", "你已经在交易")
	target := a.one("SELECT id,name FROM players WHERE name=?", name)
	check(target != nil, "找不到这个旅人")
	pid := str(target["id"])
	check(pid != id, "不能和自己交易")
	other := a.live[pid]
	check(other != nil, "对方不在线")
	check(!truth(other.State["safe"]), "对方正在用助手")
	check(str(other.State["stallId"]) == "", "对方正在摆摊")
	check(str(other.State["boutId"]) == "", "对方正在战斗")
	check(a.tradeBy[pid] == "", "对方正在交易")
	a.mustNear(l, other)
	a.newTrade(id, pid)
	a.pushTrade(id)
	a.pushTrade(pid)
	a.toast(pid, str(l.State["name"])+" 想和你当面交易")
	a.toast(id, "交易窗开了")
}

func (a *App) mustTrade(id string) *Trade {
	t := a.trades[a.tradeBy[id]]
	check(t != nil, "没有进行中的交易")
	return t
}

func (a *App) mustNear(l, other *Live) {
	gap, ok := a.bridgeGap(str(l.State["mapId"]), l.State, str(other.State["mapId"]), other.State)
	check(ok && gap <= 140, "离得太远")
}

func (a *App) partnerOf(t *Trade, id string) string {
	if t.A == id {
		return t.B
	}
	return t.A
}

// offerTrade 放入或改写自己这一侧的金币和物品。改动会解开双方的锁定。
func (a *App) offerTrade(id string, body M) {
	t := a.mustTrade(id)
	check(!t.Confirm[id], "你已经确认了")
	l := a.mustLive(id)
	other := a.live[a.partnerOf(t, id)]
	check(other != nil, "对方离开了")
	a.mustNear(l, other)
	offer := t.Offer[id]
	if body["gold"] != nil {
		gold := num(body["gold"])
		check(gold >= 0 && gold == float64(int(gold)) && gold <= maxGold, "金币不对")
		check(gold <= num(a.player(id)["gold"]), "金币不够")
		offer.Gold = gold
	}
	if str(body["itemId"]) != "" {
		r := a.takeItem(id, str(body["itemId"]))
		check(str(r["location"]) == "bag", "只能交易背包里的东西")
		tpl := a.def("items", str(r["template_id"]))
		check(tpl != nil && !truth(tpl["bound"]) && str(tpl["kind"]) != "quest", "这件不能交易")
		n := num(body["count"])
		if n <= 0 {
			n = 1
		}
		n = float64(int(n))
		if str(tpl["kind"]) == "equipment" || r["extra"] != nil {
			n = 1
		}
		check(n >= 1 && n <= num(r["count"]), "数量不对")
		items := []M{}
		for _, it := range offer.Items {
			if str(it["itemId"]) != str(r["id"]) {
				items = append(items, it)
			}
		}
		items = append(items, M{"itemId": r["id"], "templateId": r["template_id"], "count": n, "name": tpl["name"]})
		offer.Items = items
	}
	if str(body["remove"]) != "" {
		items := []M{}
		for _, it := range offer.Items {
			if str(it["itemId"]) != str(body["remove"]) {
				items = append(items, it)
			}
		}
		offer.Items = items
	}
	t.Offer[id] = offer
	t.Lock[t.A], t.Lock[t.B] = false, false
	t.Confirm[t.A], t.Confirm[t.B] = false, false
	a.pushTrade(t.A)
	a.pushTrade(t.B)
}

// lockTrade 锁住自己这一侧。两边都锁上之后才能确认。
func (a *App) lockTrade(id string) {
	t := a.mustTrade(id)
	l := a.mustLive(id)
	other := a.live[a.partnerOf(t, id)]
	check(other != nil, "对方离开了")
	a.mustNear(l, other)
	t.Lock[id] = true
	t.Confirm[id] = false
	a.pushTrade(t.A)
	a.pushTrade(t.B)
}

// confirmTrade 双方都锁定并确认后成交。
func (a *App) confirmTrade(id string) {
	t := a.mustTrade(id)
	check(t.Lock[t.A] && t.Lock[t.B], "要两边都锁定")
	l := a.mustLive(id)
	other := a.live[a.partnerOf(t, id)]
	check(other != nil, "对方离开了")
	a.mustNear(l, other)
	t.Confirm[id] = true
	if !(t.Confirm[t.A] && t.Confirm[t.B]) {
		a.pushTrade(t.A)
		a.pushTrade(t.B)
		return
	}
	a.settleTrade(t)
}

func (a *App) settleTrade(t *Trade) {
	la, lb := a.live[t.A], a.live[t.B]
	check(la != nil && lb != nil, "有人离开了")
	a.mustNear(la, lb)
	a.giveSide(t.B, t.A, t.Offer[t.A])
	a.giveSide(t.A, t.B, t.Offer[t.B])
	a.toast(t.A, "交易完成")
	a.toast(t.B, "交易完成")
	a.notify(t.A)
	a.notify(t.B)
	a.clearTrade(t)
	a.send(la, M{"type": "trade", "trade": nil})
	a.send(lb, M{"type": "trade", "trade": nil})
}

// giveSide 把 from 的出价交给 to。先校验再扣，避免成交到一半才发现物品不在。
func (a *App) giveSide(to, from string, offer TradeOffer) {
	if offer.Gold > 0 {
		a.addGold(from, -offer.Gold)
		a.addGold(to, offer.Gold)
	}
	for _, it := range offer.Items {
		r := a.takeItem(from, str(it["itemId"]))
		check(str(r["location"]) == "bag", "物品不在背包里")
		n := num(it["count"])
		check(n >= 1 && n <= num(r["count"]), "数量不对")
		var ex M
		if r["extra"] != nil && str(r["extra"]) != "" {
			ex = obj(parse(str(r["extra"])))
		}
		a.consume(r, n)
		a.addItem(to, str(r["template_id"]), n, ex, "bag")
	}
}

// cancelTrade 任意一边都可以取消，物品不动。
func (a *App) cancelTrade(id string) {
	t := a.trades[a.tradeBy[id]]
	if t == nil {
		return
	}
	a.toast(t.A, "交易取消了")
	a.toast(t.B, "交易取消了")
	la, lb := a.live[t.A], a.live[t.B]
	a.clearTrade(t)
	if la != nil {
		a.send(la, M{"type": "trade", "trade": nil})
	}
	if lb != nil {
		a.send(lb, M{"type": "trade", "trade": nil})
	}
}

func (a *App) clearTrade(t *Trade) {
	if la := a.live[t.A]; la != nil && str(la.State["tradeId"]) == t.ID {
		la.State["tradeId"] = ""
	}
	if lb := a.live[t.B]; lb != nil && str(lb.State["tradeId"]) == t.ID {
		lb.State["tradeId"] = ""
	}
	delete(a.tradeBy, t.A)
	delete(a.tradeBy, t.B)
	delete(a.trades, t.ID)
}

func (a *App) pushTrade(id string) {
	if l := a.live[id]; l != nil {
		a.send(l, M{"type": "trade", "trade": a.tradeView(id)})
	}
}

// tradeView 双栏视图。自己的栏和对方的栏分开。
func (a *App) tradeView(id string) any {
	t := a.trades[a.tradeBy[id]]
	if t == nil {
		return nil
	}
	other := a.partnerOf(t, id)
	return M{
		"id":    t.ID,
		"you":   a.sideView(id, t),
		"them":  a.sideView(other, t),
		"ready": t.Lock[t.A] && t.Lock[t.B],
	}
}

func (a *App) sideView(id string, t *Trade) M {
	p := a.player(id)
	offer := t.Offer[id]
	items := []M{}
	for _, it := range offer.Items {
		tpl := a.def("items", str(it["templateId"]))
		items = append(items, M{"itemId": it["itemId"], "templateId": it["templateId"], "name": fallback(tpl["name"], str(it["name"])), "count": it["count"]})
	}
	if items == nil {
		items = []M{}
	}
	return M{"id": id, "name": p["name"], "gold": offer.Gold, "items": items, "locked": t.Lock[id], "confirmed": t.Confirm[id]}
}
