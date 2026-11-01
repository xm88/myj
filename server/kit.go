// 行囊分仓、染色、航标旗和心法册。都只改存放位置或显示，不读伤害、税、门和步速。

package main

import (
	"math"
	"strings"
	"unicode/utf8"
)

// markCap 一个人最多插三面旗。多了就先拔掉旧的。
const markCap = 3

// shelfCatalog 是仓库里的三间。格子数仍是 bankCap，三间加在一起算。
var shelfCatalog = []M{
	{"id": "sundry", "name": "杂物仓", "blurb": "没指定时落在这里。旧档也在这间。"},
	{"id": "stock", "name": "材料仓", "blurb": "凝胶、矿和药草。"},
	{"id": "keeps", "name": "珍藏仓", "blurb": "想留着的兵器和物件。"},
}

// dyeCatalog 是旅团自己的六套衣色。暖茶就是原来的披风，空着不写库。
var dyeCatalog = []M{
	{"id": "tea", "name": "暖茶", "coat": "#d08a3c", "trim": "#f6e2a8", "hair": "#3a2414", "blurb": "旅团原来的茶色披风。不染也是这一身。"},
	{"id": "lake", "name": "湖青", "coat": "#3d7f8a", "trim": "#d5eef2", "hair": "#1c3338", "blurb": "月光湖边的青。只改衣色。"},
	{"id": "dusk", "name": "暮紫", "coat": "#6a4a86", "trim": "#e6d4f2", "hair": "#2a1830", "blurb": "傍晚留在披风上的紫。"},
	{"id": "frost", "name": "霜白", "coat": "#e4ddd0", "trim": "#b7c4ce", "hair": "#4a4038", "blurb": "高地上的薄霜色。"},
	{"id": "ember", "name": "赤霞", "coat": "#c4513a", "trim": "#f3c1a4", "hair": "#3a1810", "blurb": "赤岩晒过的红。步子不变。"},
	{"id": "pine", "name": "松绿", "coat": "#3d6b45", "trim": "#d7e7c0", "hair": "#1c2a1c", "blurb": "林子里的松绿。"},
}

func dyeByID(id string) M {
	for _, row := range dyeCatalog {
		if str(row["id"]) == id {
			return row
		}
	}
	return nil
}

// shelfOf 认出三间仓。空字符串是杂物仓，这样旧档不用改 bin。
func shelfOf(bin string) (string, bool) {
	switch strings.TrimSpace(bin) {
	case "", "sundry":
		return "sundry", true
	case "stock", "keeps":
		return strings.TrimSpace(bin), true
	default:
		return "", false
	}
}

// shownShelf 给界面看。认不出的也归到杂物，避免旧行从列表里消失。
func shownShelf(bin string) string {
	if shelf, ok := shelfOf(bin); ok {
		return shelf
	}
	return "sundry"
}

// verseLane 把现有技能分成三页。守身、范围或必中、其余。网页 verse.ts 用同一套，不改系数。
func verseLane(skill M) string {
	if num(skill["healPct"]) > 0 || skill["buff"] != nil || num(skill["shieldFlat"]) > 0 || num(skill["tauntMs"]) > 0 {
		return "guard"
	}
	if truth(skill["aoe"]) || truth(skill["forceCrit"]) {
		return "edge"
	}
	return "root"
}

// setDye 记下衣色。空着或暖茶都写回空字符串，造型用原来的茶色。速度和属性不读它。
func (a *App) setDye(id, dyeID string) {
	dyeID = strings.TrimSpace(dyeID)
	label := "暖茶"
	if dyeID == "tea" {
		dyeID = ""
	}
	if dyeID != "" {
		row := dyeByID(dyeID)
		check(row != nil, "没有这套染色")
		label = str(row["name"])
	}
	a.run("UPDATE players SET dye_id=?,updated_at=? WHERE id=?", dyeID, now(), id)
	if l := a.live[id]; l != nil {
		l.State["dye"] = dyeID
	}
	if dyeID == "" {
		a.toast(id, "染回了暖茶。属性和步子没变")
	} else {
		a.toast(id, "染上了"+label+"。属性和步子没变")
	}
	a.notify(id)
}

func (a *App) marksOf(id string) []M {
	out := []M{}
	for _, row := range objects(parse(str(a.player(id)["marks"]))) {
		if str(row["id"]) == "" || str(row["mapId"]) == "" {
			continue
		}
		out = append(out, M{"id": row["id"], "name": row["name"], "mapId": row["mapId"], "x": num(row["x"]), "y": num(row["y"])})
	}
	return out
}

func (a *App) saveMarks(id string, marks []M) {
	if marks == nil {
		marks = []M{}
	}
	a.run("UPDATE players SET marks=?,updated_at=? WHERE id=?", js(marks), now(), id)
}

// dropMark 把旗插在脚下。只记地图和坐标，走路和航标传送仍走原来的函数。
func (a *App) dropMark(id, name string) {
	l := a.live[id]
	check(l != nil, "先进入世界")
	clean := strings.Join(strings.Fields(strings.TrimSpace(name)), "")
	n := utf8.RuneCountInString(clean)
	check(n >= 1 && n <= 6, "旗子要有 1 到 6 个字的名字")
	mapID := str(l.State["mapId"])
	check(a.def("maps", mapID) != nil, "这张图不能插旗")
	marks := a.marksOf(id)
	check(len(marks) < markCap, "航标旗最多三面")
	marks = append(marks, M{
		"id": uid(), "name": clean, "mapId": mapID,
		"x": math.Round(num(l.State["x"])), "y": math.Round(num(l.State["y"])),
	})
	a.saveMarks(id, marks)
	a.toast(id, "插下了「"+clean+"」。换岛仍走原来的航标")
	a.notify(id)
}

func (a *App) forgetMark(id, markID string) {
	marks := a.marksOf(id)
	next := []M{}
	found := false
	for _, row := range marks {
		if str(row["id"]) == markID {
			found = true
			continue
		}
		next = append(next, row)
	}
	check(found, "没有这面旗")
	a.saveMarks(id, next)
	a.toast(id, "拔掉了这面旗")
	a.notify(id)
}

func (a *App) manualOf(id string) M {
	raw := obj(parse(str(a.player(id)["manual"])))
	studied := []any{}
	seen := map[string]bool{}
	for _, v := range arr(raw["studied"]) {
		sid := str(v)
		if sid == "" || seen[sid] {
			continue
		}
		seen[sid] = true
		studied = append(studied, sid)
	}
	return M{"focus": str(raw["focus"]), "studied": studied}
}

// studyVerse 翻开心法的一页。不改技能栏，也不改系数。栏上原来的四招照旧能放。
func (a *App) studyVerse(id, skillID string) {
	skill := a.def("skills", skillID)
	check(skill != nil, "没有这招")
	weapon := str(a.player(id)["weapon_type"])
	check(str(skill["weapon"]) == weapon, "这不是当前武器的心法")
	manual := a.manualOf(id)
	studied := arr(manual["studied"])
	seen := false
	for _, v := range studied {
		if str(v) == skillID {
			seen = true
		}
	}
	if !seen {
		studied = append(studied, skillID)
	}
	a.run("UPDATE players SET manual=?,updated_at=? WHERE id=?", js(M{"focus": skillID, "studied": studied}), now(), id)
	a.toast(id, "温习了"+str(skill["name"])+"。威力没变")
	a.notify(id)
}
