// 迁移兼容性测试：对照 TypeScript 基准，检查种族武器面板和进程重启后的持久化数据。

package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These expected values were produced by the original TypeScript service,
// across every race/weapon combination with gear, pet aura, attributes and a buff.
// TestLegacyStatsParity 使用旧版基准比较十二组种族和武器组合，验证面板数值与文字展示一致。
func TestLegacyStatsParity(t *testing.T) {
	a := testApp(t)
	raw, e := os.ReadFile("testdata/legacy-stats.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixtures []M
	if e = json.Unmarshal(raw, &fixtures); e != nil {
		t.Fatal(e)
	}
	for i, f := range fixtures {
		user := uid()
		a.run("INSERT INTO users(id,email,password_hash,is_guest,created_at) VALUES(?,?,?,0,?)", user, user+"@example.com", "", now())
		a.createCharacter(user, "兼容"+string(rune('A'+i)), str(f["weapon"]), str(f["race"]))
		id := str(a.requirePlayer(user)["id"])
		a.run("UPDATE players SET level=10,attributes=? WHERE id=?", js(M{"vit": 3, "mag": 4, "str": 5, "agi": 6, "end": 7}), id)
		a.addItem(id, "wpn_sword_1", 1, M{"quality": "blue", "affixes": []M{{"id": "test", "name": "测试", "stat": "elem_fire", "value": .12}}}, "equipped")
		a.givePet(id, "forest_cat", false)
		got := a.stats(id, 10, str(f["weapon"]), []M{{"id": "test", "stat": "atkPct", "value": .15, "until": now() + 600000}})
		for k, v := range obj(f["stats"]) {
			if k == "elem" || k == "elemRes" {
				if !reflect.DeepEqual(obj(parse(js(got[k]))), obj(v)) {
					t.Errorf("%s/%s %s=%v want %v", f["race"], f["weapon"], k, got[k], v)
				}
				continue
			}
			if math.Abs(num(got[k])-num(v)) > 1e-8 {
				t.Errorf("%s/%s %s=%v want %v", f["race"], f["weapon"], k, got[k], v)
			}
		}
		if !reflect.DeepEqual(parse(js(a.profile(id)["attributeRows"])), f["attributeRows"]) {
			t.Errorf("%s/%s attribute panel changed", f["race"], f["weapon"])
		}
	}
}

// TestRestartPreservesAccountInventoryAndDispatch 重开同一数据库，检查角色位置、金币、任务、灵伴及未领取派遣仍然存在。
func TestRestartPreservesAccountInventoryAndDispatch(t *testing.T) {
	root, _ := filepath.Abs("..")
	file := filepath.Join(t.TempDir(), "persistent.sqlite")
	a := newApp(root, file)
	token, id := newGuest(t, a)
	a.givePet(id, "forest_cat", false)
	pet := str(a.one("SELECT id FROM player_pets WHERE player_id=?", id)["id"])
	a.startDispatch(id, pet, "node_green_herb", "scout")
	job := str(a.jobs(id)[0]["id"])
	a.addGold(id, 37)
	a.attach(id, nil)
	a.live[id].State["x"] = 777.
	a.save(a.live[id])
	a.db.Close()
	reopened := newApp(root, file)
	defer reopened.db.Close()
	p := okRequest(t, reopened, "/api/profile", token, nil)
	if str(p["id"]) != id || num(p["x"]) != 777 || num(p["gold"]) != reopened.constant("starterGold")+37 || len(arr(p["pets"])) != 1 || len(arr(p["dispatches"])) != 1 || aJobID(p) != job || len(arr(p["quests"])) != 1 {
		t.Fatalf("restart lost data: %v", p)
	}
	reopened.finishDispatch(id, job)
	reopened.transaction(func() { reopened.claimDispatch(id, job) })
	if reopened.countItem(id, "herb_green") <= 0 {
		t.Fatal("persisted dispatch did not pay out")
	}
}

// aJobID 从测试档案中取首条派遣 ID，帮助断言重启前后记录一致。
func aJobID(p M) string {
	jobs := objects(p["dispatches"])
	if len(jobs) == 0 {
		return ""
	}
	return str(jobs[0]["id"])
}
