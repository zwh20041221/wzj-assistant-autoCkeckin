package qrws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zwh20041221/wzj-assistant-autoCkeckin/internal/qr"
)

// Minimal Bayeux/Faye client tailored for Teachermate QR channel

// logging helpers with timestamp and debug gate
var debugEnabled bool

func SetDebug(b bool)                  { debugEnabled = b }
func ts() string                       { return time.Now().Format("2006-01-02 15:04:05.000") }
func infoln(args ...any)               { fmt.Println(append([]any{"[" + ts() + "]"}, args...)...) }
func infof(format string, args ...any) { fmt.Printf("[%s] ", ts()); fmt.Printf(format, args...) }
func dbgln(args ...any) {
	if debugEnabled {
		infoln(args...)
	}
}
func dbgf(format string, args ...any) {
	if debugEnabled {
		infof(format, args...)
	}
}

type Client struct {
	endpoint   string
	conn       *websocket.Conn
	mu         sync.Mutex
	clientID   string
	connected  bool
	seq        int
	subscribed string // courseId/signId key
	stopCh     chan struct{}
	// 学生结果通道：当服务端推送 type=3 时，向外部报告一次
	ResultCh      chan StudentResult
	handshakeDone chan struct{}
}

func New() *Client {
	return &Client{
		endpoint: "wss://www.teachermate.com.cn/faye",
		stopCh:   make(chan struct{}),
		ResultCh: make(chan StudentResult, 1),
	}
}

func (c *Client) nextSeq() string {
	c.seq++
	return fmt.Sprintf("%d", c.seq)
}

// Start establishes the connection and performs handshake + connect, keeping heartbeats.
// It does not subscribe to any course/sign yet (preconnect).
func (c *Client) Start() error {
	c.mu.Lock()
	if c.conn != nil {
		c.mu.Unlock()
		return nil
	}
	u, _ := url.Parse(c.endpoint)
	// 添加超时控制
	dialer := &websocket.Dialer{HandshakeTimeout: 10 * time.Second, Subprotocols: []string{"bayeux"}}
	// 设置 Origin 以匹配服务端期待的来源（部分 Faye 部署要求）
	hdr := http.Header{"Origin": []string{"https://www.teachermate.com.cn"}}
	conn, _, err := dialer.Dial(u.String(), hdr)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	c.conn = conn
	c.mu.Unlock()

	// reader loop
	go c.readLoop()
	// send handshake（注意：此处不能持锁，否则 send 内部加锁会导致死锁）
	// 重置握手完成信号
	c.mu.Lock()
	c.handshakeDone = make(chan struct{})
	c.mu.Unlock()
	c.send([]any{map[string]any{
		"channel":        "/meta/handshake",
		"version":        "1.0",
		"minimumVersion": "1.0",
		"supportedConnectionTypes": []string{
			"websocket", "eventsource", "long-polling", "cross-origin-long-polling", "callback-polling",
		},
		"id": c.nextSeq(),
	}})
	infoln("[WS] handshake sent")
	// 握手 5 秒超时提示（不阻塞流程）
	go func(ch <-chan struct{}) {
		select {
		case <-ch:
			dbgln("[WS] handshake success signal received")
		case <-time.After(5 * time.Second):
			infoln("[WS] handshake timeout: no response within 5s")
		case <-c.stopCh:
		}
	}(c.handshakeDone)
	return nil
}

func (c *Client) Close() {

	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) readLoop() {
	dbgln("[WS] readLoop started")
	defer dbgln("[WS] readLoop exit")
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			infoln("[WS] read error:", err)
			return
		}
		if len(data) > 0 {
			raw := data
			if len(raw) > 1024 {
				raw = raw[:1024]
			}
			dbgf("[WS][RAW %dB] %s\n", len(data), string(raw))
		}
		// parse array of messages
		var msgs []map[string]any
		if err := json.Unmarshal(data, &msgs); err != nil {
			dbgln("[WS] invalid JSON frame")
			continue
		}
		if len(msgs) == 0 {
			// heartbeat reply
			dbgln("[WS] heartbeat ack []")
			continue
		}
		dbgln("[WS] messages count:", len(msgs))
		for i, m := range msgs {
			ch, _ := m["channel"].(string)
			succ, _ := m["successful"].(bool)
			dbgf("[WS] msg[%d] channel=%s successful=%v\n", i, ch, succ)
			if succ {
				switch ch {
				case "/meta/handshake":
					if cid, ok := m["clientId"].(string); ok {
						c.clientID = cid
						// 发出握手完成信号
						c.mu.Lock()
						if c.handshakeDone != nil {
							select {
							case <-c.handshakeDone:
							default:
								close(c.handshakeDone)
							}
						}
						c.mu.Unlock()
						infoln("[WS] handshake ok, clientId=", c.clientID)
						// connect once handshake succeeds
						c.connect()
					}
				case "/meta/connect":
					// advice.timeout present (ms)
					timeout := 60000
					if advice, ok := m["advice"].(map[string]any); ok {
						if t, ok := advice["timeout"].(float64); ok {
							timeout = int(t)
						}
					}
					startHeartbeat := !c.connected
					c.connected = true
					infoln("[WS] connect ok, timeout=", timeout)
					if startHeartbeat {
						go c.heartbeatLoop(timeout)
					}
					// 若之前已登记订阅目标，则在 connect 成功后自动订阅
					if c.subscribed != "" && c.clientID != "" {
						var courseID, signID int
						fmt.Sscanf(c.subscribed, "%d/%d", &courseID, &signID)
						if courseID > 0 && signID > 0 {
							c.send([]any{map[string]any{
								"channel":      "/meta/subscribe",
								"clientId":     c.clientID,
								"subscription": fmt.Sprintf("/attendance/%d/%d/qr", courseID, signID),
								"id":           c.nextSeq(),
							}})
							infof("[WS] auto-subscribe after connect: /attendance/%d/%d/qr\n", courseID, signID)
						}
					}
				case "/meta/subscribe":
					infoln("[WS] subscribe ack")
				}
			} else {
				// non-successful: 打印并按 advice 处理
				if ch == "/meta/connect" {
					// 可能包含 advice: { reconnect: "handshake", interval: ms }
					if advice, ok := m["advice"].(map[string]any); ok {
						if rc, ok := advice["reconnect"].(string); ok && rc == "handshake" {
							interval := 0
							if iv, ok := advice["interval"].(float64); ok {
								interval = int(iv)
							}
							if interval > 0 {
								dbgln("[WS] reconnect advice interval=", interval, "ms")
								time.Sleep(time.Duration(interval) * time.Millisecond)
							}
							infoln("[WS] /meta/connect unknown client, re-handshaking...")
							c.rehandshake()
							continue
						}
					}
					if errStr, ok := m["error"].(string); ok {
						infoln("[WS] /meta/connect error:", errStr)
					}
				}
				if isQRChannel(ch) {
					dbgln("[WS] QR channel payload")
					c.handleQRMessage(m)
				}
			}
		}
	}
}

func (c *Client) connect() {
	c.send([]any{map[string]any{
		"channel":        "/meta/connect",
		"clientId":       c.clientID,
		"connectionType": "websocket",
		"id":             c.nextSeq(),
	}})
}

func (c *Client) heartbeatLoop(timeout int) {
	// Half of server advice timeout
	interval := time.Duration(timeout/2) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Bayeux heartbeat: empty array + connect
			c.send([]any{})
			c.connect()
		}
	}
}

// Attach subscribes to specific course/sign QR channel; safe to call multiple times.
func (c *Client) Attach(courseID, signID int) {
	key := fmt.Sprintf("%d/%d", courseID, signID)
	if c.subscribed == key || courseID == 0 || signID == 0 {
		return
	}
	// 记录期望订阅的目标
	c.subscribed = key
	// 若已连接则立即订阅；否则等待 /meta/connect 成功后自动订阅
	if !c.connected || c.clientID == "" {
		infoln("[WS] connect 尚未完成，延迟订阅:", key)
		return
	}
	c.send([]any{map[string]any{
		"channel":      "/meta/subscribe",
		"clientId":     c.clientID,
		"subscription": fmt.Sprintf("/attendance/%d/%d/qr", courseID, signID),
		"id":           c.nextSeq(),
	}})
}

func (c *Client) send(payload any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	_ = c.conn.WriteJSON(payload)
}

// rehandshake sends a fresh handshake per server advice and resets state
func (c *Client) rehandshake() {
	c.mu.Lock()
	c.clientID = ""
	c.connected = false
	c.subscribed = ""
	c.handshakeDone = make(chan struct{})
	c.mu.Unlock()
	c.send([]any{map[string]any{
		"channel":        "/meta/handshake",
		"version":        "1.0",
		"minimumVersion": "1.0",
		"supportedConnectionTypes": []string{
			"websocket", "eventsource", "long-polling", "cross-origin-long-polling", "callback-polling",
		},
		"id": c.nextSeq(),
	}})
	infoln("[WS] re-handshake sent")
}

var qrChanRe = regexp.MustCompile(`^/attendance/\d+/\d+/qr$`)

func isQRChannel(ch string) bool {
	return qrChanRe.MatchString(ch)
}

func (c *Client) handleQRMessage(m map[string]any) {
	// Expect m["data"].(map), with type == 1 and qrUrl string
	data, _ := m["data"].(map[string]any)
	if data == nil {
		return
	}
	// type: 1=code, 3=student
	t, _ := data["type"].(float64)
	if int(t) == 1 {
		if url, ok := data["qrUrl"].(string); ok && url != "" {
			// Render QR in terminal
			infof("[QR] 刷新二维码 @ %s\n", time.Now().Format(time.RFC3339))
			qr.Print(url)
		}
		return
	}
	if int(t) == 3 {
		// 学生签到结果
		if stu, ok := data["student"].(map[string]any); ok {
			res := StudentResult{
				Name:          strOf(stu["name"]),
				StudentNumber: strOf(stu["studentNumber"]),
				Rank:          intOf(stu["rank"]),
				ID:            int64Of(stu["id"]),
			}
			infof("[QR] 学生签到结果: name=%s number=%s rank=%d id=%d\n", res.Name, res.StudentNumber, res.Rank, res.ID)
			select { // 非阻塞发送，避免无人接收卡住
			case c.ResultCh <- res:
			default:
			}
		}
	}
}

type StudentResult struct {
	ID            int64
	Name          string
	StudentNumber string
	Rank          int
}

func strOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func intOf(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	default:
		return 0
	}
}
func int64Of(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	default:
		return 0
	}
}
