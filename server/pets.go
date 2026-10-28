// 灵伴与派遣：主战、成长、离线计时、奖励领取及世界新闻；领取状态持久化防止重复发放。

package main

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// busy 判断灵伴是否仍有未领取的派遣，防止重复派遣或同时出战。
func (a *App) busy(pet string) bool {
	return a.one("SELECT id FROM pet_dispatch WHERE player_pet_id=? AND status='running'", pet) != nil
}

// jobs 读取角色仍在进行或等待领取的派遣记录。
func (a *App) jobs(id string) []M {
	return a.rows("SELECT * FROM pet_dispatch WHERE player_id=? AND status='running'", id)
}

// givePet 发放灵伴并处理重复契约；没有主战灵伴时自动出战，稀有契约产生新闻。
func (a *App) givePet(id, tpl string, capture bool) {
	def := a.def("pets", tpl)
	check(def != nil, "没有这种灵伴")
	old := a.one("SELECT id FROM player_pets WHERE player_id=? AND template_id=?", id, tpl)
	if old != nil {
		a.addGold(id, 30)
		a.mail(id, "已经契约过了", str(def["name"])+" 把多余的契约化成了 30 金币。", nil)
	} else {
		active := 0
		if a.one("SELECT id FROM player_pets WHERE player_id=? AND active=1", id) == nil {
			active = 1
		}
		a.run("INSERT INTO player_pets(id,player_id,template_id,level,exp,active,created_at) VALUES(?,?,?,1,0,?,?)", uid(), id, tpl, active, now())
		a.refresh(id)
		if str(def["rarity"]) == "epic" {
			a.pushNews("旅人「" + str(a.player(id)["name"]) + "」与" + str(def["name"]) + "建立了契约。")
		}
	}
	if capture {
		a.emit(id, M{"type": "capture", "petId": tpl})
		a.toast(id, "契约成功："+str(def["name"]))
	} else {
		a.toast(id, str(def["name"])+" 加入了旅团")
	}
	a.notify(id)
}

// news 读取持久化的最近世界新闻。
func (a *App) news() []M {
	return objects(parse(str(a.one("SELECT value FROM world_state WHERE key='news'")["value"])))
}

// pushNews 保存最多二十条新闻，并向在线客户端广播新消息。
func (a *App) pushNews(text string) {
	list := append([]M{{"t": now(), "text": text}}, a.news()...)
	if len(list) > 20 {
		list = list[:20]
	}
	a.run("INSERT INTO world_state(key,value) VALUES('news',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", js(list))
	a.broadcast(M{"type": "news", "text": text})
}

// activePet 切换或取消主战灵伴，派遣中的灵伴不能出战。
func (a *App) activePet(id, pet string) {
	if pet != "" {
		p := a.one("SELECT id FROM player_pets WHERE id=? AND player_id=?", pet, id)
		check(p != nil, "没有这只灵伴")
		check(!a.busy(pet), "它还在派遣途中")
	}
	a.run("UPDATE player_pets SET active=0 WHERE player_id=?", id)
	if pet != "" {
		a.run("UPDATE player_pets SET active=1 WHERE id=?", pet)
	}
	a.refresh(id)
	a.notify(id)
}

// petExp 累计灵伴经验并按等级曲线升级，上限为二十级。
func (a *App) petExp(pet string, amount float64) {
	p := a.one("SELECT * FROM player_pets WHERE id=?", pet)
	if p == nil || amount <= 0 || num(p["level"]) >= 20 {
		return
	}
	level, exp := num(p["level"]), num(p["exp"])+amount
	for level < 20 && exp >= 24+level*18 {
		exp -= 24 + level*18
		level++
	}
	a.run("UPDATE player_pets SET level=?,exp=? WHERE id=?", level, exp, pet)
}

// startDispatch 校验灵伴归属、忙碌状态和路线解锁，取消出战后持久化结束时间。
func (a *App) startDispatch(id, pet, node, duration string) {
	p := a.player(id)
	owned := a.one("SELECT * FROM player_pets WHERE id=? AND player_id=?", pet, id)
	check(owned != nil, "没有这只灵伴")
	check(!a.busy(pet), "它已经出门了")
	check(len(a.jobs(id)) < 3, "同时最多派遣 3 只")
	n, d := a.def("dispatchNodes", node), a.def("durations", duration)
	check(n != nil && d != nil, "没有这条派遣")
	check(num(p["level"]) >= num(n["unlockLevel"]), fmt.Sprintf("需要 %g 级", num(n["unlockLevel"])))
	a.run("UPDATE player_pets SET active=0 WHERE id=?", pet)
	a.refresh(id)
	t := now()
	a.run("INSERT INTO pet_dispatch(id,player_id,player_pet_id,node_id,duration_id,started_at,ends_at,status,result,created_at) VALUES(?,?,?,?,?,?,?,'running',NULL,?)", uid(), id, pet, node, duration, t, t+num(d["ms"]), t)
	a.emit(id, M{"type": "dispatch_start"})
	a.toast(id, str(a.def("pets", str(owned["template_id"]))["name"])+"出发了："+str(n["name"]))
	a.notify(id)
}

// claimDispatch 确认派遣已结束且未领取，按专长、天气和等级计算奖励并只发放一次。
func (a *App) claimDispatch(id, job string) {
	r := a.one("SELECT * FROM pet_dispatch WHERE id=? AND player_id=?", job, id)
	check(str(r["status"]) == "running", "没有这次派遣")
	check(now() >= num(r["ends_at"]), "还没回来")
	p := a.one("SELECT * FROM player_pets WHERE id=?", r["player_pet_id"])
	n, d := a.def("dispatchNodes", str(r["node_id"])), a.def("durations", str(r["duration_id"]))
	check(p != nil && n != nil && d != nil, "派遣数据坏了")
	def := a.def("pets", str(p["template_id"]))
	dispatch := obj(def["dispatch"])
	// 派遣专长、通用加成、探索加成和当前天气先累加，再乘灵伴等级成长。
	eff := 1 + num(dispatch[str(n["kind"])]) + num(dispatch["versatile"]) + num(dispatch["explore"])*.5 + num(obj(def["weather"])[str(a.weather["id"])])
	if l := a.live[id]; l != nil {
		for _, b := range objects(l.State["buffs"]) {
			if str(b["id"]) == "food_explore" && num(b["until"]) > num(r["started_at"]) {
				eff += .15
				break
			}
		}
	}
	eff *= 1 + (num(p["level"])-1)*.03
	random := .85 + rand.Float64()*.3
	items := []M{}
	for _, b := range objects(n["basePerHour"]) {
		items = append(items, M{"itemId": b["itemId"], "count": math.Max(1, math.Round(num(b["count"])*num(d["mul"])*eff*random))})
	}
	event := rand.Float64() < num(d["eventChance"])+num(dispatch["explore"])*.2
	result := M{"items": items, "event": event, "efficiency": math.Round(eff*100) / 100}
	// 先改变领取状态再发奖励；整个调用处于事务中，失败不会留下半笔领取。
	check(a.run("UPDATE pet_dispatch SET status='claimed',result=? WHERE id=? AND status='running'", js(result), job) == 1, "已经领过了")
	for _, i := range items {
		a.addItem(id, str(i["itemId"]), num(i["count"]), nil, "bag")
	}
	// 探索事件写入隐藏线索并发放地图碎片和邮件，后续任务按标记解锁。
	if event {
		a.setFlag(id, "hidden_cave")
		a.addItem(id, "map_fragment", 1, nil, "bag")
		a.mail(id, "灵伴有发现", str(def["name"])+" 找到了不在航标上的入口。去告诉艾兰。", nil)
	}
	a.petExp(str(p["id"]), 16)
	a.trackExp(id, "life:gather", 10, false)
	a.emit(id, M{"type": "dispatch_claim"})
	a.toast(id, "派遣回来了")
	a.notify(id)
}

// finishDispatch 开发模式下将派遣结束时间提前；生产模式拒绝使用测试功能。
func (a *App) finishDispatch(id, job string) {
	if !a.dev {
		panic(GameError{"测试开关没开", 403})
	}
	check(a.run("UPDATE pet_dispatch SET ends_at=? WHERE id=? AND player_id=? AND status='running'", now()-1, job, id) == 1, "没有进行中的派遣")
	a.notify(id)
}
