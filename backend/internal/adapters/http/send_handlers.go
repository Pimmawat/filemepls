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
	// maxMessageSize bounds a single inbound frame: a 256KB relayed file chunk
	// plus its peer-ID prefix, with room to spare for an SDP blob. The hub is
	// anonymous by default, so without a limit any visitor could make the
	// server buffer a frame of whatever size they felt like.
	maxMessageSize = 512 * 1024
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

func (c *wsClient) SendBinary(frame []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	// No log on failure: this runs once per relayed chunk, so a dead peer would
	// fill the log with one line per 256KB. The read loop tears the connection
	// down on its own anyway.
	_ = c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

// SendWSHandler upgrades to a WebSocket and joins the anonymous LAN-send
// signaling hub. Deliberately has no auth: any visitor who can reach this
// backend can discover and exchange files with any other currently
// connected visitor — the same trust model as LocalSend on a LAN. Normally
// the hub only relays signaling (WebRTC offer/answer/ICE candidates) and the
// file bytes go browser-to-browser over a WebRTC DataChannel; binary frames
// are the fallback for peers that cannot reach each other directly, and those
// do pass through here (see sendhub.Hub.RelayBinary).
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
		conn.SetReadLimit(maxMessageSize)
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
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))

			// Binary frames are relayed file bytes, not signaling:
			// [IDLen ASCII destination ID][payload]. Dropped silently when the
			// destination is gone - this runs once per chunk, and an error reply
			// per chunk is a flood, not a diagnostic. The sender finds out the
			// same way it would on a dead WebRTC channel: the transfer stalls.
			if msgType == websocket.BinaryMessage {
				if len(data) <= sendhub.IDLen {
					continue
				}
				_ = hub.RelayBinary(id, string(data[:sendhub.IDLen]), data[sendhub.IDLen:])
				continue
			}

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
