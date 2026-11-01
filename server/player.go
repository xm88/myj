// 账号与角色：兼容 bcrypt 和 HS256 JWT，管理角色创建、潜力点、成长、金币和在线存档。

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"math"
	"math/rand/v2"
	"regexp"
	"strings"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// dummyHash 让不存在的邮箱也走一次 bcrypt，避免用响应时间判断邮箱是否注册。
var dummyHash []byte

func init() {
	hash, e := bcrypt.GenerateFromPassword([]byte("float-island-dummy-password"), 10)
	if e != nil {
		panic(e)
	}
	dummyHash = hash
}

// token 签发带账号 ID 和十四天有效期的 HS256 JWT，兼容旧客户端登录格式。
func (a *App) token(id string) string {
	enc := base64.RawURLEncoding.EncodeToString
	head := enc([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := enc([]byte(js(M{"uid": id, "iat": math.Floor(now() / 1000), "exp": math.Floor(now()/1000) + 14*86400})))
	raw := head + "." + payload
	h := hmac.New(sha256.New, []byte(a.secret))
	h.Write([]byte(raw))
	return raw + "." + enc(h.Sum(nil))
}

// userID 验证 Bearer 令牌的算法、签名和有效期，成功后返回账号 ID。
func (a *App) userID(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		panic(GameError{"需要登录", 401})
	}
	p := strings.Split(strings.TrimPrefix(header, "Bearer "), ".")
	if len(p) != 3 {
		panic(GameError{"登录已过期", 401})
	}
	dec := base64.RawURLEncoding.DecodeString
	sig, e := dec(p[2])
	h := hmac.New(sha256.New, []byte(a.secret))
	h.Write([]byte(p[0] + "." + p[1]))
	raw, e2 := dec(p[1])
	head, e3 := dec(p[0])
	m := obj(parse(string(raw)))
	// 不仅检查签名，还固定 HS256 算法并检查到期时间，避免接受错误类型令牌。
	if e != nil || e2 != nil || e3 != nil || str(obj(parse(string(head)))["alg"]) != "HS256" || !hmac.Equal(sig, h.Sum(nil)) || num(m["exp"]) <= now()/1000 || str(m["uid"]) == "" {
		panic(GameError{"登录已过期", 401})
	}
	return str(m["uid"])
}

// normalizeAccount 校验注册用的邮箱和密码。密码沿用原先会去掉首尾空白的行为。
func normalizeAccount(email, password string) (string, string) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	check(emailPattern.MatchString(email), "邮箱格式不对")
	check(len([]rune(password)) >= 6 && len([]rune(password)) <= 64 && len(password) <= 72, "密码需要 6 到 64 位，UTF-8 编码不超过 72 字节")
	return email, password
}

// registerAccount 在已持有世界锁时写入账号。密码摘要由调用方事先算好。
func (a *App) registerAccount(email, hash string) M {
	check(a.one("SELECT id FROM users WHERE email=?", email) == nil, "这个邮箱已经注册过了")
	id := uid()
	a.run("INSERT INTO users(id,email,password_hash,is_guest,created_at) VALUES(?,?,?,0,?)", id, email, hash, now())
	return M{"token": a.token(id)}
}

// lookupUser 只读取账号行，供锁外的密码校验使用。
func (a *App) lookupUser(email string) M {
	return a.one("SELECT * FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(email)))
}

// passwordMatches 比较 bcrypt 摘要。账号不存在时仍比较固定摘要，避免邮箱探测。
func passwordMatches(user M, password string) bool {
	if len(password) == 0 || len(password) > 72 {
		return false
	}
	hash := dummyHash
	if user != nil {
		hash = []byte(str(user["password_hash"]))
	}
	return user != nil && bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

// uniqueGuestName 在有限次数内挑一个没被占用的旅人名字，避免碰撞后死循环占住世界锁。
func (a *App) uniqueGuestName() string {
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("旅人%d", 1000+rand.IntN(9000))
		if a.one("SELECT id FROM players WHERE name=?", name) == nil {
			return name
		}
	}
	for i := 0; i < 8; i++ {
		name := "旅人" + uid()[:8]
		if a.one("SELECT id FROM players WHERE name=?", name) == nil {
			return name
		}
	}
	fail("暂时发不出旅人名字")
	return ""
}

// createGuest 写入游客账号和初始角色。调用方负责事务，以及在拿锁之前生成密码摘要。
func (a *App) createGuest(id, hash string) {
	a.run("INSERT INTO users(id,email,password_hash,is_guest,created_at) VALUES(?,?,?,1,?)", id, "guest_"+id+"@guest.local", hash, now())
	a.createCharacter(id, a.uniqueGuestName(), "sword_shield", "human")
}

// player 按角色 ID 读取持久化记录，角色不存在时返回业务错误。
func (a *App) player(id string) M {
	r := a.one("SELECT * FROM players WHERE id=?", id)
	if r == nil {
		panic(GameError{"角色不存在", 404})
	}
	return r
}

// requirePlayer 读取账号唯一的角色，未创建角色时返回明确的缺失错误。
func (a *App) requirePlayer(user string) M {
	r := a.one("SELECT * FROM players WHERE user_id=?", user)
	if r == nil {
		panic(GameError{"还没有角色", 404})
	}
	return r
}

// defaultLoadout 选择当前武器前四个技能，作为创建角色或换武器后的默认技能栏。
func (a *App) defaultLoadout(weapon string) []string {
	r := []string{}
	for _, s := range objects(a.data["skills"]) {
		if str(s["weapon"]) == weapon && len(r) < 4 {
			r = append(r, str(s["id"]))
		}
	}
	return r
}

// createCharacter 校验角色名与武器种族，创建初始存档、熟练、装备、道具和主线任务。
func (a *App) createCharacter(user, name, weapon, race string) {
	check(a.one("SELECT id FROM players WHERE user_id=?", user) == nil, "已经有角色了")
	check(weaponLabels[weapon] != nil, "这把武器还没开放")
	check(validName(name, 2, 12), "名字需要 2 到 12 个字，可用中文、字母和数字")
	check(a.one("SELECT id FROM players WHERE name=?", name) == nil, "已经有人用这个名字了")
	id := uid()
	t := now()
	town := a.def("maps", "town_starfall")
	spawn := obj(town["spawn"])
	a.run(`INSERT INTO players(id,user_id,name,level,exp,map_id,x,y,hp,mp,gold,weapon_type,skill_loadout,attributes,race,flags,buffs,created_at,updated_at) VALUES(?,?,?,1,0,?,?,?,?,?,?,?,?,'{}',?,'{}','[]',?,?)`, id, user, name, town["id"], spawn["x"], spawn["y"], 120, 40, a.constant("starterGold"), weapon, js(a.defaultLoadout(weapon)), raceID(race), t, t)
	// 新角色初始化当前武器与三条生活熟练，再发放装备、契约道具和第一条主线。
	for _, track := range []string{"weapon:" + weapon, "life:gather", "life:craft", "life:cook"} {
		a.run("INSERT INTO player_tracks(player_id,track,level,exp) VALUES(?,?,1,0)", id, track)
	}
	starter := map[string]string{"sword_shield": "wpn_sword_1", "greatsword": "wpn_great_1", "staff": "wpn_staff_1", "bow": "wpn_bow_1"}[weapon]
	a.addItem(id, starter, 1, a.equipmentExtra("white"), "equipped")
	a.addItem(id, "capture_orb", a.constant("starterOrbs"), nil, "bag")
	a.addItem(id, "bread", 2, nil, "bag")
	a.acceptQuest(id, "q_main_01", true)
	a.touchAlbum(id, nil)
}

// addGold 以存档金币为依据校验余额，写库后同步在线金币。
func (a *App) addGold(id string, delta float64) {
	check(!math.IsNaN(delta) && !math.IsInf(delta, 0), "金币不对")
	r := a.player(id)
	next := num(r["gold"]) + delta
	check(next >= 0, "金币不够")
	check(next <= maxGold, "金币已经够多了")
	a.run("UPDATE players SET gold=?,updated_at=? WHERE id=?", next, now(), id)
	if l := a.live[id]; l != nil {
		l.State["gold"] = next
	}
}

// addExp 累计经验并逐级升级到等级上限，升级后刷新面板、恢复状态并推进等级任务。
func (a *App) addExp(id string, amount float64) {
	if amount <= 0 {
		return
	}
	r := a.player(id)
	level, exp := num(r["level"]), num(r["exp"])+amount
	ups := 0
	// 一次奖励可能跨越多级，逐级扣经验以保留正确的剩余经验。
	for level < 30 && exp >= expNext(level) {
		exp -= expNext(level)
		level++
		ups++
	}
	a.run("UPDATE players SET level=?,exp=?,updated_at=? WHERE id=?", level, exp, now(), id)
	if l := a.live[id]; l != nil {
		l.State["level"] = level
		l.State["exp"] = exp
	}
	if ups > 0 {
		a.refresh(id)
		if l := a.live[id]; l != nil {
			l.State["hp"] = l.Stats["maxHp"]
			l.State["mp"] = l.Stats["maxMp"]
		}
		a.emit(id, M{"type": "level", "level": level})
		a.toast(id, fmt.Sprintf("升级了，现在是 %g 级，获得 %d 点属性", level, ups*5))
		a.touchAlbum(id, nil)
		a.notify(id)
	}
}

// track 读取武器或生活技能熟练，缺失记录视为一级零经验。
func (a *App) track(id, track string) M {
	r := a.one("SELECT level,exp FROM player_tracks WHERE player_id=? AND track=?", id, track)
	if r == nil {
		return M{"level": 1, "exp": 0}
	}
	return r
}

// trackExp 按对应经验曲线升级熟练，并将等级与剩余经验写回存档。
func (a *App) trackExp(id, track string, amount float64, weapon bool) {
	r := a.track(id, track)
	level, exp := num(r["level"]), num(r["exp"])+amount
	cap := 10.
	if weapon {
		cap = 20
	}
	for level < cap && exp >= trackNext(level, weapon) {
		exp -= trackNext(level, weapon)
		level++
	}
	a.run("INSERT INTO player_tracks(player_id,track,level,exp) VALUES(?,?,?,?) ON CONFLICT(player_id,track) DO UPDATE SET level=excluded.level,exp=excluded.exp", id, track, level, exp)
}

// allocate 分配或重置潜力点，分配总量不能超过等级提供的预算。
func (a *App) allocate(id string, body M, reset bool) {
	r := a.player(id)
	attrs := parseAttrs(str(r["attributes"]))
	if reset {
		check(spentAttrs(attrs) > 0, "还没有加过点")
		attrs = emptyAttrs()
	} else {
		added := 0.
		for _, k := range attrKeys {
			n := math.Floor(num(body[k]))
			check(n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0), "点数不对")
			attrs[k] = num(attrs[k]) + n
			added += n
		}
		check(added > 0, "没有要加的点")
		check(spentAttrs(attrs) <= num(r["level"])*5, "点数不够")
	}
	a.run("UPDATE players SET attributes=?,updated_at=? WHERE id=?", js(attrs), now(), id)
	a.refresh(id)
	a.notify(id)
}

// setFlag 持久化角色解锁标记，用于隐藏任务和探索内容的前置条件。
func (a *App) setFlag(id, key string) {
	r := a.player(id)
	flags := obj(parse(str(r["flags"])))
	flags[key] = true
	a.run("UPDATE players SET flags=?,updated_at=? WHERE id=?", js(flags), now(), id)
}

// save 将在线等级、坐标、血量、金币、技能栏和增益写回角色存档。
func (a *App) save(l *Live) {
	s := l.State
	a.run(`UPDATE players SET level=?,exp=?,map_id=?,x=?,y=?,hp=?,mp=?,gold=?,weapon_type=?,skill_loadout=?,buffs=?,updated_at=? WHERE id=?`, s["level"], s["exp"], s["mapId"], s["x"], s["y"], s["hp"], s["mp"], s["gold"], s["weaponType"], js(s["loadout"]), js(s["buffs"]), now(), s["id"])
}
