package agentgateway

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func ActionsHandler(coord *Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "session_id")
		if sessionID == "" {
			writeErr(w, http.StatusBadRequest, "session_not_found")
			return
		}
		var req ActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_json")
			return
		}
		res, err := coord.SubmitAction(r.Context(), sessionID, req)
		if err != nil {
			status, code := mapActionErr(err)
			writeErr(w, status, code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}
}

func mapActionErr(err error) (int, string) {
	switch {
	case errors.Is(err, errSessionNotFound):
		return http.StatusNotFound, "session_not_found"
	case errors.Is(err, errInvalidRequestID):
		return http.StatusBadRequest, "invalid_request_id"
	case errors.Is(err, errInvalidTurnID):
		return http.StatusBadRequest, "invalid_turn_id"
	case errors.Is(err, errNotYourTurn):
		return http.StatusBadRequest, "not_your_turn"
	case errors.Is(err, errInvalidAction):
		return http.StatusBadRequest, "invalid_action"
	case errors.Is(err, errInvalidRaise):
		return http.StatusBadRequest, "invalid_raise"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
