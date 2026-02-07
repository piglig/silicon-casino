package agentgateway

import "silicon-casino/internal/game/viewmodel"

func (c *Coordinator) emitStateSnapshot(sess *sessionState) {
	if sess == nil || sess.runtime == nil || sess.buffer == nil {
		return
	}
	state := viewmodel.BuildAgentState(sess.runtime.engine.State, sess.seat, sess.runtime.turnID, false)
	sess.buffer.Append("state_snapshot", sess.session.ID, state)
}

func (c *Coordinator) emitTurnStarted(rt *tableRuntime) {
	if rt == nil {
		return
	}
	actorSeat := rt.engine.State.CurrentActor
	allowedActions := []string{}
	actorState := viewmodel.BuildAgentState(rt.engine.State, actorSeat, rt.turnID, false)
	if len(actorState.LegalActions) > 0 {
		allowedActions = actorState.LegalActions
	}
	for _, sess := range rt.players {
		if sess == nil || sess.buffer == nil {
			continue
		}
		sess.buffer.Append("turn_started", sess.session.ID, map[string]any{
			"hand_id":         rt.engine.State.HandID,
			"turn_id":         rt.turnID,
			"seat_id":         actorSeat,
			"deadline_ms":     rt.engine.State.ActionTimeout.Milliseconds(),
			"allowed_actions": allowedActions,
		})
	}
}

func (c *Coordinator) emitPublicSnapshot(rt *tableRuntime) {
	if rt == nil || rt.publicBuffer == nil {
		return
	}
	state := viewmodel.BuildPublicState(rt.engine.State)
	rt.publicBuffer.Append("table_snapshot", rt.id, state)
}

func (c *Coordinator) emitPublicActionLog(rt *tableRuntime, seat int, action string, amount *int64, thoughtLog string) {
	if rt == nil || rt.publicBuffer == nil {
		return
	}
	rt.publicBuffer.Append("action_log", rt.id, map[string]any{
		"player_seat": seat,
		"action":      action,
		"amount":      amount,
		"thought_log": thoughtLog,
		"event":       "action",
	})
}
