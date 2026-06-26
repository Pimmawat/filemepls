package http

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"filemepls/internal/sendhub"
)

// dialSend opens a WebSocket connection to the test server's /api/send/ws
// endpoint and reads its "hello" message, returning the connection plus
// the assigned self ID for use in later assertions.
func dialSend(t *testing.T, wsURL string) (*websocket.Conn, sendhub.Message) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	var hello sendhub.Message
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatalf("read hello: %v", err)
	}
	if hello.Type != "hello" || hello.SelfID == "" {
		t.Fatalf("got %+v, want a hello with a non-empty selfId", hello)
	}
	return conn, hello
}

// readUntil reads messages off conn until one matches want, or fails the
// test after a timeout — roster broadcasts can arrive interleaved with
// other messages depending on connection/disconnection timing.
func readUntilType(t *testing.T, conn *websocket.Conn, msgType string) sendhub.Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		var msg sendhub.Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("waiting for %q: %v", msgType, err)
		}
		if msg.Type == msgType {
			return msg
		}
	}
}

func TestSendWS_HelloThenRosterOnSecondJoin(t *testing.T) {
	hub := sendhub.New()
	router := NewRouter(Deps{SendHub: hub, AllowedOrigins: []string{"http://localhost:3000"}})
	srv := httptest.NewServer(router)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/send/ws"

	connA, helloA := dialSend(t, wsURL)

	// A's roster starts empty.
	rosterA := readUntilType(t, connA, "roster")
	if len(rosterA.Peers) != 0 {
		t.Fatalf("A's initial roster = %+v, want empty", rosterA.Peers)
	}

	connB, helloB := dialSend(t, wsURL)

	// Once B joins, both A and B should see each other in an updated roster.
	rosterA = readUntilType(t, connA, "roster")
	if len(rosterA.Peers) != 1 || rosterA.Peers[0].ID != helloB.SelfID {
		t.Fatalf("A's roster after B joined = %+v, want exactly B", rosterA.Peers)
	}
	rosterB := readUntilType(t, connB, "roster")
	if len(rosterB.Peers) != 1 || rosterB.Peers[0].ID != helloA.SelfID {
		t.Fatalf("B's roster = %+v, want exactly A", rosterB.Peers)
	}
}

func TestSendWS_RelaysSignalBetweenPeers(t *testing.T) {
	hub := sendhub.New()
	router := NewRouter(Deps{SendHub: hub, AllowedOrigins: []string{"http://localhost:3000"}})
	srv := httptest.NewServer(router)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/send/ws"

	connA, helloA := dialSend(t, wsURL)
	connB, helloB := dialSend(t, wsURL)
	// Drain the roster broadcasts triggered by B joining before exercising relay.
	readUntilType(t, connA, "roster")
	readUntilType(t, connB, "roster")

	offer := sendhub.Message{Type: "signal", To: helloB.SelfID, Payload: []byte(`{"kind":"offer","sdp":"v=0"}`)}
	if err := connA.WriteJSON(offer); err != nil {
		t.Fatalf("WriteJSON() error: %v", err)
	}

	got := readUntilType(t, connB, "signal")
	if got.From != helloA.SelfID || got.FromName != helloA.SelfName {
		t.Fatalf("B received %+v, want From=%s FromName=%s", got, helloA.SelfID, helloA.SelfName)
	}
	if string(got.Payload) != `{"kind":"offer","sdp":"v=0"}` {
		t.Errorf("payload = %s, want passthrough of the original", got.Payload)
	}
}

func TestSendWS_RelayToDisconnectedPeerReturnsError(t *testing.T) {
	hub := sendhub.New()
	router := NewRouter(Deps{SendHub: hub, AllowedOrigins: []string{"http://localhost:3000"}})
	srv := httptest.NewServer(router)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/send/ws"

	conn, _ := dialSend(t, wsURL)
	readUntilType(t, conn, "roster")

	if err := conn.WriteJSON(sendhub.Message{Type: "signal", To: "no-such-peer"}); err != nil {
		t.Fatalf("WriteJSON() error: %v", err)
	}

	got := readUntilType(t, conn, "error")
	if got.Error == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestSendWS_DisconnectUpdatesRoster(t *testing.T) {
	hub := sendhub.New()
	router := NewRouter(Deps{SendHub: hub, AllowedOrigins: []string{"http://localhost:3000"}})
	srv := httptest.NewServer(router)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/send/ws"

	connA, _ := dialSend(t, wsURL)
	readUntilType(t, connA, "roster")
	connB, _ := dialSend(t, wsURL)
	readUntilType(t, connA, "roster") // A sees B join
	readUntilType(t, connB, "roster")

	_ = connB.Close()

	rosterA := readUntilType(t, connA, "roster")
	if len(rosterA.Peers) != 0 {
		t.Fatalf("A's roster after B disconnected = %+v, want empty", rosterA.Peers)
	}
}
