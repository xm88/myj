// 街头日常：摊位招牌和货架、队伍目标、镇民好感、两味合剂。
// 不改回合、伤害、掉落、税、门和步速。合剂不进原来的 16 条配方，也不涨熟练。

package main

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// regardCap 一位镇民最多记到这里。再聊、再送都不加，也不发金币。
const regardCap = 9

// stallShelves 是摊上的四层架子。按物品种类归，不改标价和税。
var stallShelves = []M{
	{"id": "arms", "name": "兵器架"},
	{"id": "tonic", "name": "丹食架"},
	{"id": "stock", "name": "材料架"},
	{"id": "sundry", "name": "杂货架"},
}

// partyAims 是队长定的目标。只给队员看方向，不拉人，也不改号令。
var partyAims = []M{
	{"id": "follow", "name": "跟上", "hint": "队员看着队长在哪边"},
	{"id": "hunt", "name": "打怪", "hint": "先清眼前的怪。出手仍听号令"},
	{"id": "gather", "name": "采集", "hint": "就地采一轮，再会合"},
	{"id": "town", "name": "回镇", "hint": "做完就回星落镇"},
}

// regardFaces 是会记熟脸的镇民。爱收的东西只从背包送出，不打折。
var regardFaces = []M{
	{"id": "npc_elder", "liked": "herb_green", "warm": "又是你。先把镇里的事做完，再往林子里走。", "known": "艾兰看见你就点头。熟脸了，进镇不用再自我介绍。"},
	{"id": "npc_smith", "liked": "temper_grit", "warm": "火候我看着。你常来，砂和料放着就行。", "known": "霍特认得你的锤子声。兵器还是你的，我只负责敲。"},
	{"id": "npc_shop", "liked": "slime_gel", "warm": "凝胶要是多，可以留给自己熬药，也可以摆出去。", "known": "米娜把你算成熟客。价还是标价，不加也不减。"},
	{"id": "npc_ranger", "liked": "herb_green", "warm": "林子今天还安静。熟路的话，别一个人往蘑菇王那边闯。", "known": "林守记得你。东边是镇，西边照旧危险。"},
}

// blendCatalog 是合剂台的两味。材料和金币扣掉，产物进背包。不涨生活熟练，也不完成打造任务。
var blendCatalog = []M{
	{
		"id": "gel_pill", "name": "凝胶丸", "gold": 5,
		"blurb":  "和史莱姆药剂同一味。材料可以在背包或仓库，熟练不涨。",
		"inputs": []M{{"itemId": "slime_gel", "count": 3}},
		"output": M{"itemId": "potion_hp", "count": 1},
	},
	{
		"id": "dew_tea", "name": "露茶包", "gold": 8,
		"blurb":  "月光露配翠叶草，沏成回蓝茶。比料理贵，熟练不涨。",
		"inputs": []M{{"itemId": "moon_dew", "count": 1}, {"itemId": "herb_green", "count": 2}},
		"output": M{"itemId": "potion_mp", "count": 1},
	},
}

func stallShelf(kind string) (string, string) {
	switch kind {
	case "equipment":
		return "arms", "兵器架"
	case "consumable":
		return "tonic", "丹食架"
	case "material":
		return "stock", "材料架"
	default:
		return "sundry", "杂货架"
	}
}

func aimByID(id string) M {
	for _, row := range partyAims {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func regardFace(id string) M {
	for _, row := range regardFaces {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func blendByID(id string) M {
	for _, row := range blendCatalog {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

func regardTier(n float64) (string, string) {
	switch {
	case n >= 9:
		return "known", "认得你"
	case n >= 6:
		return "familiar", "熟脸"
	case n >= 3:
		return "knownish", "面熟"
	case n >= 1:
		return "met", "见过"
	default:
		return "stranger", "陌生"
	}
}

// cleanSign 收成 0 到 16 个字。空着就是没有招牌。
func cleanSign(sign string) string {
	clean := strings.Join(strings.Fields(strings.TrimSpace(sign)), "")
	n := utf8.RuneCountInString(clean)
	check(n <= 16, "招牌最多 16 个字")
	if clean != "" {
		check(validName(clean, 1, 16), "招牌里有不能用的字")
	}
	return clean
}

// setStallSign 改自己摊上的一句话。不改摊名、标价和旅税。
func (a *App) setStallSign(id, sign string) {
	stall := a.ownStall(id)
	clean := cleanSign(sign)
	a.run("UPDATE stalls SET sign=? WHERE id=?", clean, stall["id"])
	if clean == "" {
		a.toast(id, "招牌收起来了")
	} else {
		a.toast(id, "招牌写成了「"+clean+"」")
	}
	a.notify(id)
}

func (a *App) regardBlob(id string) M {
	r := a.player(id)
	blob := obj(parse(str(r["regard"])))
	if blob == nil {
		return M{}
	}
	return blob
}

func (a *App) saveRegard(id string, blob M) {
	if blob == nil {
		blob = M{}
	}
	a.run("UPDATE players SET regard=?,updated_at=? WHERE id=?", js(blob), now(), id)
}

func (a *App) regardN(id, npcID string) float64 {
	return num(obj(a.regardBlob(id)[npcID])["n"])
}

// noteRegard 一天和同一位镇民第一句算一次。不加金币、经验和物品。
func (a *App) noteRegard(id, npcID string) {
	face := regardFace(npcID)
	if face == nil {
		return
	}
	blob := a.regardBlob(id)
	row := obj(blob[npcID])
	day := guildDay(now())
	if num(row["talk"]) == day {
		return
	}
	before := num(row["n"])
	next := before
	if before < regardCap {
		next = before + 1
	}
	row["n"] = next
	row["talk"] = day
	blob[npcID] = row
	a.saveRegard(id, blob)
	a.regardToast(id, npcID, before, next)
	a.notify(id)
}

func (a *App) regardToast(id, npcID string, before, next float64) {
	if next <= before {
		return
	}
	name := str(a.def("npcs", npcID)["name"])
	switch next {
	case 3:
		a.toast(id, name+"对你面熟了")
	case 6:
		a.toast(id, name+"记得你的脸了")
	case 9:
		a.toast(id, name+"认得你了。名帖上可以挂「镇上的熟脸」，不加金币")
		a.refreshPlate(id)
	}
}

// giveRegard 从背包送一件对方爱收的东西。一天一次，到顶就不再收。不找零。
func (a *App) giveRegard(id, npcID string) {
	a.nearNPC(id, npcID)
	face := regardFace(npcID)
	check(face != nil, "这个人不收东西")
	blob := a.regardBlob(id)
	row := obj(blob[npcID])
	check(num(row["n"]) < regardCap, "已经认得你了，不用再送")
	day := guildDay(now())
	check(num(row["gift"]) != day, "今天已经送过了")
	liked := str(face["liked"])
	tpl := a.def("items", liked)
	check(tpl != nil, "没有这件东西")
	check(a.bagCount(id, liked) >= 1, "身上没有"+str(tpl["name"]))
	a.takeStock(id, liked, 1)
	before := num(row["n"])
	row["n"] = before + 1
	row["gift"] = day
	blob[npcID] = row
	a.saveRegard(id, blob)
	a.toast(id, "把"+str(tpl["name"])+"送给了"+str(a.def("npcs", npcID)["name"])+"。没有找零")
	a.regardToast(id, npcID, before, before+1)
	a.notify(id)
}

func (a *App) regardView(id, npcID string) M {
	face := regardFace(npcID)
	if face == nil {
		return nil
	}
	blob := a.regardBlob(id)
	row := obj(blob[npcID])
	n := num(row["n"])
	_, tierName := regardTier(n)
	liked := str(face["liked"])
	likedName := str(a.def("items", liked)["name"])
	day := guildDay(now())
	gifted := num(row["gift"]) == day
	return M{
		"npcId": npcID, "n": n, "cap": regardCap, "tierName": tierName,
		"liked": liked, "likedName": likedName, "gifted": gifted,
		"canGift": n < regardCap && !gifted && a.bagCount(id, liked) >= 1,
	}
}

func (a *App) regardList(id string) []M {
	blob := a.regardBlob(id)
	out := []M{}
	for _, face := range regardFaces {
		npcID := str(face["id"])
		n := num(obj(blob[npcID])["n"])
		_, tierName := regardTier(n)
		out = append(out, M{"id": npcID, "name": a.def("npcs", npcID)["name"], "n": n, "cap": regardCap, "tierName": tierName})
	}
	return out
}

func (a *App) anyRegardKnown(id string) bool {
	blob := a.regardBlob(id)
	for _, face := range regardFaces {
		if num(obj(blob[str(face["id"])])["n"]) >= regardCap {
			return true
		}
	}
	return false
}

// setPartyAim 只让队长改目标。人不移动，号令也不改。
func (a *App) setPartyAim(id, aim string) {
	p := a.parties[a.playerParty[id]]
	check(p != nil, "你还没有队伍")
	check(str(p["leaderId"]) == id, "只有队长能定目标")
	row := aimByID(aim)
	check(row != nil, "没有这个目标")
	p["aim"] = aim
	for _, v := range arr(p["memberIds"]) {
		pid := str(v)
		if pid == id {
			a.toast(pid, "队伍目标定成「"+str(row["name"])+"」。步子和出手没变")
		} else {
			a.toast(pid, "队长把目标定成「"+str(row["name"])+"」")
		}
	}
}

// followOf 给队员一个方向。队长自己只看见目标名。不写坐标进存档。
func (a *App) followOf(id string) M {
	p := a.parties[a.playerParty[id]]
	if p == nil {
		return nil
	}
	aim := str(p["aim"])
	aimName := ""
	if row := aimByID(aim); row != nil {
		aimName = str(row["name"])
	}
	leaderID := str(p["leaderId"])
	if leaderID == id {
		text := "你是队长"
		if aimName != "" {
			text += " · 目标「" + aimName + "」"
		} else {
			text += " · 还没定目标"
		}
		return M{"lead": true, "text": text, "aim": aim, "aimName": aimName, "same": false, "far": false}
	}
	base := M{"lead": false, "aim": aim, "aimName": aimName, "same": false, "far": false}
	prefix := ""
	if aimName != "" {
		prefix = "目标「" + aimName + "」· "
	}
	lead := a.live[leaderID]
	if lead == nil {
		base["text"] = prefix + "队长不在线"
		return base
	}
	mapID := str(lead.State["mapId"])
	mapName := mapID
	if m := a.def("maps", mapID); m != nil {
		mapName = str(m["name"])
	}
	base["mapId"] = mapID
	base["mapName"] = mapName
	self := a.live[id]
	if self == nil || str(self.State["mapId"]) != mapID {
		base["text"] = prefix + "队长在" + mapName
		return base
	}
	dx := num(lead.State["x"]) - num(self.State["x"])
	dy := num(lead.State["y"]) - num(self.State["y"])
	gap := math.Hypot(dx, dy)
	compass := compassOf(dx, dy)
	base["same"] = true
	base["x"] = num(lead.State["x"])
	base["y"] = num(lead.State["y"])
	base["gap"] = math.Round(gap)
	base["compass"] = compass
	base["far"] = gap >= 48
	if compass == "旁边" {
		base["text"] = prefix + "队长就在旁边"
	} else {
		base["text"] = prefix + fmt.Sprintf("队长在%s · %s · 约 %g 步", mapName, compass, math.Round(gap))
	}
	return base
}

// compassOf 把队长相对自己的位置收成八个方向。画面上方是北。
func compassOf(dx, dy float64) string {
	if math.Hypot(dx, dy) < 48 {
		return "旁边"
	}
	ax, ay := math.Abs(dx), math.Abs(dy)
	east := dx >= 0
	south := dy >= 0
	if ay < ax*0.45 {
		if east {
			return "东"
		}
		return "西"
	}
	if ax < ay*0.45 {
		if south {
			return "南"
		}
		return "北"
	}
	ns, ew := "北", "西"
	if south {
		ns = "南"
	}
	if east {
		ew = "东"
	}
	return ns + ew
}

func (a *App) bagCount(id, tpl string) float64 {
	n := 0.0
	for _, row := range a.stockRows(id, tpl) {
		if str(row["location"]) == "bag" {
			n += num(row["count"])
		}
	}
	return n
}

func (a *App) stockCount(id, tpl string) float64 {
	n := 0.0
	for _, row := range a.stockRows(id, tpl) {
		n += num(row["count"])
	}
	return n
}

// stockRows 先背包、再仓库。装备词条不在这里，避免把兵器当材料。
func (a *App) stockRows(id, tpl string) []M {
	return a.rows("SELECT * FROM player_items WHERE player_id=? AND template_id=? AND extra IS NULL AND location IN ('bag','bank') ORDER BY CASE location WHEN 'bag' THEN 0 ELSE 1 END, created_at", id, tpl)
}

func (a *App) takeStock(id, tpl string, count float64) {
	check(count > 0, "数量不对")
	rows := a.stockRows(id, tpl)
	have := 0.0
	for _, row := range rows {
		have += num(row["count"])
	}
	name := str(a.def("items", tpl)["name"])
	if name == "" {
		name = "材料"
	}
	check(have >= count, name+"不够")
	left := count
	for _, row := range rows {
		if left <= 0 {
			break
		}
		n := math.Min(left, num(row["count"]))
		a.consume(row, n)
		left -= n
	}
}

// blend 扣背包和仓库里的材料。金币和产物走原来的函数，不涨熟练，也不记打造任务。
func (a *App) blend(id, recipeID string) {
	row := blendByID(recipeID)
	check(row != nil, "没有这味合剂")
	for _, in := range objects(row["inputs"]) {
		tpl := a.def("items", str(in["itemId"]))
		check(tpl != nil, "材料不存在")
		check(a.stockCount(id, str(in["itemId"])) >= num(in["count"]), str(tpl["name"])+"不够")
	}
	output := obj(row["output"])
	tpl := a.def("items", str(output["itemId"]))
	check(tpl != nil && str(tpl["kind"]) != "equipment", "这味合不了")
	a.addGold(id, -num(row["gold"]))
	for _, in := range objects(row["inputs"]) {
		a.takeStock(id, str(in["itemId"]), num(in["count"]))
	}
	a.addItem(id, str(output["itemId"]), num(output["count"]), nil, "bag")
	a.noteDay(id, "hand")
	a.toast(id, "合好了："+str(tpl["name"])+"。熟练没涨")
	a.notify(id)
}
