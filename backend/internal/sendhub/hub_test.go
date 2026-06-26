package sendhub

import (
	"encoding/json"
	"testing"
)

type fakeClient struct {
	received []Message
}

func (c *fakeClient) Send(m Message) {
	c.received = append(c.received, m)
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
