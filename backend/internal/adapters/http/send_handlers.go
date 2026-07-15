package http

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"filemepls/internal/sendhub"
)

const (
	// pongWait is how long a connection may go without any read (data or a
	// pong reply to our ping) before it's considered dead and torn down.
	pongWait = 60 * time.Second
	// pingPeriod must be < pongWait so a live peer always answers in time.
	pingPeriod = (pongWait * 9) / 10
	// writeWait caps a single write so one stuck client can't block a roster
	// broadcast (or the ping loop) indefinitely.
	writeWait = 10 * time.Second
)

// wsClient adapts a single *websocket.Conn to sendhub.Client. A mutex
// guards writes because the hub can call Send from other clients'
// goroutines (e.g. relaying a signal, or broadcasting a roster change)
// concurrently with this connection's own read loop — gorilla only allows
// one concurrent writer per connection.
type wsClient struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (c *wsClient) Send(msg sendhub.Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("sendhub: marshal message: %v", err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Printf("sendhub: write message: %v", err)
	}
}

// SendWSHandler upgrades to a WebSocket and joins the anonymous LAN-send
// signaling hub. Deliberately has no auth: any visitor who can reach this
// backend can discover and exchange files with any other currently
// connected visitor — the same trust model as LocalSend on a LAN. The hub
// only relays signaling (WebRTC offer/answer/ICE candidates); actual file
// bytes are exchanged directly between browsers over a WebRTC DataChannel
// and never pass through the server.
func SendWSHandler(hub *sendhub.Hub, allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		// A missing Origin header means a non-browser client (or a same-
		// origin request some proxies strip it from); only reject when an
		// Origin is present and doesn't match the configured allow-list.
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || allowed[origin]
		},
	}

	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return // Upgrade already wrote an HTTP error response
		}
		client := &wsClient{conn: conn}
		defer func() { _ = conn.Close() }()

		// A dead-but-not-closed (half-open) TCP connection would otherwise sit
		// in the roster forever and leak this goroutine. The read deadline +
		// pong handler tear it down: every ping we send must be answered
		// (browsers auto-pong) within pongWait, and any inbound frame also
		// refreshes the deadline.
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})

		id, name := hub.Join(client)
		defer func() {
			hub.Leave(id)
			hub.BroadcastRoster()
		}()

		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					client.mu.Lock()
					_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
					err := conn.WriteMessage(websocket.PingMessage, nil)
					client.mu.Unlock()
					if err != nil {
						return
					}
				}
			}
		}()

		client.Send(sendhub.Message{Type: "hello", SelfID: id, SelfName: name})
		hub.BroadcastRoster()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))

			var msg sendhub.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type != "signal" || msg.To == "" {
				continue
			}
			if !hub.Relay(id, msg) {
				client.Send(sendhub.Message{Type: "error", Error: "peer not connected"})
			}
		}
	}
}
