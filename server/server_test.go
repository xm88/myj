// 业务回归测试：使用独立临时存档验证鉴权、数值、事务、经济并发、派遣和世界协议。

package main

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testApp 在临时目录创建独立游戏存档，测试结束后关闭数据库。
func testApp(t *testing.T) *App {
	t.Helper()
	root, _ := filepath.Abs("..")
	a := newApp(root, filepath.Join(t.TempDir(), "game.sqlite"))
	t.Cleanup(func() { a.db.Close() })
	return a
}

// request 通过 HTTP 测试记录器调用接口，并返回 JSON 响应与状态码。
func request(t *testing.T, a *App, path, token string, body any) (M, int) {
	t.Helper()
	method := "GET"
	var raw []byte
	if body != nil {
		method = "POST"
		raw = []byte(js(body))
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.handleHTTP(w, r)
	var out M
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatalf("%s invalid JSON: %s", path, w.Body.String())
	}
	return out, w.Code
}

// okRequest 调用接口并要求成功，失败时打印路径与响应以定位回归问题。
func okRequest(t *testing.T, a *App, path, token string, body any) M {
	t.Helper()
	m, status := request(t, a, path, token, body)
	if status != 200 {
		t.Fatalf("%s: status=%d payload=%v", path, status, m)
	}
	return m
}

// newGuest 通过公开接口创建游客，返回令牌和角色 ID 供后续测试复用。
func newGuest(t *testing.T, a *App) (string, string) {
	t.Helper()
	m := okRequest(t, a, "/api/auth/guest", "", M{})
	token := str(m["token"])
	p := okRequest(t, a, "/api/profile", token, nil)
	return token, str(p["id"])
}

// expectFailure 要求操作抛出可识别的业务错误，防止把程序错误当成预期拒绝。
func expectFailure(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("expected failure")
		} else if _, ok := e.(GameError); !ok {
			panic(e)
		}
	}()
	fn()
}

// TestFormula 验证防御、元素下限、暴击、经验曲线和种族属性贡献的计算规则。
func TestFormula(t *testing.T) {
	cases := []struct {
		attack, coefficient, def, bonus, resist float64
		force                                   bool
		want                                    float64
	}{{100, 1, 0, 0, 0, false, 100}, {100, 2, 500, .2, 0, true, 180}, {100, 1, 0, 0, .5, false, 50}, {100, 1, 0, 0, 99, false, 20}}
	for _, c := range cases {
		got, crit := damage(c.attack, c.coefficient, c.def, c.bonus, c.resist, 0, 1.5, 1, c.force, func() float64 { return 1 })
		if got != c.want || crit != c.force {
			t.Errorf("damage %v = %g, %v", c, got, crit)
		}
	}
	if expNext(1) >= expNext(10) || expNext(10) >= expNext(30) {
		t.Fatal("experience curve")
	}
	for _, c := range []struct {
		race, key string
		want      float64
	}{{"human", "vit", 10}, {"demon", "vit", 20}, {"demon", "end", 0}, {"immortal", "mag", 20}, {"immortal", "vit", 0}} {
		if num(totalAttrs(10, emptyAttrs(), c.race)[c.key]) != c.want {
			t.Fatal(c)
		}
	}
	s := M{"maxHp": 100, "maxMp": 40, "atk": 10, "def": 4, "spell": 0, "attackInterval": 1100, "speed": 170}
	attrBonus(s, M{"vit": 2, "str": 1, "agi": 10, "end": 2, "mag": 3}, "human")
	for k, want := range (M{"maxHp": 110, "maxMp": 49, "atk": 10.66, "def": 7, "spell": 3.5, "attackInterval": 1070, "speed": 176}) {
		if math.Abs(num(s[k])-num(want)) > 1e-8 {
			t.Errorf("%s=%v want %v", k, s[k], want)
		}
	}
}

// TestAuthProfileAndValidation 验证初始角色、登录鉴权、加点预算及旧 bcrypt 密码摘要的兼容性。
func TestAuthProfileAndValidation(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	p := okRequest(t, a, "/api/profile", token, nil)
	if num(p["level"]) != 1 || len(arr(p["quests"])) != 1 || len(arr(p["bag"])) != 2 || len(arr(p["loadout"])) != 4 {
		t.Fatalf("starter profile: %v", p)
	}
	if _, code := request(t, a, "/api/profile", "", nil); code != 401 {
		t.Fatal("missing auth")
	}
	if _, code := request(t, a, "/api/profile", token+"x", nil); code != 401 {
		t.Fatal("forged token")
	}
	okRequest(t, a, "/api/attributes", token, M{"str": 5})
	if _, code := request(t, a, "/api/attributes", token, M{"vit": 1}); code != 400 {
		t.Fatal("overspent attributes")
	}
	okRequest(t, a, "/api/attributes/reset", token, M{})
	if num(a.profile(id)["unspent"]) != 5 {
		t.Fatal("reset")
	}
	auth := okRequest(t, a, "/api/auth/register", "", M{"email": "Test@Example.com", "password": "password"})
	if str(auth["token"]) == "" {
		t.Fatal("register")
	}
	okRequest(t, a, "/api/auth/login", "", M{"email": "test@example.com", "password": "password"})
	if _, code := request(t, a, "/api/auth/login", "", M{"email": "test@example.com", "password": "wrong"}); code != 401 {
		t.Fatal("bad password")
	}
	okRequest(t, a, "/api/characters", str(auth["token"]), M{"name": "仙族测试", "weaponType": "staff", "race": "immortal"})
	hash, _ := bcrypt.GenerateFromPassword([]byte("legacy-password"), 10)
	a.run("INSERT INTO users(id,email,password_hash,is_guest,created_at) VALUES('legacy','legacy@example.com',?,0,?)", strings.Replace(string(hash), "$2a$", "$2b$", 1), now())
	okRequest(t, a, "/api/auth/login", "", M{"email": "legacy@example.com", "password": "legacy-password"})
}

// TestTransactionRollback 验证材料不足或业务失败时，金币、物品与在线状态完整回滚。
func TestTransactionRollback(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.attach(id, nil)
	gold := num(a.profile(id)["gold"])
	before := num(a.live[id].State["gold"])
	_, code := request(t, a, "/api/craft", token, M{"recipeId": str(objects(a.data["recipes"])[0]["id"])})
	if code != 400 {
		t.Fatal("craft without materials should fail")
	}
	if num(a.player(id)["gold"]) != gold || num(a.live[id].State["gold"]) != before {
		t.Fatal("failed craft changed gold")
	}
	a.mail(id, "test", "rollback", M{"gold": 100})
	expectFailure(t, func() {
		a.transaction(func() { a.addGold(id, 10); a.addItem(id, "bread", 1, nil, "bag"); fail("rollback") })
	})
	if num(a.player(id)["gold"]) != gold || num(a.live[id].State["gold"]) != before || a.countItem(id, "bread") != 2 {
		t.Fatal("transaction leaked")
	}
}

// TestMarketConcurrentPurchaseAndClaims 模拟两人同时抢购，确认订单只能成交一次，邮件奖励也只能领取一次。
func TestMarketConcurrentPurchaseAndClaims(t *testing.T) {
	a := testApp(t)
	sellerToken, seller := newGuest(t, a)
	b1, i1 := newGuest(t, a)
	b2, i2 := newGuest(t, a)
	a.addGold(i1, 1000)
	a.addGold(i2, 1000)
	a.addItem(seller, "herb_green", 3, nil, "bag")
	item := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='herb_green'", seller)
	okRequest(t, a, "/api/market/list", sellerToken, M{"itemId": item["id"], "count": 1, "price": 100})
	order := str(a.market()[0]["id"])
	before := num(a.player(seller)["gold"])
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, token := range []string{b1, b2} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			_, code := request(t, a, "/api/market/buy", token, M{"id": order})
			statuses <- code
		}(token)
	}
	wg.Wait()
	close(statuses)
	success := 0
	for code := range statuses {
		if code == 200 {
			success++
		} else if code != 400 {
			t.Fatalf("unexpected status %d", code)
		}
	}
	if success != 1 || a.countItem(i1, "herb_green")+a.countItem(i2, "herb_green") != 1 {
		t.Fatal("order purchased more than once")
	}
	if num(a.player(seller)["gold"]) != before+100-math.Floor(100*a.constant("marketTax")) {
		t.Fatal("seller tax")
	}
	a.mail(i1, "奖励", "领取一次", M{"gold": 7})
	mail := str(a.one("SELECT id FROM mail WHERE player_id=? AND title='奖励'", i1)["id"])
	old := num(a.player(i1)["gold"])
	okRequest(t, a, "/api/mail/claim", b1, M{"id": mail})
	if _, code := request(t, a, "/api/mail/claim", b1, M{"id": mail}); code != 400 {
		t.Fatal("mail double claim")
	}
	if num(a.player(i1)["gold"]) != old+7 {
		t.Fatal("mail gold")
	}
}

// TestDispatchStorageAndPersistence 验证派遣时限、生产开关、重复领取、仓库转移与坐标保存。
func TestDispatchStorageAndPersistence(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	a.givePet(id, "forest_cat", false)
	pet := str(a.one("SELECT id FROM player_pets WHERE player_id=?", id)["id"])
	okRequest(t, a, "/api/dispatches", token, M{"petId": pet, "nodeId": "node_green_herb", "durationId": "scout"})
	job := str(a.jobs(id)[0]["id"])
	if _, code := request(t, a, "/api/dispatches/claim", token, M{"id": job}); code != 400 {
		t.Fatal("early claim")
	}
	a.dev = false
	if _, code := request(t, a, "/api/dispatches/finish", token, M{"id": job}); code != 403 {
		t.Fatal("production dev tools")
	}
	a.dev = true
	okRequest(t, a, "/api/dispatches/finish", token, M{"id": job})
	okRequest(t, a, "/api/dispatches/claim", token, M{"id": job})
	if a.countItem(id, "herb_green") <= 0 {
		t.Fatal("dispatch reward")
	}
	if _, code := request(t, a, "/api/dispatches/claim", token, M{"id": job}); code != 400 {
		t.Fatal("dispatch double claim")
	}
	item := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='herb_green'", id)
	okRequest(t, a, "/api/storage", token, M{"itemId": item["id"], "count": 1, "direction": "deposit"})
	bank := a.one("SELECT id FROM player_items WHERE player_id=? AND template_id='herb_green' AND location='bank'", id)
	okRequest(t, a, "/api/storage", token, M{"itemId": bank["id"], "count": 1, "direction": "withdraw"})
	a.attach(id, nil)
	a.live[id].State["x"] = 777.
	a.save(a.live[id])
	a.detach(id, nil)
	if num(a.player(id)["x"]) != 777 {
		t.Fatal("position not persisted")
	}
}

// TestWebSocketWorld 通过真实 WebSocket 验证世界消息、战斗快照和助手模式的移动限制。
func TestWebSocketWorld(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", a.handleSocket)
	mux.HandleFunc("/", a.handleHTTP)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(srv.URL, "http", "ws", 1)+"/ws?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var world M
	if conn.ReadJSON(&world) != nil || str(world["type"]) != "world" {
		t.Fatal("initial world message")
	}
	a.withLock(a.tick)
	var snapshot M
	if conn.ReadJSON(&snapshot) != nil || str(snapshot["type"]) != "snapshot" {
		t.Fatal("snapshot")
	}
	okRequest(t, a, "/api/travel", token, M{"mapId": "wild_greenleaf"})
	a.withLock(func() {
		l := a.live[id]
		i := a.instances[l.InstanceKey]
		for _, m := range i.Monsters {
			if str(m["templateId"]) == "slime" {
				l.State["x"], l.State["y"], l.State["targetId"] = m["x"], m["y"], m["id"]
				break
			}
		}
		a.tick()
		events := a.live[id].Events
		_ = events
	})
	seen := false
	for j := 0; j < 20; j++ {
		var m M
		if e := conn.ReadJSON(&m); e != nil {
			t.Fatal(e)
		}
		if str(m["type"]) == "snapshot" && str(m["mapId"]) == "wild_greenleaf" {
			for _, ev := range objects(m["events"]) {
				if str(ev["kind"]) == "dmg" {
					seen = true
				}
			}
			break
		}
	}
	if !seen {
		t.Fatal("combat did not emit server damage")
	}
	a.withLock(func() {
		l := a.live[id]
		before := num(l.State["x"])
		a.socketMessage(id, M{"type": "mode", "mode": "assistant"})
		a.socketMessage(id, M{"type": "input", "x": 1, "destX": 1000, "destY": 1000})
		a.tick()
		if num(l.State["x"]) != before {
			t.Error("assistant moved")
		}
	})
}

// TestGuardPartyAndGear 验证守势减伤与停手、队伍频道，以及装备对比行不改旧存档。
func TestGuardPartyAndGear(t *testing.T) {
	a := testApp(t)
	token, id := newGuest(t, a)
	p := okRequest(t, a, "/api/profile", token, nil)
	weapon := obj(obj(p["equipment"])["weapon"])
	found := false
	for _, line := range objects(weapon["lines"]) {
		if str(line["stat"]) == "atk" && num(line["value"]) == 6 {
			found = true
		}
	}
	if !found {
		t.Fatalf("starter weapon lines: %v", weapon["lines"])
	}
	expectFailure(t, func() { a.postChat(id, "party", "这边集合") })

	token2, id2 := newGuest(t, a)
	a.attach(id, nil)
	a.attach(id2, nil)
	a.inviteParty(id, str(a.player(id2)["name"]))
	a.joinParty(id2)
	msg := a.postChat(id, "party", "这边集合")
	if !strings.HasPrefix(str(msg["channel"]), "party:") {
		t.Fatal(msg)
	}
	history := a.chatHistory(id2, "party")
	if len(history) != 1 || str(history[0]["text"]) != "这边集合" {
		t.Fatalf("party history: %v", history)
	}
	_ = token2

	okRequest(t, a, "/api/travel", token, M{"mapId": "wild_greenleaf"})
	a.withLock(func() {
		l := a.live[id]
		i := a.instances[l.InstanceKey]
		var mob M
		for _, m := range i.Monsters {
			if str(m["templateId"]) == "slime" {
				mob = m
				break
			}
		}
		if mob == nil {
			t.Fatal("no slime")
		}
		a.socketMessage(id, M{"type": "guard"})
		if num(l.State["guardUntil"]) <= now() {
			t.Fatal("guard not armed")
		}
		hp := num(mob["hp"])
		l.State["x"], l.State["y"], l.State["targetId"] = mob["x"], mob["y"], mob["id"]
		l.State["attackAt"] = 0
		l.State["guardUntil"] = now() + 8000
		a.tick()
		if num(mob["hp"]) != hp {
			t.Fatalf("guard still swung: %v -> %v", hp, mob["hp"])
		}
		if num(l.State["attackAt"]) < num(l.State["guardUntil"])-1 {
			t.Fatal("next swing was not delayed")
		}
		before := num(l.State["hp"])
		l.State["shield"] = 0
		l.State["invulnUntil"] = 0
		l.State["safe"] = false
		l.Events = []M{}
		a.hurtPlayer(i, mob, l, 1)
		lost := before - num(l.State["hp"])
		raw, _ := damage(num(a.def("monsters", "slime")["atk"]), 1, num(l.Stats["def"]), 0, num(obj(l.Stats["elemRes"])["physical"]), .05, 1.5, 1, false, func() float64 { return 1 })
		if lost <= 0 || lost >= raw {
			t.Fatalf("guard damage %v, unguarded non-crit %v", lost, raw)
		}
		marked := false
		for _, ev := range l.Events {
			if truth(ev["guarded"]) {
				marked = true
			}
		}
		if !marked {
			t.Fatal("damage event missing guarded")
		}
		snap := a.snapshot(l, now())
		self := obj(snap["self"])
		if num(self["guard"]) <= 0 || self["interval"] == nil {
			t.Fatalf("snapshot cadence: %v", self)
		}
	})
}
