package agentgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestStateAndSeatsEndpoints(t *testing.T) {
	coord, s1ID, _ := setupMatchedSessions(t)

	router := chi.NewRouter()
	router.Get("/api/agent/sessions/{session_id}/state", StateHandler(coord))
	router.Get("/api/agent/sessions/{session_id}/seats", SeatsHandler(coord))
	router.Get("/api/agent/sessions/{session_id}/seats/{seat_id}", SeatByIDHandler(coord))

	req := httptest.NewRequest(http.MethodGet, "/api/agent/sessions/"+s1ID+"/state", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("state expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var state map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if _, ok := state["my_hole_cards"]; !ok {
		t.Fatalf("state missing my_hole_cards: %v", state)
	}
	if _, ok := state["community_cards"]; !ok {
		t.Fatalf("state missing community_cards")
	}
	seatsAny, ok := state["seats"].([]any)
	if !ok || len(seatsAny) != 2 {
		t.Fatalf("expected 2 seats, got %#v", state["seats"])
	}
	for _, s := range seatsAny {
		seatMap := s.(map[string]any)
		if _, ok := seatMap["street_contribution"]; !ok {
			t.Fatalf("seat missing street_contribution: %v", seatMap)
		}
		if _, ok := seatMap["to_call"]; !ok {
			t.Fatalf("seat missing to_call: %v", seatMap)
		}
		if _, ok := seatMap["hole_cards"]; ok {
			t.Fatalf("seat should not expose hole_cards: %v", seatMap)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agent/sessions/"+s1ID+"/seats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seats expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agent/sessions/"+s1ID+"/seats/0", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seat by id expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var seatResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &seatResp); err != nil {
		t.Fatalf("decode seat response: %v", err)
	}
	seat, ok := seatResp["seat"].(map[string]any)
	if !ok {
		t.Fatalf("seat payload missing: %v", seatResp)
	}
	if _, ok := seat["street_contribution"]; !ok {
		t.Fatalf("seat payload missing street_contribution")
	}
}
