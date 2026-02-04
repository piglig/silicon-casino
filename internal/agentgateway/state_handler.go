package agentgateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"silicon-casino/internal/game/viewmodel"

	"github.com/go-chi/chi/v5"
)

func StateHandler(coord *Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "session_id")
		if sessionID == "" {
			writeErr(w, http.StatusBadRequest, "session_not_found")
			return
		}
		state, err := coord.GetState(sessionID)
		if err != nil {
			if errors.Is(err, errSessionNotFound) {
				writeErr(w, http.StatusNotFound, "session_not_found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal_error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	}
}

func SeatsHandler(coord *Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "session_id")
		state, err := coord.GetState(sessionID)
		if err != nil {
			if errors.Is(err, errSessionNotFound) {
				writeErr(w, http.StatusNotFound, "session_not_found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal_error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hand_id": state.HandID,
			"street":  state.Street,
			"seats":   state.Seats,
		})
	}
}

func SeatByIDHandler(coord *Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "session_id")
		seatIDRaw := chi.URLParam(r, "seat_id")
		seatID, err := strconv.Atoi(seatIDRaw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_seat_id")
			return
		}
		state, err := coord.GetState(sessionID)
		if err != nil {
			if errors.Is(err, errSessionNotFound) {
				writeErr(w, http.StatusNotFound, "session_not_found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "internal_error")
			return
		}
		var seat *viewmodel.SeatView
		for i := range state.Seats {
			if state.Seats[i].SeatID == seatID {
				seat = &state.Seats[i]
				break
			}
		}
		if seat == nil {
			writeErr(w, http.StatusNotFound, "seat_not_found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hand_id": state.HandID,
			"street":  state.Street,
			"seat":    seat,
		})
	}
}

func (c *Coordinator) GetState(sessionID string) (viewmodel.AgentStateView, error) {
	c.mu.Lock()
	sess := c.sessions[sessionID]
	if sess == nil || sess.runtime == nil {
		c.mu.Unlock()
		return viewmodel.AgentStateView{}, errSessionNotFound
	}
	rt := sess.runtime
	c.mu.Unlock()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	return viewmodel.BuildAgentState(rt.engine.State, sess.seat, rt.turnID, false), nil
}
