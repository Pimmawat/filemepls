// Package sendhub implements the signaling side of a LocalSend-style
// "nearby device" file transfer: anonymous, no-login presence and message
// relay between whoever currently has the /send page open. It deliberately
// knows nothing about WebRTC — it just tracks who's connected and forwards
// opaque signaling payloads (SDP offers/answers, ICE candidates) between a
// sender and a chosen peer. Actual file bytes never pass through here, or
// through the server at all: the browsers exchange them directly over a
// WebRTC DataChannel once signaling completes.
package sendhub

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
)

// Message is the single envelope type exchanged over the WebSocket.
//   - "hello": server->client, sent once on connect with the client's own
//     assigned ID/name.
//   - "roster": server->client, the current list of other connected peers.
//     Sent to everyone whenever the roster changes.
//   - "signal": client->server to relay Payload to peer To; server->client
//     with From/FromName stamped in, Payload passed through unchanged.
//   - "error": server->client, relay failed (e.g. target peer disconnected
//     mid-handshake).
type Message struct {
	Type     string `json:"type"`
	SelfID   string `json:"selfId,omitempty"`
	SelfName string `json:"selfName,omitempty"`
	// No omitempty: a "roster" message must always serialize peers as an
	// array, even when empty — frontend code relies on it always being
	// present (an omitted field decodes to undefined in JS, not []).
	Peers    []Peer          `json:"peers"`
	To       string          `json:"to,omitempty"`
	From     string          `json:"from,omitempty"`
	FromName string          `json:"fromName,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Client is anything the hub can push a Message to. Implemented by the
// HTTP adapter's WebSocket connection wrapper — this package stays free of
// any networking dependency, so it's plain, fast-to-test Go.
type Client interface {
	Send(Message)
}

type Hub struct {
	mu      sync.Mutex
	clients map[string]registeredClient
}

type registeredClient struct {
	name   string
	client Client
}

func New() *Hub {
	return &Hub{clients: make(map[string]registeredClient)}
}

// Join registers a new anonymous client under a fresh ID and a randomly
// assigned friendly name. It does not broadcast the updated roster —
// callers send the client its own "hello" first, then call
// BroadcastRoster, so the new client never misses its own hello behind a
// roster message.
func (h *Hub) Join(client Client) (id, name string) {
	id = newID()
	name = randomName()

	h.mu.Lock()
	h.clients[id] = registeredClient{name: name, client: client}
	h.mu.Unlock()
	return id, name
}

// Leave removes a disconnected client. Callers are expected to call
// BroadcastRoster afterward so remaining clients see it disappear.
func (h *Hub) Leave(id string) {
	h.mu.Lock()
	delete(h.clients, id)
	h.mu.Unlock()
}

// BroadcastRoster pushes the current roster to every connected client. Each
// client receives every other connected peer, never itself.
func (h *Hub) BroadcastRoster() {
	h.mu.Lock()
	snapshot := make(map[string]registeredClient, len(h.clients))
	for id, c := range h.clients {
		snapshot[id] = c
	}
	h.mu.Unlock()

	for selfID, self := range snapshot {
		peers := make([]Peer, 0, len(snapshot)-1)
		for id, c := range snapshot {
			if id == selfID {
				continue
			}
			peers = append(peers, Peer{ID: id, Name: c.name})
		}
		self.client.Send(Message{Type: "roster", Peers: peers})
	}
}

// Relay forwards a "signal" message from fromID to msg.To, stamping
// From/FromName so the recipient knows who sent it. Returns false if the
// target isn't currently connected.
func (h *Hub) Relay(fromID string, msg Message) bool {
	h.mu.Lock()
	target, ok := h.clients[msg.To]
	fromName := h.clients[fromID].name
	h.mu.Unlock()
	if !ok {
		return false
	}

	msg.From = fromID
	msg.FromName = fromName
	msg.To = ""
	target.client.Send(msg)
	return true
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

var adjectives = []string{
	"Swift", "Quiet", "Bright", "Calm", "Brave", "Lucky", "Clever", "Gentle",
	"Bold", "Sunny", "Misty", "Golden", "Silver", "Cosmic", "Mighty", "Nimble",
}

var nouns = []string{
	"Fox", "Tiger", "Eagle", "Panda", "Otter", "Falcon", "Wolf", "Heron",
	"Dolphin", "Lynx", "Sparrow", "Koala", "Raven", "Badger", "Hawk", "Seal",
}

// randomName mints a LocalSend-style two-word display name (e.g. "Swift
// Blue Fox" minus the color, since we only need enough entropy to avoid
// confusing two people on the same screen, not global uniqueness — that's
// the ID's job).
func randomName() string {
	return fmt.Sprintf("%s %s", adjectives[randIndex(len(adjectives))], nouns[randIndex(len(nouns))])
}

func randIndex(n int) int {
	var b [1]byte
	_, _ = rand.Read(b[:])
	return int(b[0]) % n
}
