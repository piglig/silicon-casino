package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"silicon-casino/internal/agentgateway"
	"silicon-casino/internal/store"

	"github.com/go-chi/chi/v5"
)

func publicRoomsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := st.ListRooms(r.Context())
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := []map[string]any{}
		for _, it := range items {
			out = append(out, map[string]any{
				"id":             it.ID,
				"name":           it.Name,
				"min_buyin_cc":   it.MinBuyinCC,
				"small_blind_cc": it.SmallBlindCC,
				"big_blind_cc":   it.BigBlindCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
	}
}

func publicTablesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room_id")
		limit, offset := parsePagination(r)
		items, err := st.ListTables(r.Context(), roomID, limit, offset)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"table_id":       it.ID,
				"room_id":        it.RoomID,
				"status":         it.Status,
				"created_at":     it.CreatedAt,
				"small_blind_cc": it.SmallBlindCC,
				"big_blind_cc":   it.BigBlindCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func publicTableHistoryHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		roomID := r.URL.Query().Get("room_id")
		agentID := r.URL.Query().Get("agent_id")
		items, err := st.ListTableHistory(r.Context(), roomID, agentID, limit, offset)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"table_id":           it.TableID,
				"room_id":            it.RoomID,
				"status":             it.Status,
				"small_blind_cc":     it.SmallBlindCC,
				"big_blind_cc":       it.BigBlindCC,
				"created_at":         it.CreatedAt,
				"last_hand_ended_at": it.LastHandEnded,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func publicAgentTableHandler(coord *agentgateway.Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		tableID, roomID, ok := coord.FindTableByAgent(agentID)
		if !ok {
			writeHTTPError(w, http.StatusNotFound, "table_not_found")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": agentID,
			"room_id":  roomID,
			"table_id": tableID,
		})
	}
}

func publicAgentTablesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := chi.URLParam(r, "agent_id")
		if agentID == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		limit, offset := parsePagination(r)
		items, err := st.ListAgentTables(r.Context(), agentID, limit, offset)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"table_id":           it.TableID,
				"room_id":            it.RoomID,
				"status":             it.Status,
				"small_blind_cc":     it.SmallBlindCC,
				"big_blind_cc":       it.BigBlindCC,
				"created_at":         it.CreatedAt,
				"last_hand_ended_at": it.LastHandEnded,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func publicTableReplayHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer replayQueryP95MS.Set(time.Since(start).Milliseconds())
		replayQueryTotal.Add(1)
		tableID := chi.URLParam(r, "table_id")
		if tableID == "" {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		fromSeq := int64(1)
		if v := r.URL.Query().Get("from_seq"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 1 {
				replayQueryErrorsTotal.Add(1)
				writeHTTPError(w, http.StatusBadRequest, "invalid_request")
				return
			}
			fromSeq = n
		}
		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				replayQueryErrorsTotal.Add(1)
				writeHTTPError(w, http.StatusBadRequest, "invalid_request")
				return
			}
			limit = n
		}
		if limit > 500 {
			limit = 500
		}
		lastSeq, err := st.GetTableReplayLastSeq(r.Context(), tableID)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if lastSeq == 0 {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusNotFound, "table_not_found")
			return
		}
		items, err := st.ListTableReplayEventsFromSeq(r.Context(), tableID, fromSeq, limit)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			var payload any
			if len(it.Payload) > 0 {
				_ = json.Unmarshal(it.Payload, &payload)
			}
			out = append(out, map[string]any{
				"id":             it.ID,
				"table_id":       it.TableID,
				"hand_id":        it.HandID,
				"global_seq":     it.GlobalSeq,
				"hand_seq":       it.HandSeq,
				"event_type":     it.EventType,
				"actor_agent_id": it.ActorAgentID,
				"payload":        payload,
				"schema_version": it.SchemaVer,
				"created_at":     it.CreatedAt,
			})
		}
		nextFrom := fromSeq + int64(len(items))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":         out,
			"next_from_seq": nextFrom,
			"has_more":      nextFrom <= lastSeq,
			"last_seq":      lastSeq,
		})
	}
}

func publicTableTimelineHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer replayQueryP95MS.Set(time.Since(start).Milliseconds())
		replayQueryTotal.Add(1)
		tableID := chi.URLParam(r, "table_id")
		if tableID == "" {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		hands, err := st.ListHandsByTableID(r.Context(), tableID)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		lastSeq, err := st.GetTableReplayLastSeq(r.Context(), tableID)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if lastSeq == 0 {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusNotFound, "table_not_found")
			return
		}
		events, err := st.ListTableReplayEventsFromSeq(r.Context(), tableID, 1, int(lastSeq))
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		byHandSeqRange := make(map[string]map[string]int64)
		for _, ev := range events {
			if ev.HandID == "" {
				continue
			}
			rng := byHandSeqRange[ev.HandID]
			if rng == nil {
				rng = map[string]int64{"start": ev.GlobalSeq, "end": ev.GlobalSeq}
				byHandSeqRange[ev.HandID] = rng
			}
			if ev.GlobalSeq < rng["start"] {
				rng["start"] = ev.GlobalSeq
			}
			if ev.GlobalSeq > rng["end"] {
				rng["end"] = ev.GlobalSeq
			}
		}
		out := make([]map[string]any, 0, len(hands))
		for _, h := range hands {
			rng := byHandSeqRange[h.ID]
			startSeq := int64(0)
			endSeq := int64(0)
			if rng != nil {
				startSeq = rng["start"]
				endSeq = rng["end"]
			}
			out = append(out, map[string]any{
				"hand_id":         h.ID,
				"start_seq":       startSeq,
				"end_seq":         endSeq,
				"winner_agent_id": h.WinnerAgentID,
				"pot_cc":          h.PotCC,
				"street_end":      h.StreetEnd,
				"started_at":      h.StartedAt,
				"ended_at":        h.EndedAt,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"table_id": tableID,
			"items":    out,
		})
	}
}

func publicTableSnapshotHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		replayQueryTotal.Add(1)
		start := time.Now()
		defer replayQueryP95MS.Set(time.Since(start).Milliseconds())
		tableID := chi.URLParam(r, "table_id")
		if tableID == "" {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		atSeqRaw := r.URL.Query().Get("at_seq")
		if atSeqRaw == "" {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		atSeq, err := strconv.ParseInt(atSeqRaw, 10, 64)
		if err != nil || atSeq < 1 {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		lastSeq, err := st.GetTableReplayLastSeq(r.Context(), tableID)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if lastSeq == 0 {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusNotFound, "table_not_found")
			return
		}
		if atSeq > lastSeq {
			atSeq = lastSeq
		}
		snap, err := st.GetLatestTableReplaySnapshotAtOrBefore(r.Context(), tableID, atSeq)
		snapshotSeq := int64(0)
		replayState := map[string]any{}
		if err == nil && snap != nil {
			replaySnapshotHitTotal.Add(1)
			snapshotSeq = snap.AtGlobalSeq
			_ = json.Unmarshal(snap.StateBlob, &replayState)
		} else {
			replaySnapshotMissTotal.Add(1)
		}
		hits := replaySnapshotHitTotal.Value()
		total := hits + replaySnapshotMissTotal.Value()
		if total > 0 {
			replaySnapshotHitRatio.Set(float64(hits) / float64(total))
		}
		fromSeq := snapshotSeq + 1
		limit := int(atSeq - snapshotSeq + 1)
		if limit < 1 {
			limit = 1
		}
		events, err := st.ListTableReplayEventsFromSeq(r.Context(), tableID, fromSeq, limit)
		if err != nil {
			replayQueryErrorsTotal.Add(1)
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		for _, ev := range events {
			if ev.GlobalSeq > atSeq {
				break
			}
			var payload map[string]any
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				continue
			}
			if ev.EventType == "state_snapshot" {
				replayState = payload
			}
			replayState["last_event_type"] = ev.EventType
			replayState["global_seq"] = ev.GlobalSeq
		}
		replaySnapshotRebuildMS.Set(time.Since(start).Milliseconds())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"table_id": tableID,
			"at_seq":   atSeq,
			"state":    replayState,
		})
	}
}

func publicLeaderboardHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		items, err := st.ListLeaderboard(r.Context(), limit, offset)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"agent_id": it.AgentID,
				"name":     it.Name,
				"net_cc":   it.NetCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}
