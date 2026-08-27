package sendhub

import (
	"encoding/json"
	"testing"
)

type fakeClient struct {
	received []Message
	frames   [][]byte
}

func (c *fakeClient) Send(m Message) {
	c.received = append(c.received, m)
}

func (c *fakeClient) SendBinary(frame []byte) {
	c.frames = append(c.frames, frame)
}

func (c *fakeClient) last() Message {
	if len(c.received) == 0 {
		return Message{}
	}
	return c.received[len(c.received)-1]
}

func TestHub_JoinAssignsUniqueIDAndName(t *testing.T) {
	h := New()
	id1, name1 := h.Join(&fakeClient{})
	id2, name2 := h.Join(&fakeClient{})

	if id1 == "" || id2 == "" {
		t.Fatal("expected non-empty IDs")
	}
	if id1 == id2 {
		t.Error("expected distinct IDs for distinct clients")
	}
	if name1 == "" || name2 == "" {
		t.Error("expected non-empty assigned names")
	}
}

func TestHub_BroadcastRoster_ExcludesSelf(t *testing.T) {
	h := New()
	a, b := &fakeClient{}, &fakeClient{}
	idA, _ := h.Join(a)
	idB, _ := h.Join(b)

	h.BroadcastRoster()

	rosterA := a.last()
	if rosterA.Type != "roster" || len(rosterA.Peers) != 1 || rosterA.Peers[0].ID != idB {
		t.Errorf("A's roster = %+v, want exactly peer B", rosterA)
	}
	rosterB := b.last()
	if rosterB.Type != "roster" || len(rosterB.Peers) != 1 || rosterB.Peers[0].ID != idA {
		t.Errorf("B's roster = %+v, want exactly peer A", rosterB)
	}
}

func TestHub_Leave_UpdatesRemainingRoster(t *testing.T) {
	h := New()
	a, b := &fakeClient{}, &fakeClient{}
	idA, _ := h.Join(a)
	h.Join(b)
	h.BroadcastRoster()

	h.Leave(idA)
	h.BroadcastRoster()

	rosterB := b.last()
	if len(rosterB.Peers) != 0 {
		t.Errorf("B's roster after A left = %+v, want empty", rosterB)
	}
}

func TestHub_Relay_StampsFromAndDeliversPayload(t *testing.T) {
	h := New()
	a, b := &fakeClient{}, &fakeClient{}
	idA, _ := h.Join(a)
	idB, _ := h.Join(b)

	payload := json.RawMessage(`{"kind":"offer","sdp":"v=0..."}`)
	ok := h.Relay(idA, Message{Type: "signal", To: idB, Payload: payload})
	if !ok {
		t.Fatal("expected Relay to succeed")
	}

	got := b.last()
	if got.Type != "signal" || got.From != idA || string(got.Payload) != string(payload) {
		t.Errorf("B received %+v, want signal from %s with payload %s", got, idA, payload)
	}
	if got.To != "" {
		t.Errorf("relayed message.To = %q, want cleared", got.To)
	}
}

func TestHub_Relay_UnknownTargetReturnsFalse(t *testing.T) {
	h := New()
	a := &fakeClient{}
	idA, _ := h.Join(a)

	if h.Relay(idA, Message{Type: "signal", To: "no-such-peer"}) {
		t.Error("expected Relay to a disconnected peer to return false")
	}
}

func TestHub_Relay_FromNameStamped(t *testing.T) {
	h := New()
	a, b := &fakeClient{}, &fakeClient{}
	idA, nameA := h.Join(a)
	idB, _ := h.Join(b)

	h.Relay(idA, Message{Type: "signal", To: idB})

	if got := b.last().FromName; got != nameA {
		t.Errorf("FromName = %q, want %q", got, nameA)
	}
}

func TestHub_RelayBinary_StampsSenderAndForwardsPayload(t *testing.T) {
	h := New()
	from, _ := h.Join(&fakeClient{})
	target := &fakeClient{}
	to, _ := h.Join(target)

	if !h.RelayBinary(from, to, []byte("file bytes")) {
		t.Fatal("RelayBinary to a connected peer returned false")
	}
	if len(target.frames) != 1 {
		t.Fatalf("target frames = %d, want 1", len(target.frames))
	}

	frame := target.frames[0]
	if len(frame) <= IDLen {
		t.Fatalf("frame is %d bytes, want more than the %d-byte ID prefix", len(frame), IDLen)
	}
	// The receiver reads the sender's ID straight off the front of the frame,
	// so a wrong prefix length silently misroutes every chunk.
	if got := string(frame[:IDLen]); got != from {
		t.Errorf("frame ID prefix = %q, want %q", got, from)
	}
	if got := string(frame[IDLen:]); got != "file bytes" {
		t.Errorf("frame payload = %q, want %q", got, "file bytes")
	}
}

func TestHub_RelayBinary_UnknownPeer(t *testing.T) {
	h := New()
	from, _ := h.Join(&fakeClient{})

	if h.RelayBinary(from, "not-a-peer", []byte("x")) {
		t.Error("RelayBinary to a disconnected peer returned true")
	}
}

func TestHub_NewIDLength(t *testing.T) {
	// Both ends slice relay frames at exactly IDLen; if newID ever mints a
	// different width the prefix and the payload swap places.
	id, _ := New().Join(&fakeClient{})
	if len(id) != IDLen {
		t.Errorf("newID length = %d, want IDLen (%d)", len(id), IDLen)
	}
}
