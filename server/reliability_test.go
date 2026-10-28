package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestProductionRejectsDefaultSecret(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("JWT_SECRET", "")
	defer func() {
		if recover() == nil {
			t.Fatal("empty production secret accepted")
		}
	}()
	loadSecret()
}

func TestProductionRejectsShortSecret(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("JWT_SECRET", "too-short")
	defer func() {
		if recover() == nil {
			t.Fatal("short production secret accepted")
		}
	}()
	loadSecret()
}

func TestDevSecretFallback(t *testing.T) {
	t.Setenv("NODE_ENV", "development")
	t.Setenv("JWT_SECRET", "  ")
	if loadSecret() != devJWTSecret {
		t.Fatal("dev secret fallback")
	}
}

func TestSQLitePragmas(t *testing.T) {
	a := testApp(t)
	if str(a.one("PRAGMA journal_mode")["journal_mode"]) != "wal" {
		t.Fatal("journal mode")
	}
	if num(a.one("PRAGMA foreign_keys")["foreign_keys"]) != 1 {
		t.Fatal("foreign keys")
	}
	if num(a.one("PRAGMA synchronous")["synchronous"]) != 1 {
		t.Fatal("synchronous")
	}
}

func TestAuthThrottle(t *testing.T) {
	a := testApp(t)
	for i := 0; i < 30; i++ {
		a.throttleAuth("192.0.2.1")
	}
	if _, code := request(t, a, "/api/auth/login", "", M{"email": "nobody@example.com", "password": "wrongpass"}); code != 429 {
		t.Fatalf("throttle status %d", code)
	}
}

func TestGoldCap(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	before := num(a.player(id)["gold"])
	expectFailure(t, func() { a.addGold(id, 2_000_000_000) })
	if num(a.player(id)["gold"]) != before {
		t.Fatal("gold cap leaked")
	}
}

func TestGuestNamesAreUnique(t *testing.T) {
	a := testApp(t)
	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		_, id := newGuest(t, a)
		name := str(a.player(id)["name"])
		if seen[name] || !validName(name, 2, 12) {
			t.Fatal(name)
		}
		seen[name] = true
	}
}

func TestTransactionRollsBackLiveMembership(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	before := num(a.live[id].State["gold"])
	expectFailure(t, func() {
		a.transaction(func() {
			a.live["ghost"] = &Live{State: M{"gold": 1}, Stats: M{}, CD: M{}}
			a.addGold(id, 5)
			delete(a.live, id)
			fail("回滚")
		})
	})
	if _, ok := a.live["ghost"]; ok {
		t.Fatal("ghost live player leaked")
	}
	l := a.live[id]
	if l == nil || num(l.State["gold"]) != before || num(a.player(id)["gold"]) != before {
		t.Fatalf("live state not restored gold=%v db=%v", l, num(a.player(id)["gold"]))
	}
}

func TestIdlePublicLineIsRemoved(t *testing.T) {
	a := testApp(t)
	_, id := newGuest(t, a)
	a.attach(id, nil)
	key := a.live[id].InstanceKey
	line := a.instances[key]
	if line == nil || line.Dungeon {
		t.Fatal("expected a public line")
	}
	delete(line.Players, id)
	line.EmptySince = now() - 120000
	a.tick()
	if _, ok := a.instances[key]; ok {
		t.Fatal("idle line still simulating")
	}
}

func TestSecondConnectionReplacesFirst(t *testing.T) {
	a := testApp(t)
	token, _ := newGuest(t, a)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", a.handleSocket)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := strings.Replace(srv.URL, "http", "ws", 1) + "/ws?token=" + token
	first, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	_ = first.SetReadDeadline(time.Now().Add(5 * time.Second))
	var world M
	if first.ReadJSON(&world) != nil || str(world["type"]) != "world" {
		t.Fatalf("world: %v", world)
	}
	second, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var replaced M
	if first.ReadJSON(&replaced) != nil || str(replaced["type"]) != "replaced" {
		t.Fatalf("replaced: %v", replaced)
	}
	_ = second.SetReadDeadline(time.Now().Add(5 * time.Second))
	var again M
	if second.ReadJSON(&again) != nil || str(again["type"]) != "world" {
		t.Fatalf("second world: %v", again)
	}
}
