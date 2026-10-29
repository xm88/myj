// 连续世界：各地图仍是一块块分线，但相邻传送门按世界坐标对齐。
// 走到界边会在门的另一侧落地，而不是被拉回入口。缝隙附近的人、摊和怪物可以隔着边界互动。

package main

import "math"

// buildOrigins 从星落镇出发，把互相有门的地图原点对齐到同一扇门上。
func (a *App) buildOrigins() map[string][2]float64 {
	origins := map[string][2]float64{"town_starfall": {0, 0}}
	changed := true
	for pass := 0; changed && pass < 8; pass++ {
		changed = false
		for _, m := range objects(a.data["maps"]) {
			id := str(m["id"])
			if str(m["kind"]) == "dungeon" {
				continue
			}
			if _, placed := origins[id]; !placed {
				continue
			}
			for _, p := range objects(m["portals"]) {
				to := str(p["to"])
				dest := a.def("maps", to)
				if dest == nil || str(dest["kind"]) == "dungeon" {
					continue
				}
				if _, placed := origins[to]; placed {
					continue
				}
				src, dst, ok := a.link(id, to)
				if !ok {
					continue
				}
				origins[to] = [2]float64{
					origins[id][0] + num(src["x"]) - num(dst["x"]),
					origins[id][1] + num(src["y"]) - num(dst["y"]),
				}
				changed = true
			}
		}
	}
	// 没有成对门的野外才临时放到镇北。雷鸣、风鸣已经按门对齐，不应再走到这里。
	for _, m := range objects(a.data["maps"]) {
		id := str(m["id"])
		if str(m["kind"]) == "dungeon" {
			continue
		}
		if _, placed := origins[id]; placed {
			continue
		}
		origins[id] = [2]float64{0, -num(m["height"])}
	}
	// 副本不接进大陆，但镜头仍要有一个原点，单独把这一张画出来。
	for _, m := range objects(a.data["maps"]) {
		id := str(m["id"])
		if str(m["kind"]) != "dungeon" {
			continue
		}
		if _, placed := origins[id]; !placed {
			origins[id] = [2]float64{0, 0}
		}
	}
	return origins
}

// originOf 返回地图在连续世界上的原点。
func (a *App) originOf(mapID string) (float64, float64, bool) {
	if a.origins == nil {
		a.origins = a.buildOrigins()
	}
	o, ok := a.origins[mapID]
	if !ok {
		return 0, 0, false
	}
	return o[0], o[1], true
}

// worldPos 把地图内坐标换成连续世界坐标。
func (a *App) worldPos(mapID string, x, y float64) (float64, float64) {
	ox, oy, ok := a.originOf(mapID)
	if !ok {
		return x, y
	}
	return ox + x, oy + y
}

// link 找出两张图之间成对的传送门。单侧门不算缝。
func (a *App) link(from, to string) (M, M, bool) {
	var src, dst M
	for _, p := range objects(a.def("maps", from)["portals"]) {
		if str(p["to"]) == to {
			src = p
			break
		}
	}
	for _, p := range objects(a.def("maps", to)["portals"]) {
		if str(p["to"]) == from {
			dst = p
			break
		}
	}
	if src == nil || dst == nil {
		return nil, nil, false
	}
	return src, dst, true
}

// bridgeGap 计算同一张图上的距离，或两扇对齐的门之间的缝隙距离。
// 只有双方都站在门附近时，缝隙才算数，避免隔着整张图打到另一边。
func (a *App) bridgeGap(mapA string, posA M, mapB string, posB M) (float64, bool) {
	if mapA == mapB {
		return dist(posA, posB), true
	}
	src, dst, ok := a.link(mapA, mapB)
	if !ok {
		return 1e9, false
	}
	if dist(posA, src) > 220 || dist(posB, dst) > 220 {
		return 1e9, false
	}
	vx := num(src["x"]) + num(posB["x"]) - num(dst["x"])
	vy := num(src["y"]) + num(posB["y"]) - num(dst["y"])
	return math.Hypot(num(posA["x"])-vx, num(posA["y"])-vy), true
}

// ensurePublic 找到或创建一张野外/城镇分线，方便缝隙另一侧还没人时也能刷出怪物和摊位。
func (a *App) ensurePublic(mapID string) *Instance {
	m := a.def("maps", mapID)
	if m == nil || str(m["kind"]) == "dungeon" {
		return nil
	}
	lineCap := int(a.constant("lineCap"))
	if lineCap < 1 {
		lineCap = 20
	}
	var best *Instance
	bestLine := math.MaxInt
	for _, inst := range a.instances {
		if inst.MapID == mapID && !inst.Dungeon && len(inst.Players) < lineCap && inst.Line < bestLine {
			best = inst
			bestLine = inst.Line
		}
	}
	if best != nil {
		return best
	}
	for i := 1; ; i++ {
		key := mapID + "#" + itoa(i)
		if a.instances[key] != nil {
			continue
		}
		inst := &Instance{Key: key, MapID: mapID, Line: i, Monsters: map[string]M{}, Players: map[string]bool{}, Nodes: M{}, RareAt: now() + 20000}
		for _, s := range objects(m["spawns"]) {
			a.spawnMonster(inst, str(s["monsterId"]), num(s["x"]), num(s["y"]))
		}
		a.instances[key] = inst
		return inst
	}
}

// itoa 避免 chunk 路径再引一层 strconv 的同时保持可读。
func itoa(n int) string {
	return fmtInt(n)
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := []byte{}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// cross 走进相邻区域。落地在对面门内侧，不回到该图入口。副本仍整图进入。
func (a *App) cross(l *Live, to string, strict bool) {
	s := l.State
	if str(s["stallId"]) != "" || str(s["boutId"]) != "" || str(s["tradeId"]) != "" {
		if strict {
			fail("先把摊位、战斗或交易了结")
		}
		return
	}
	m := a.def("maps", to)
	if m == nil {
		if strict {
			fail("没有这张地图")
		}
		return
	}
	if num(s["level"]) < num(m["unlockLevel"]) {
		if strict {
			fail("等级还不够")
		}
		return
	}
	if str(m["kind"]) == "dungeon" {
		if strict {
			a.enter(l, to, true)
		}
		return
	}
	_, dst, ok := a.link(str(s["mapId"]), to)
	a.enter(l, to, false)
	if ok {
		cx, cy := num(m["width"])/2, num(m["height"])/2
		px, py := num(dst["x"]), num(dst["y"])
		dx, dy := cx-px, cy-py
		d := math.Hypot(dx, dy)
		if d < 1 {
			d = 1
		}
		s["x"] = clamp(px+dx/d*48, 20, num(m["width"])-20)
		s["y"] = clamp(py+dy/d*48, 20, num(m["height"])-20)
	} else {
		s["x"] = num(m["width"]) / 2
		s["y"] = 48
	}
	s["crossUntil"] = now() + 900
	a.toast(str(s["id"]), "走进"+str(m["name"]))
}

// maybeCross 人走到门的外侧时自动过界。刚落地的一小段时间不会立刻被弹回去。
func (a *App) maybeCross(l *Live) {
	s := l.State
	if num(s["crossUntil"]) > now() || truth(s["safe"]) {
		return
	}
	if str(s["stallId"]) != "" || str(s["boutId"]) != "" || str(s["tradeId"]) != "" {
		return
	}
	m := a.def("maps", str(s["mapId"]))
	if m == nil || str(m["kind"]) == "dungeon" {
		return
	}
	w, h := num(m["width"]), num(m["height"])
	x, y := num(s["x"]), num(s["y"])
	for _, p := range objects(m["portals"]) {
		to := str(p["to"])
		dest := a.def("maps", to)
		if dest == nil || str(dest["kind"]) == "dungeon" {
			continue
		}
		px, py := num(p["x"]), num(p["y"])
		edge := gateEdge(px, py, w, h)
		if edge == "" || math.Abs(x-px) > 140 || math.Abs(y-py) > 140 {
			continue
		}
		hit := false
		switch edge {
		case "west":
			hit = x <= px+8
		case "east":
			hit = x >= px-8
		case "north":
			hit = y <= py+8
		case "south":
			hit = y >= py-8
		}
		if hit {
			a.cross(l, to, false)
			return
		}
	}
}

// gateEdge 根据门靠近哪一条边，决定人要走出哪一侧才算过界。
func gateEdge(x, y, w, h float64) string {
	switch {
	case x <= 80:
		return "west"
	case x >= w-80:
		return "east"
	case y <= 80:
		return "north"
	case y >= h-80:
		return "south"
	default:
		return ""
	}
}

// gates 列出当前地图可以走过去的相邻区域，供小地图标出界边。
func (a *App) gates(mapID string) []M {
	m := a.def("maps", mapID)
	out := []M{}
	if m == nil {
		return out
	}
	w, h := num(m["width"]), num(m["height"])
	for _, p := range objects(m["portals"]) {
		dest := a.def("maps", str(p["to"]))
		if dest == nil {
			continue
		}
		out = append(out, M{"to": p["to"], "name": dest["name"], "x": p["x"], "y": p["y"], "edge": gateEdge(num(p["x"]), num(p["y"]), w, h), "dungeon": str(dest["kind"]) == "dungeon"})
	}
	return out
}

// seamEntities 把缝隙另一侧靠近门的人、怪、摊和 NPC 投影到当前地图，方便隔着边界看见。
func (a *App) seamEntities(l *Live) []M {
	out := []M{}
	here := str(l.State["mapId"])
	m := a.def("maps", here)
	if m == nil || str(m["kind"]) == "dungeon" {
		return out
	}
	if distToNearestGate(l.State, objects(m["portals"])) > 200 {
		return out
	}
	line := 1
	if inst := a.instances[l.InstanceKey]; inst != nil && inst.Line > 0 {
		line = inst.Line
	}
	for _, g := range a.gates(here) {
		if truth(g["dungeon"]) {
			continue
		}
		to := str(g["to"])
		src, dst, ok := a.link(here, to)
		if !ok || dist(l.State, src) > 200 {
			continue
		}
		for _, n := range objects(a.data["npcs"]) {
			if str(n["mapId"]) != to || dist(n, dst) > 220 {
				continue
			}
			x, y := project(src, dst, n)
			out = append(out, M{"id": "npc:" + str(n["id"]), "kind": "npc", "name": n["name"], "x": x, "y": y, "mapId": to, "focus": "npc:" + str(n["id"]), "seam": true})
		}
		if inst := a.ensurePublic(to); inst != nil {
			_ = line
			for _, mob := range inst.Monsters {
				if !truth(mob["alive"]) || dist(mob, dst) > 220 {
					continue
				}
				x, y := project(src, dst, mob)
				d := a.def("monsters", str(mob["templateId"]))
				out = append(out, M{"id": mob["id"], "kind": "monster", "templateId": mob["templateId"], "name": mob["name"], "x": x, "y": y, "hp": math.Max(0, num(mob["hp"])), "maxHp": mob["maxHp"], "alive": true, "tint": d["tint"], "role": d["role"], "radius": d["radius"], "capturable": str(d["petId"]) != "", "seam": true, "mapId": to})
			}
			for id := range inst.Players {
				if id == str(l.State["id"]) {
					continue
				}
				p := a.live[id]
				if p == nil || dist(p.State, dst) > 220 {
					continue
				}
				x, y := project(src, dst, p.State)
				out = append(out, M{"id": id, "kind": "player", "name": p.State["name"], "x": x, "y": y, "hp": p.State["hp"], "maxHp": p.Stats["maxHp"], "alive": true, "seam": true, "mapId": to})
			}
		}
		for _, stall := range a.rows("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.open=1 AND s.map_id=?", to) {
			if dist(stall, dst) > 220 {
				continue
			}
			x, y := project(src, dst, stall)
			out = append(out, M{"id": "stall:" + str(stall["id"]), "kind": "stall", "stallId": stall["id"], "name": stall["title"], "owner": stall["owner_name"], "x": x, "y": y, "seam": true, "mapId": to})
		}
	}
	return out
}

func distToNearestGate(pos M, portals []M) float64 {
	best := 1e9
	for _, p := range portals {
		if d := dist(pos, p); d < best {
			best = d
		}
	}
	return best
}

func project(src, dst, ent M) (float64, float64) {
	return num(src["x"]) + num(ent["x"]) - num(dst["x"]), num(src["y"]) + num(ent["y"]) - num(dst["y"])
}

// cameraSpan 是自由镜头默认看见的世界宽度。相邻地块按原点铺在同一张面上。
const cameraSpan = 1100.0

// cameraChunks 列出当前地图和与它成对相连的邻块。副本不把大陆带进来。
func (a *App) cameraChunks(mapID string) []M {
	m := a.def("maps", mapID)
	out := []M{}
	if m == nil {
		return out
	}
	ids := []string{mapID}
	if str(m["kind"]) != "dungeon" {
		seen := map[string]bool{mapID: true}
		for _, p := range objects(m["portals"]) {
			to := str(p["to"])
			if seen[to] {
				continue
			}
			dest := a.def("maps", to)
			if dest == nil || str(dest["kind"]) == "dungeon" {
				continue
			}
			if _, _, ok := a.link(mapID, to); !ok {
				continue
			}
			seen[to] = true
			ids = append(ids, to)
		}
	}
	for _, id := range ids {
		mm := a.def("maps", id)
		ox, oy, ok := a.originOf(id)
		if mm == nil || !ok {
			continue
		}
		out = append(out, M{"id": id, "name": mm["name"], "kind": mm["kind"], "width": mm["width"], "height": mm["height"], "originX": ox, "originY": oy})
	}
	return out
}

// afarEntities 把相邻地块上、镜头够得着的人、怪和摊按各自的地图坐标送出。
// 世界坐标由快照统一换算。副本没有邻块。
func (a *App) afarEntities(l *Live) []M {
	out := []M{}
	here := str(l.State["mapId"])
	m := a.def("maps", here)
	if m == nil || str(m["kind"]) == "dungeon" {
		return out
	}
	px, py := a.worldPos(here, num(l.State["x"]), num(l.State["y"]))
	line := 1
	if inst := a.instances[l.InstanceKey]; inst != nil && inst.Line > 0 {
		line = inst.Line
	}
	for _, g := range a.gates(here) {
		if truth(g["dungeon"]) {
			continue
		}
		to := str(g["to"])
		if _, _, ok := a.link(here, to); !ok {
			continue
		}
		inst := a.publicLine(to, line)
		if inst == nil {
			continue
		}
		for _, mob := range inst.Monsters {
			if !truth(mob["alive"]) {
				continue
			}
			wx, wy := a.worldPos(to, num(mob["x"]), num(mob["y"]))
			if math.Hypot(wx-px, wy-py) > 1600 {
				continue
			}
			d := a.def("monsters", str(mob["templateId"]))
			out = append(out, M{"id": mob["id"], "kind": "monster", "templateId": mob["templateId"], "name": mob["name"], "x": mob["x"], "y": mob["y"], "hp": math.Max(0, num(mob["hp"])), "maxHp": mob["maxHp"], "alive": true, "tint": d["tint"], "role": d["role"], "radius": d["radius"], "capturable": str(d["petId"]) != "", "mapId": to, "afar": true})
		}
		for id := range inst.Players {
			if id == str(l.State["id"]) {
				continue
			}
			p := a.live[id]
			if p == nil {
				continue
			}
			wx, wy := a.worldPos(to, num(p.State["x"]), num(p.State["y"]))
			if math.Hypot(wx-px, wy-py) > 1600 {
				continue
			}
			out = append(out, M{"id": id, "kind": "player", "name": p.State["name"], "x": p.State["x"], "y": p.State["y"], "hp": p.State["hp"], "maxHp": p.Stats["maxHp"], "alive": true, "mapId": to, "afar": true})
		}
		for _, stall := range a.rows("SELECT s.*, p.name AS owner_name FROM stalls s JOIN players p ON p.id=s.player_id WHERE s.open=1 AND s.map_id=?", to) {
			wx, wy := a.worldPos(to, num(stall["x"]), num(stall["y"]))
			if math.Hypot(wx-px, wy-py) > 1600 {
				continue
			}
			out = append(out, M{"id": "stall:" + str(stall["id"]), "kind": "stall", "stallId": stall["id"], "name": stall["title"], "owner": stall["owner_name"], "x": stall["x"], "y": stall["y"], "mapId": to, "afar": true})
		}
	}
	return out
}

// publicLine 取指定分线；那条线还没人时退回一条公开线，方便邻块有怪可看。
func (a *App) publicLine(mapID string, line int) *Instance {
	for _, inst := range a.instances {
		if inst.MapID == mapID && !inst.Dungeon && inst.Line == line {
			return inst
		}
	}
	return a.ensurePublic(mapID)
}
