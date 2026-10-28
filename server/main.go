// Go 服务入口与通信生命周期：单连接单写协程、广播、游戏循环、定期存档和进程关闭。

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// closeReplaced 告诉旧窗口：这个角色已经被新连接接管，不要立刻重连把对方挤掉。
const closeReplaced = 4001

// closeUnauthorized 表示令牌无效或还没有角色，客户端应停止重连。
const closeUnauthorized = 4003

// Each connection has one writer. A slow/disconnected client never blocks the game loop.
// Peer 一条 WebSocket 的连接、发送队列与退出信号；写消息只有一个协程。
type Peer struct {
	// 读消息在连接处理协程中进行，写消息只交给 write。
	conn *websocket.Conn
	// 不关闭 out，避免并发提交时向已关闭通道写入；拥堵连接直接关闭。
	out chan []byte
	// done 广播退出，once 保证多个关闭路径只执行一次。
	done      chan struct{}
	once      sync.Once
	closeCode int
	// writing 为真时由写协程负责关闭连接，避免和写消息同时 Close。
	writing atomic.Bool
}

// close 以正常离开的关闭码结束连接。
func (p *Peer) close() { p.closeWith(websocket.CloseGoingAway) }

// closeWith 只通知写协程退出。写协程还没启动时直接关闭连接，避免升级后的连接泄漏。
func (p *Peer) closeWith(code int) {
	p.once.Do(func() {
		if code != 0 {
			p.closeCode = code
		}
		close(p.done)
		if !p.writing.Load() {
			_ = p.conn.Close()
		}
	})
}

// enqueue 非阻塞地加入发送队列；队列拥堵时关闭慢连接，避免拖住游戏循环。
func (p *Peer) enqueue(b []byte) {
	select {
	case <-p.done:
		return
	default:
	}
	select {
	case p.out <- b:
	case <-p.done:
	default:
		p.close()
	}
}

// write 作为该连接唯一的写协程发送消息，并通过写超时和心跳处理断开的客户端。
func (p *Peer) write() {
	p.writing.Store(true)
	defer func() {
		code := p.closeCode
		if code == 0 {
			code = websocket.CloseNormalClosure
		}
		_ = p.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, ""), time.Now().Add(time.Second))
		_ = p.conn.Close()
	}()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-p.done:
			if p.closeCode == closeReplaced {
				_ = p.conn.SetWriteDeadline(time.Now().Add(time.Second))
				_ = p.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"replaced"}`))
			}
			return
		case <-ping.C:
			_ = p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if p.conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		case b := <-p.out:
			_ = p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if p.conn.WriteMessage(websocket.TextMessage, b) != nil {
				return
			}
		}
	}
}

// send 发送 JSON 消息；事务内先暂存，提交成功后才交给连接队列。
func (a *App) send(l *Live, message M) {
	if l == nil || l.Peer == nil {
		return
	}
	p := l.Peer
	// 在业务锁内先序列化，后续状态变化不会修改已排队的消息内容。
	b := []byte(js(message))
	fn := func() { p.enqueue(b) }
	if a.inTx {
		a.pending = append(a.pending, fn)
	} else {
		fn()
	}
}

// notify 发送 dirty 标记，让客户端重新读取背包、任务等完整档案。
func (a *App) notify(id string) { a.send(a.live[id], M{"type": "dirty"}) }

// toast 向在线角色发送中文提示消息。
func (a *App) toast(id, text string) { a.send(a.live[id], M{"type": "toast", "text": text}) }

// broadcast 向所有在线连接广播同一条世界消息。
func (a *App) broadcast(m M) {
	for _, l := range a.live {
		a.send(l, m)
	}
}

// withLock 串行访问共享世界状态，并记录异常，避免单次回调终止循环。
func (a *App) withLock(fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if e := recover(); e != nil {
			log.Printf("world error: %v", e)
		}
	}()
	fn()
}

// handleSocket 验证令牌、接管角色连接并读取意图；旧连接不能再操控已被新连接接管的角色。
func (a *App) handleSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	p := &Peer{conn: conn, out: make(chan []byte, 64), done: make(chan struct{})}
	go p.write()
	const wsIdle = 45 * time.Second
	conn.SetReadLimit(64 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(wsIdle))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsIdle))
	})
	id := ""
	ok := func() (ok bool) {
		a.mu.Lock()
		defer a.mu.Unlock()
		defer func() {
			if e := recover(); e != nil {
				// 写协程已经在跑，这里不再直接 WriteJSON，避免两个协程同时写同一条连接。
				if _, yes := e.(GameError); yes {
					p.closeWith(closeUnauthorized)
				} else {
					log.Printf("socket attach: %v", e)
					p.close()
				}
			}
		}()
		user := a.userID("Bearer " + r.URL.Query().Get("token"))
		id = str(a.requirePlayer(user)["id"])
		a.attach(id, p)
		return true
	}()
	if !ok {
		p.close()
		return
	}
	defer func() { p.close(); a.withLock(func() { a.detach(id, p) }) }()
	for {
		_, raw, e := conn.ReadMessage()
		if e != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(wsIdle))
		var m M
		if json.Unmarshal(raw, &m) != nil || m == nil {
			continue
		}
		a.withLock(func() {
			l := a.live[id]
			// 新连接接管角色后，旧连接尚在读循环中也不能继续提交动作。
			if l == nil || l.Peer != p {
				return
			}
			defer func() {
				if e := recover(); e != nil {
					text := "做不到"
					if ge, ok := e.(GameError); ok {
						text = ge.Message
					} else {
						log.Printf("socket command: %v", e)
					}
					a.toast(id, text)
				}
			}()
			a.transaction(func() { a.socketMessage(id, m) })
		})
	}
}

// main 加载服务配置，启动 HTTP、WebSocket、100ms 世界循环和定期存档，并处理关闭信号。
func main() {
	validate := flag.Bool("validate-config", false, "validate config/*.json and exit")
	flag.Parse()
	root := findRoot()
	if *validate {
		d := loadConfig(root)
		fmt.Printf("配置有效：地图 %d，怪物 %d，物品 %d，任务 %d\n", len(arr(d["maps"])), len(arr(d["monsters"])), len(arr(d["items"])), len(arr(d["quests"])))
		return
	}
	file := os.Getenv("FLOAT_DB")
	if file == "" {
		file = filepath.Join(root, "data", "float-island.sqlite")
	}
	a := newApp(root, file)
	defer a.db.Close()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8787"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", a.handleSocket)
	mux.HandleFunc("/", a.handleHTTP)
	srv := &http.Server{Addr: "0.0.0.0:" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	// 关闭信号先停模拟循环，再保存最终角色状态并关闭连接。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(100 * time.Millisecond)
		save := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		defer save.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				a.withLock(a.tick)
			case <-save.C:
				a.withLock(func() {
					for _, l := range a.live {
						a.save(l)
					}
				})
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("浮岛旅团 Go 服务已启动 http://127.0.0.1:%s；数据库 %s", port, file)
	err := srv.ListenAndServe()
	stop()
	<-done
	a.withLock(func() {
		for _, l := range a.live {
			a.save(l)
			if l.Peer != nil {
				l.Peer.close()
			}
		}
	})
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
