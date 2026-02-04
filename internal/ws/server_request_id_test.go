package ws

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleActionValidRequestIDEnqueued(t *testing.T) {
	srv := &Server{}
	client := &Client{playerIdx: 0}
	session := &TableSession{
		players:  [2]*Client{client, nil},
		actionCh: make(chan ActionEnvelope, 1),
	}
	client.session = session

	msg := []byte(`{"type":"action","request_id":"req_123","action":"check"}`)
	srv.handleAction(client, msg)

	select {
	case got := <-session.actionCh:
		if got.RequestID != "req_123" {
			t.Fatalf("expected request_id req_123, got %q", got.RequestID)
		}
		if string(got.Action.Type) != "check" {
			t.Fatalf("expected action check, got %q", got.Action.Type)
		}
	default:
		t.Fatal("expected action to be enqueued")
	}
}

func TestHandleActionMissingRequestIDReturnsError(t *testing.T) {
	srv := &Server{}
	send := make(chan []byte, 1)
	client := &Client{playerIdx: 0, send: send}
	session := &TableSession{
		players:  [2]*Client{client, nil},
		actionCh: make(chan ActionEnvelope, 1),
	}
	client.session = session

	msg := []byte(`{"type":"action","action":"check"}`)
	srv.handleAction(client, msg)

	var got ActionResult
	select {
	case raw := <-send:
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal action_result: %v", err)
		}
	default:
		t.Fatal("expected action_result response")
	}
	if got.Ok {
		t.Fatal("expected ok=false")
	}
	if got.Error != "invalid_request_id" {
		t.Fatalf("expected invalid_request_id, got %q", got.Error)
	}
	if got.RequestID != "" {
		t.Fatalf("expected empty request_id, got %q", got.RequestID)
	}
}

func TestHandleActionRequestIDTooLongReturnsError(t *testing.T) {
	srv := &Server{}
	send := make(chan []byte, 1)
	client := &Client{playerIdx: 0, send: send}
	session := &TableSession{
		players:  [2]*Client{client, nil},
		actionCh: make(chan ActionEnvelope, 1),
	}
	client.session = session

	longID := strings.Repeat("a", 65)
	msg := []byte(`{"type":"action","request_id":"` + longID + `","action":"check"}`)
	srv.handleAction(client, msg)

	var got ActionResult
	select {
	case raw := <-send:
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal action_result: %v", err)
		}
	default:
		t.Fatal("expected action_result response")
	}
	if got.Ok {
		t.Fatal("expected ok=false")
	}
	if got.Error != "invalid_request_id" {
		t.Fatalf("expected invalid_request_id, got %q", got.Error)
	}
	if got.RequestID != longID {
		t.Fatalf("expected request_id to echo invalid value, got %q", got.RequestID)
	}
}
