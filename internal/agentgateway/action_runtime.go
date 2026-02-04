package agentgateway

import (
	"context"
	"errors"
	"strings"

	"silicon-casino/internal/game"
)

var (
	errSessionNotFound  = errors.New("session_not_found")
	errInvalidRequestID = errors.New("invalid_request_id")
	errInvalidTurnID    = errors.New("invalid_turn_id")
	errNotYourTurn      = errors.New("not_your_turn")
	errInvalidAction    = errors.New("invalid_action")
	errInvalidRaise     = errors.New("invalid_raise")
)

func (c *Coordinator) SubmitAction(ctx context.Context, sessionID string, req ActionRequest) (*ActionResponse, error) {
	if len(req.RequestID) < 1 || len(req.RequestID) > 64 {
		return nil, errInvalidRequestID
	}
	if req.Action == "" {
		return nil, errInvalidAction
	}
	prev, err := c.getIdempotentActionResult(ctx, sessionID, req.RequestID)
	if err != nil {
		return nil, err
	}
	if prev != nil {
		return prev, nil
	}

	c.mu.Lock()
	sess := c.sessions[sessionID]
	if sess == nil || sess.runtime == nil {
		c.mu.Unlock()
		return nil, errSessionNotFound
	}
	rt := sess.runtime
	c.mu.Unlock()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if req.TurnID != rt.turnID {
		res := ActionResponse{Accepted: false, RequestID: req.RequestID, Reason: "invalid_turn_id"}
		_, _ = c.saveActionResult(ctx, sessionID, req, res)
		if sess.buffer != nil {
			sess.buffer.Append("action_rejected", sessionID, map[string]any{
				"request_id": req.RequestID,
				"turn_id":    req.TurnID,
				"reason":     "invalid_turn_id",
			})
		}
		return &res, errInvalidTurnID
	}
	actor := rt.engine.State.CurrentActor
	if sess.seat != actor {
		res := ActionResponse{Accepted: false, RequestID: req.RequestID, Reason: "not_your_turn"}
		_, _ = c.saveActionResult(ctx, sessionID, req, res)
		if sess.buffer != nil {
			sess.buffer.Append("action_rejected", sessionID, map[string]any{
				"request_id": req.RequestID,
				"turn_id":    req.TurnID,
				"reason":     "not_your_turn",
			})
		}
		return &res, errNotYourTurn
	}

	action := game.Action{
		Player: actor,
		Type:   game.ActionType(req.Action),
	}
	if req.Amount != nil {
		action.Amount = *req.Amount
	}
	done, applyErr := rt.engine.ApplyAction(ctx, action)
	if applyErr != nil {
		reason := mapApplyError(applyErr)
		res := ActionResponse{Accepted: false, RequestID: req.RequestID, Reason: reason}
		_, _ = c.saveActionResult(ctx, sessionID, req, res)
		if sess.buffer != nil {
			sess.buffer.Append("action_rejected", sessionID, map[string]any{
				"request_id": req.RequestID,
				"turn_id":    req.TurnID,
				"reason":     reason,
			})
		}
		if reason == "invalid_raise" {
			return &res, errInvalidRaise
		}
		return &res, errInvalidAction
	}
	c.emitPublicActionLog(rt, actor, req.Action, req.Amount)
	if done {
		if handDone := rt.handleRoundEnd(ctx); handDone {
			_ = rt.startNextHand(ctx)
		}
	}
	rt.turnID = nextTurnID()
	res := ActionResponse{Accepted: true, RequestID: req.RequestID}
	_, err = c.saveActionResult(ctx, sessionID, req, res)
	if err != nil {
		return nil, err
	}
	if sess.buffer != nil {
		sess.buffer.Append("action_accepted", sessionID, map[string]any{
			"request_id": req.RequestID,
			"turn_id":    req.TurnID,
		})
	}
	for _, p := range rt.players {
		c.emitStateSnapshot(p)
	}
	c.emitTurnStarted(rt)
	c.emitPublicSnapshot(rt)
	return &res, nil
}

func mapApplyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid_raise"):
		return "invalid_raise"
	case strings.Contains(msg, "invalid_action"):
		return "invalid_action"
	case strings.Contains(msg, "not_your_turn"):
		return "not_your_turn"
	default:
		return "invalid_action"
	}
}

func (rt *tableRuntime) handleRoundEnd(ctx context.Context) bool {
	st := rt.engine.State
	if st.Players[0].Folded || st.Players[1].Folded {
		_, _ = rt.engine.Settle(ctx)
		return true
	}
	if st.Players[0].AllIn || st.Players[1].AllIn {
		rt.engine.FastForwardToShowdown()
		_, _ = rt.engine.Settle(ctx)
		return true
	}
	if st.Street == game.StreetRiver {
		_, _ = rt.engine.Settle(ctx)
		return true
	}
	rt.engine.NextStreet()
	return false
}

func (rt *tableRuntime) startNextHand(ctx context.Context) error {
	players := [2]*game.Player{
		{ID: rt.players[0].agent.ID, Name: rt.players[0].agent.Name, Seat: 0},
		{ID: rt.players[1].agent.ID, Name: rt.players[1].agent.Name, Seat: 1},
	}
	return rt.engine.StartHand(ctx, players[0], players[1], rt.room.SmallBlindCC, rt.room.BigBlindCC)
}
