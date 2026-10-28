// REST 边界：请求体、鉴权、路由和错误响应；通过同一互斥锁协调 HTTP 与世界模拟。

package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// handleHTTP 处理跨域、请求体和统一错误响应；通过互斥锁串行访问世界状态。
func (a *App) handleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization, content-type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	var payload any
	status := 200
	func() {
		defer func() {
			if e := recover(); e != nil {
				if ge, ok := e.(GameError); ok {
					status = ge.Status
					payload = M{"error": ge.Message}
				} else {
					log.Printf("HTTP error: %v", e)
					status = 500
					payload = M{"error": "服务器出错了"}
				}
			}
		}()
		body := M{}
		if r.Method != "GET" && r.Method != "HEAD" {
			raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if e != nil {
				var tooBig *http.MaxBytesError
				if errors.As(e, &tooBig) {
					fail("请求太大")
				}
				fail("请求没有读完")
			}
			if len(raw) > 0 {
				if json.Unmarshal(raw, &body) != nil || body == nil {
					fail("请求格式不对")
				}
			}
		}
		// 登录哈希很慢，放在世界锁外面，避免一次注册把战斗循环卡住。
		if result, ok := a.handleAuth(r, body); ok {
			payload = result
			return
		}
		// 世界循环、REST 与 WebSocket 共用业务锁，避免同时修改余额、目标或位置。
		a.mu.Lock()
		defer a.mu.Unlock()
		if r.Method == "POST" {
			a.transaction(func() { payload = a.route(r, body) })
		} else {
			payload = a.route(r, body)
		}
	}()
	if payload == nil {
		payload = M{"ok": true}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// route 按方法与路径分发 REST 请求，公开接口之后要求登录与角色归属校验。
func (a *App) route(r *http.Request, b M) any {
	path, method := r.URL.Path, r.Method
	is := func(m, p string) bool { return method == m && path == p }
	s := func(k string) string { return trimmed(b, k) }
	n := func(k string, d float64) float64 { return math.Floor(number(b[k], d)) }
	if is("GET", "/api/health") {
		return M{"ok": true, "version": "0.1.2", "language": "go", "devTools": a.dev, "weather": a.weather["id"], "period": a.period}
	}
	if is("GET", "/api/config") {
		return a.publicConfig()
	}
	if is("GET", "/api/weapons") {
		return M{"weapons": weaponList(), "races": raceList()}
	}
	// 上面的健康、配置接口公开。登录在 handleAuth 里处理，从这里开始要求已验证的账号。
	user := a.userID(r.Header.Get("Authorization"))
	if is("POST", "/api/characters") {
		a.createCharacter(user, s("name"), s("weaponType"), s("race"))
		return a.profile(str(a.requirePlayer(user)["id"]))
	}
	id := str(a.requirePlayer(user)["id"])
	if is("GET", "/api/profile") {
		return a.profile(id)
	}
	if method == "GET" {
		switch path {
		case "/api/market":
			return M{"orders": a.market()}
		case "/api/guilds":
			return M{"guilds": a.guilds()}
		case "/api/mail":
			return M{"mail": a.listMail(id)}
		case "/api/friends":
			return M{"friends": a.friends(id)}
		case "/api/chat":
			return M{"messages": a.chatHistory(id, fallback(r.URL.Query().Get("channel"), "world"))}
		case "/api/party":
			return M{"party": a.partyView(id)}
		case "/api/news":
			return M{"news": a.news()}
		case "/api/leaderboard":
			return M{"rows": a.rows("SELECT name,level,exp FROM players ORDER BY level DESC,exp DESC LIMIT 10")}
		}
	}
	if method == "POST" {
		switch path {
		case "/api/equip":
			a.equip(id, s("itemId"))
		case "/api/unequip":
			a.unequip(id, s("slot"))
		case "/api/use":
			a.useItem(id, s("itemId"))
		case "/api/storage":
			a.storage(id, s("itemId"), s("direction"), n("count", 1))
		case "/api/skills":
			a.setLoadout(id, b["ids"])
		case "/api/attributes":
			a.allocate(id, b, false)
		case "/api/attributes/reset":
			a.allocate(id, b, true)
		case "/api/pets/active":
			a.activePet(id, s("petId"))
		case "/api/dispatches":
			a.startDispatch(id, s("petId"), s("nodeId"), s("durationId"))
		case "/api/dispatches/claim":
			a.claimDispatch(id, s("id"))
		case "/api/dispatches/finish":
			a.finishDispatch(id, s("id"))
		case "/api/craft":
			a.craft(id, s("recipeId"))
		case "/api/shop/buy":
			a.buyShop(id, s("npcId"), s("itemId"), n("count", 1))
		case "/api/shop/sell":
			a.sellShop(id, s("itemId"), n("count", 1))
		case "/api/market/list":
			a.listOrder(id, s("itemId"), n("count", 1), n("price", 0))
		case "/api/market/buy":
			a.buyOrder(id, s("id"))
		case "/api/market/cancel":
			a.cancelOrder(id, s("id"))
		case "/api/guilds":
			a.createGuild(id, s("name"))
		case "/api/guilds/join":
			a.joinGuild(id, s("guildId"))
		case "/api/guilds/leave":
			a.leaveGuild(id)
		case "/api/guilds/donate":
			a.donateGuild(id, n("gold", 0))
		case "/api/mail/claim":
			a.claimMail(id, s("id"))
		case "/api/friends/request":
			a.requestFriend(id, s("name"))
			return M{"ok": true}
		case "/api/friends/accept":
			a.acceptFriend(id, s("id"))
			return M{"friends": a.friends(id)}
		case "/api/chat":
			return a.postChat(id, s("channel"), s("text"))
		case "/api/party/invite":
			a.inviteParty(id, s("name"))
			return M{"party": a.partyView(id)}
		case "/api/party/join":
			a.joinParty(id)
			return M{"party": a.partyView(id)}
		case "/api/party/leave":
			a.leaveParty(id)
			return M{"party": nil}
		case "/api/travel":
			a.travel(id, s("mapId"))
		case "/api/dialog":
			action, npc := s("action"), s("npcId")
			a.nearNPC(id, npc)
			if strings.HasPrefix(action, "accept:") {
				qid := strings.TrimPrefix(action, "accept:")
				check(str(a.def("quests", qid)["giver"]) == npc, "任务不在这里")
				a.acceptQuest(id, qid, false)
			} else if strings.HasPrefix(action, "turnin:") {
				qid := strings.TrimPrefix(action, "turnin:")
				check(str(a.def("quests", qid)["turnin"]) == npc, "任务不在这里")
				a.turnIn(id, qid)
			} else if strings.HasPrefix(action, "open:") {
				panel := strings.TrimPrefix(action, "open:")
				check(contains(a.def("npcs", npc)["panels"], panel), "没有这个功能")
				return M{"open": panel, "profile": a.profile(id)}
			}
			d := a.dialog(id, npc)
			return M{"profile": a.profile(id), "dialog": d}
		default:
			panic(GameError{"没有这个接口", 404})
		}
		return a.profile(id)
	}
	panic(GameError{"没有这个接口", 404})
}

// clientIP 默认用连接地址。只有显式信任代理时才读取 X-Forwarded-For，避免客户端自己伪造来源。
func clientIP(r *http.Request) string {
	if os.Getenv("FLOAT_TRUST_PROXY") == "1" {
		if fwd := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); fwd != "" {
			return fwd
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}
	return host
}

// throttleAuth 按 IP 限制登录、注册和游客创建。本机限额更宽，方便开发和测试脚本。
func (a *App) throttleAuth(ip string) {
	limit := 30
	if ip == "127.0.0.1" || ip == "::1" {
		limit = 300
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	if a.authHits == nil {
		a.authHits = map[string][]time.Time{}
	}
	nowT := time.Now()
	cut := nowT.Add(-time.Minute)
	kept := make([]time.Time, 0, len(a.authHits[ip])+1)
	for _, ts := range a.authHits[ip] {
		if ts.After(cut) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= limit {
		a.authHits[ip] = kept
		panic(GameError{"试得太频繁了", 429})
	}
	a.authHits[ip] = append(kept, nowT)
}

// handleAuth 处理注册、登录和游客。返回 true 表示请求已经在这里结束。
func (a *App) handleAuth(r *http.Request, b M) (any, bool) {
	if r.Method != "POST" {
		return nil, false
	}
	switch r.URL.Path {
	case "/api/auth/register":
		email, password := normalizeAccount(str(b["email"]), str(b["password"]))
		a.throttleAuth(clientIP(r))
		hash, e := bcrypt.GenerateFromPassword([]byte(password), 10)
		must(e)
		a.mu.Lock()
		defer a.mu.Unlock()
		var out M
		a.transaction(func() { out = a.registerAccount(email, string(hash)) })
		return out, true
	case "/api/auth/login":
		email := strings.TrimSpace(str(b["email"]))
		password := strings.TrimSpace(str(b["password"]))
		a.throttleAuth(clientIP(r))
		a.mu.Lock()
		user := a.lookupUser(email)
		a.mu.Unlock()
		if !passwordMatches(user, password) {
			panic(GameError{"邮箱或密码不对", 401})
		}
		return M{"token": a.token(str(user["id"]))}, true
	case "/api/auth/guest":
		a.throttleAuth(clientIP(r))
		id := uid()
		hash, e := bcrypt.GenerateFromPassword([]byte(uid()), 4)
		must(e)
		a.mu.Lock()
		defer a.mu.Unlock()
		a.transaction(func() { a.createGuest(id, string(hash)) })
		return M{"token": a.token(id)}, true
	default:
		return nil, false
	}
}
