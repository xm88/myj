// 基础设施：JSON 兼容层、持久化查询、事务回滚、配置校验与应用初始化。

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// devJWTSecret 只用于本机开发。生产环境必须用 JWT_SECRET 覆盖，仓库里不放真实密钥。
const devJWTSecret = "dev-float-island-change-me"

// gameVersion 是这一版对外的切片号。健康检查和公开配置都读它。
const gameVersion = "0.1.9"

// maxGold 防止异常金额把 float64 金币打成 Inf 后写进存档。
const maxGold = 1000000000

var secretWarn sync.Once

// M is a JSON object used for configuration and the existing wire protocol.
// Runtime world state is confined to App.mu; database mutations and ticks are serialized.
// M 用于配置、存档和协议的动态 JSON 对象，字段名称保持客户端兼容。
type M map[string]any

// obj 把动态 JSON 值转换成对象；非对象值按空对象处理。
func obj(v any) M {
	switch x := v.(type) {
	case M:
		return x
	case map[string]any:
		return M(x)
	}
	return M{}
}

// num 统一读取数据库整数、JSON 数字和数值字符串；无法转换时返回零。
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, _ := x.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(x, 64)
		return n
	}
	return 0
}

// str 读取字符串值，其他类型返回空字符串。
func str(v any) string { s, _ := v.(string); return s }

// arr 将支持的切片类型统一为 JSON 数组；缺失值返回空数组。
func arr(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []M:
		r := make([]any, len(x))
		for i, v := range x {
			r[i] = v
		}
		return r
	case []string:
		r := make([]any, len(x))
		for i, v := range x {
			r[i] = v
		}
		return r
	}
	return []any{}
}

// objects 将配置或查询结果的数组转换为对象列表。
func objects(v any) []M {
	r := []M{}
	for _, v := range arr(v) {
		r = append(r, obj(v))
	}
	return r
}

// truth 按协议中的布尔、数字和字符串值判断开关是否开启。
func truth(v any) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != ""
	default:
		return num(v) != 0
	}
}

// parse 解析存档中的 JSON；无效文本返回空值，由调用方提供默认值。
func parse(s string) any { var v any; _ = json.Unmarshal([]byte(s), &v); return v }

// js 序列化协议或存档数据，序列化失败时交给统一错误处理。
func js(v any) string { b, e := json.Marshal(v); must(e); return string(b) }

// clone 深拷贝 JSON 对象，用于保存事务前的内存状态。
func clone(m M) M { return obj(parse(js(m))) }

// now 返回 Unix 毫秒时间戳；数据库、冷却和派遣共用这一单位。
func now() float64 { return float64(time.Now().UnixMilli()) }

// uid 生成随机标识，用于账号、角色、物品和其他实例。
func uid() string { b := make([]byte, 16); _, e := rand.Read(b); must(e); return hex.EncodeToString(b) }

// clamp 将数值限制在给定区间，防止坐标、血量或输入越界。
func clamp(n, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, n)) }

// fallback 字符串缺失或为空时使用调用方给定的显示值。
func fallback(v any, s string) string {
	if str(v) != "" {
		return str(v)
	}
	return s
}

// number 读取可选数字，字段缺失时使用默认值。
func number(v any, d float64) float64 {
	if v == nil {
		return d
	}
	return num(v)
}

// contains 判断字符串是否出现在 JSON 数组中。
func contains(a any, s string) bool {
	for _, v := range arr(a) {
		if str(v) == s {
			return true
		}
	}
	return false
}

// merge 把第二个对象的字段写入第一个对象，并返回第一个对象。
func merge(a, b M) M {
	for k, v := range b {
		a[k] = v
	}
	return a
}

// GameError 可安全展示给玩家的业务错误，HTTP 状态码与中文提示同时保留。
type GameError struct {
	Message string
	Status  int
}

// Error 返回可展示给客户端的业务错误文本。
func (e GameError) Error() string { return e.Message }

// fail 终止当前业务操作，由 HTTP 或 WebSocket 边界转换为错误响应。
func fail(s string) { panic(GameError{s, 400}) }

// check 校验业务前置条件，失败时携带中文原因终止操作。
func check(ok bool, s string) {
	if !ok {
		fail(s)
	}
}

// must 向上传递数据库、文件或序列化错误，交给调用边界处理。
func must(e error) {
	if e != nil {
		panic(e)
	}
}

// queryDB 数据库连接与事务共用的查询接口，让业务函数无需区分当前是否在事务内。
type queryDB interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
}

// App 单进程游戏服共享状态；数据库操作和在线世界修改由互斥锁串行保护。
type App struct {
	// mu 统一保护 HTTP、WebSocket 命令和定时循环访问的运行时状态。
	mu sync.Mutex
	// db 是长期数据库连接；q 在事务中临时指向 sql.Tx。
	db *sql.DB
	q  queryDB
	// data 保留完整配置，index 是按类别和模板 ID 建立的只读查找索引。
	data  M
	index map[string]map[string]M
	// live 和 instances 保存在线角色与地图实例，坐标定期或断线时写库。
	live      map[string]*Live
	instances map[string]*Instance
	// parties 与 playerParty 分别按队伍和角色查找关系；invites 保存限时邀请。
	parties     map[string]M
	playerParty map[string]string
	invites     map[string]M
	// weather 包含天气 id 与截止时间；period 使用东八区真实时段。
	weather M
	period  string
	// dev 控制立即完成派遣等测试入口；secret 与已有 JWT 签名配置保持一致。
	dev    bool
	secret string
	// pending 暂存事务内通知，inTx 让嵌套业务复用当前事务。
	pending []func()
	inTx    bool
	// authHits / authMu 只限制登录类请求，不占用世界锁。
	authHits map[string][]time.Time
	authMu   sync.Mutex
	// bouts 是进行中的回合战斗。boutBy 用角色或怪物 ID 找回那一场。
	bouts  map[string]*Bout
	boutBy map[string]string
	// trades 是当面交易。tradeBy 用角色 ID 找回窗口。
	trades  map[string]*Trade
	tradeBy map[string]string
	// origins 是各地图在连续世界上的原点，第一次用到时按传送门对齐。
	origins map[string][2]float64
}

// rows 执行参数化查询，将列名与列值组成对象列表；始终关闭查询游标。
func (a *App) rows(query string, args ...any) []M {
	rows, e := a.q.Query(query, args...)
	must(e)
	defer rows.Close()
	cols, e := rows.Columns()
	must(e)
	out := []M{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		must(rows.Scan(ptrs...))
		m := M{}
		for i, k := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			m[k] = v
		}
		out = append(out, m)
	}
	must(rows.Err())
	return out
}

// one 查询第一条记录，查不到时返回空值。
func (a *App) one(query string, args ...any) M {
	r := a.rows(query, args...)
	if len(r) == 0 {
		return nil
	}
	return r[0]
}

// run 执行写入并返回影响行数，用于确认订单和奖励状态只变更一次。
func (a *App) run(query string, args ...any) int64 {
	r, e := a.q.Exec(query, args...)
	must(e)
	n, e := r.RowsAffected()
	must(e)
	return n
}

// transaction 将数据库写入作为一个整体提交；失败时回滚数据库与内存状态，成功后发送暂存通知。
func (a *App) transaction(fn func()) {
	// 内层业务沿用外层事务，避免 SQLite 出现嵌套 BEGIN。
	if a.inTx {
		fn()
		return
	}
	tx, e := a.db.Begin()
	must(e)
	a.q = tx
	a.inTx = true
	a.pending = nil
	// 数据库回滚不能自动撤销内存修改，因此同时保存角色、分线和队伍状态。
	type liveSave struct {
		live             *Live
		state, stats, cd M
		events           []M
		key              string
	}
	saved := map[string]liveSave{}
	for id, l := range a.live {
		saved[id] = liveSave{l, clone(l.State), clone(l.Stats), clone(l.CD), objects(parse(js(l.Events))), l.InstanceKey}
	}
	savedInstances := map[string]*Instance{}
	for key, i := range a.instances {
		b, e := json.Marshal(i)
		must(e)
		var cp Instance
		must(json.Unmarshal(b, &cp))
		savedInstances[key] = &cp
	}
	savedParties := map[string]M{}
	for k, v := range a.parties {
		savedParties[k] = clone(v)
	}
	savedPlayerParty := map[string]string{}
	for k, v := range a.playerParty {
		savedPlayerParty[k] = v
	}
	savedInvites := map[string]M{}
	for k, v := range a.invites {
		savedInvites[k] = clone(v)
	}
	savedWeather := clone(a.weather)
	savedPeriod := a.period
	savedBouts := map[string]*Bout{}
	for k, v := range a.bouts {
		savedBouts[k] = cloneBout(v)
	}
	savedBoutBy := map[string]string{}
	for k, v := range a.boutBy {
		savedBoutBy[k] = v
	}
	savedTrades := map[string]*Trade{}
	for k, v := range a.trades {
		savedTrades[k] = cloneTrade(v)
	}
	savedTradeBy := map[string]string{}
	for k, v := range a.tradeBy {
		savedTradeBy[k] = v
	}
	committed := false
	defer func() {
		// 提交成功后的通知失败不能再回滚已经落地的存档。
		if committed {
			return
		}
		a.q = a.db
		a.inTx = false
		a.pending = nil
		if r := recover(); r != nil {
			_ = tx.Rollback()
			for id := range a.live {
				if _, ok := saved[id]; !ok {
					delete(a.live, id)
				}
			}
			for id, m := range saved {
				l := m.live
				l.State = m.state
				l.Stats = m.stats
				l.CD = m.cd
				l.Events = m.events
				l.InstanceKey = m.key
				a.live[id] = l
			}
			a.instances = savedInstances
			a.parties = savedParties
			a.playerParty = savedPlayerParty
			a.invites = savedInvites
			a.weather = savedWeather
			a.period = savedPeriod
			a.bouts = savedBouts
			a.boutBy = savedBoutBy
			a.trades = savedTrades
			a.tradeBy = savedTradeBy
			panic(r)
		}
	}()
	fn()
	// 只有持久化提交成功才发送 dirty、提示与其他暂存消息。
	must(tx.Commit())
	committed = true
	a.q = a.db
	a.inTx = false
	pending := a.pending
	a.pending = nil
	for _, f := range pending {
		f()
	}
}

// def 按配置类别和 ID 查找模板，缺失模板返回空值。
func (a *App) def(kind, id string) M { return a.index[kind][id] }

// constant 读取世界配置中的全局规则数值。
func (a *App) constant(k string) float64 { return num(obj(a.data["constants"])[k]) }

// loadConfig 读取配置目录并校验引用，避免缺失模板进入运行时。
func loadConfig(root string) M {
	data := M{}
	for _, kind := range []string{"items", "skills", "pets", "monsters", "maps", "npcs", "quests", "recipes"} {
		b, e := os.ReadFile(filepath.Join(root, "config", kind+".json"))
		must(e)
		var v []any
		must(json.Unmarshal(b, &v))
		data[kind] = v
	}
	b, e := os.ReadFile(filepath.Join(root, "config", "world.json"))
	must(e)
	var world M
	must(json.Unmarshal(b, &world))
	merge(data, world)
	validateConfig(data)
	return data
}

// validateConfig 检查重复 ID 和跨表引用，错误配置会阻止服务启动。
func validateConfig(data M) {
	kinds := []string{"items", "skills", "pets", "monsters", "maps", "npcs", "quests", "recipes", "dispatchNodes", "durations", "weathers"}
	idx := map[string]map[string]bool{}
	for _, kind := range kinds {
		idx[kind] = map[string]bool{}
		for _, d := range objects(data[kind]) {
			id := str(d["id"])
			if id == "" || idx[kind][id] {
				panic(fmt.Errorf("invalid/duplicate %s ID: %s", kind, id))
			}
			idx[kind][id] = true
		}
	}
	ref := func(kind string, v any) {
		if str(v) != "" && !idx[kind][str(v)] {
			panic(fmt.Errorf("unknown %s reference: %s", kind, str(v)))
		}
	}
	for _, m := range objects(data["maps"]) {
		for _, s := range objects(m["spawns"]) {
			ref("monsters", s["monsterId"])
		}
		for _, s := range objects(m["rares"]) {
			ref("monsters", s["monsterId"])
		}
		for _, p := range objects(m["portals"]) {
			ref("maps", p["to"])
		}
		for _, n := range objects(m["nodes"]) {
			ref("items", n["itemId"])
		}
	}
	for _, m := range objects(data["monsters"]) {
		ref("pets", m["petId"])
		for _, d := range objects(m["drops"]) {
			ref("items", d["itemId"])
		}
	}
	for _, p := range objects(data["pets"]) {
		ref("skills", p["skillId"])
	}
	for _, n := range objects(data["npcs"]) {
		ref("maps", n["mapId"])
		for _, g := range objects(n["shop"]) {
			ref("items", g["itemId"])
		}
	}
	for _, q := range objects(data["quests"]) {
		ref("npcs", q["giver"])
		ref("npcs", q["turnin"])
		ref("quests", q["next"])
		for _, pre := range arr(q["prereq"]) {
			ref("quests", pre)
		}
		r := obj(q["rewards"])
		ref("pets", r["petId"])
		for _, i := range objects(r["items"]) {
			ref("items", i["itemId"])
		}
	}
	for _, r := range objects(data["recipes"]) {
		ref("items", obj(r["output"])["itemId"])
		for _, i := range objects(r["inputs"]) {
			ref("items", i["itemId"])
		}
	}
	for _, n := range objects(data["dispatchNodes"]) {
		ref("maps", n["mapId"])
		for _, i := range objects(n["basePerHour"]) {
			ref("items", i["itemId"])
		}
	}
}

// loadSecret 读取 JWT 签名密钥。生产环境拒绝空值和开发默认值，避免公网沿用仓库里的占位符。
func loadSecret() string {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	prod := os.Getenv("NODE_ENV") == "production"
	if s == "" {
		if prod {
			panic("NODE_ENV=production 时必须设置 JWT_SECRET")
		}
		secretWarn.Do(func() {
			log.Printf("警告：未设置 JWT_SECRET，开发环境使用内置默认密钥。不要把这个默认值用于公网。")
		})
		return devJWTSecret
	}
	if prod && (s == devJWTSecret || len(s) < 16) {
		panic("NODE_ENV=production 时 JWT_SECRET 至少要 16 个字符，并且不能使用开发默认值")
	}
	return s
}

// pragma 用查询执行会返回结果的 PRAGMA，避免把建表语句和连接参数混在同一次 Exec 里。
func (a *App) pragma(query string) {
	rows, e := a.db.Query(query)
	must(e)
	defer rows.Close()
	for rows.Next() {
	}
	must(rows.Err())
}

// newApp 打开存档、补齐旧表字段、加载配置索引并恢复全服天气。
func newApp(root, file string) *App {
	must(os.MkdirAll(filepath.Dir(file), 0755))
	db, e := sql.Open("sqlite", file)
	must(e)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	must(db.Ping())
	a := &App{db: db, q: db, data: loadConfig(root), index: map[string]map[string]M{}, live: map[string]*Live{}, instances: map[string]*Instance{}, parties: map[string]M{}, playerParty: map[string]string{}, invites: map[string]M{}, period: dayPeriod(time.Now()), dev: os.Getenv("NODE_ENV") != "production", secret: loadSecret(), bouts: map[string]*Bout{}, boutBy: map[string]string{}, trades: map[string]*Trade{}, tradeBy: map[string]string{}}
	a.pragma("PRAGMA busy_timeout=5000")
	a.pragma("PRAGMA foreign_keys=ON")
	a.pragma("PRAGMA journal_mode=WAL")
	a.pragma("PRAGMA synchronous=NORMAL")
	a.run(schema)
	cols := a.rows("PRAGMA table_info(players)")
	for _, column := range []string{"attributes", "race"} {
		found := false
		for _, c := range cols {
			if str(c["name"]) == column {
				found = true
			}
		}
		if !found {
			def := "{}"
			if column == "race" {
				def = "human"
			}
			a.run("ALTER TABLE players ADD COLUMN " + column + " TEXT NOT NULL DEFAULT '" + def + "'")
		}
	}
	for k, v := range a.data {
		a.index[k] = map[string]M{}
		for _, d := range objects(v) {
			a.index[k][str(d["id"])] = d
		}
	}
	a.migratePetColumns()
	a.backfillPets()
	a.weather = M{"id": "clear", "until": now() + a.constant("weatherMs")}
	if r := a.one("SELECT value FROM world_state WHERE key='weather'"); r != nil {
		m := obj(parse(str(r["value"])))
		if a.def("weathers", str(m["id"])) != nil {
			a.weather = m
		}
	} else {
		a.run("INSERT INTO world_state(key,value) VALUES('weather',?)", js(a.weather))
	}
	return a
}

// findRoot 优先读取 FLOAT_ROOT，再从工作目录和程序位置查找配置目录。
func findRoot() string {
	if s := os.Getenv("FLOAT_ROOT"); s != "" {
		return s
	}
	cwd, e := os.Getwd()
	must(e)
	for p := cwd; ; p = filepath.Dir(p) {
		if _, e := os.Stat(filepath.Join(p, "config", "world.json")); e == nil {
			return p
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	exe, e := os.Executable()
	must(e)
	for p := filepath.Dir(exe); ; p = filepath.Dir(p) {
		if _, e := os.Stat(filepath.Join(p, "config", "world.json")); e == nil {
			return p
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	panic("config/world.json not found; set FLOAT_ROOT")
}

// validName 校验角色或公会名称的长度与允许字符，避免空名和不可显示字符。
func validName(s string, min, max int) bool {
	r := []rune(s)
	if len(r) < min || len(r) > max {
		return false
	}
	for _, c := range r {
		if !(c >= '一' && c <= '龥' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// dist 计算两个世界坐标之间的欧氏距离，用于攻击、互动和追击范围判断。
func dist(a, b M) float64 { return math.Hypot(num(a["x"])-num(b["x"]), num(a["y"])-num(b["y"])) }

// trimmed 读取并去掉请求字符串字段的首尾空白。
func trimmed(m M, k string) string { return strings.TrimSpace(str(m[k])) }
