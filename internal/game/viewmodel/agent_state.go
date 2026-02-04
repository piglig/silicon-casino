package viewmodel

import "silicon-casino/internal/game"

type SeatView struct {
	SeatID             int    `json:"seat_id"`
	AgentID            string `json:"agent_id"`
	Stack              int64  `json:"stack"`
	StreetContribution int64  `json:"street_contribution"`
	ToCall             int64  `json:"to_call"`
	LastAction         string `json:"last_action"`
	LastActionAmount   *int64 `json:"last_action_amount,omitempty"`
	IsActive           bool   `json:"is_active"`
}

type AgentStateView struct {
	HandID           string     `json:"hand_id"`
	Street           string     `json:"street"`
	Pot              int64      `json:"pot"`
	CommunityCards   []string   `json:"community_cards"`
	CurrentActorSeat int        `json:"current_actor_seat"`
	TurnID           string     `json:"turn_id"`
	ActionTimeoutMS  int64      `json:"action_timeout_ms"`
	MySeat           int        `json:"my_seat"`
	MyBalance        int64      `json:"my_balance"`
	MyHoleCards      []string   `json:"my_hole_cards"`
	Seats            []SeatView `json:"seats"`
}

type PublicStateView struct {
	HandID           string     `json:"hand_id"`
	Street           string     `json:"street"`
	Pot              int64      `json:"pot"`
	CommunityCards   []string   `json:"community_cards"`
	CurrentActorSeat int        `json:"current_actor_seat"`
	ActionTimeoutMS  int64      `json:"action_timeout_ms"`
	Seats            []SeatView `json:"seats"`
}

func BuildAgentState(st *game.TableState, mySeat int, turnID string, includeOthersHole bool) AgentStateView {
	community := make([]string, 0, len(st.Community))
	for _, c := range st.Community {
		community = append(community, c.String())
	}

	myCards := []string{}
	for _, c := range st.Players[mySeat].Hole {
		myCards = append(myCards, c.String())
	}

	seats := make([]SeatView, 0, len(st.Players))
	for i, p := range st.Players {
		if p == nil {
			continue
		}
		toCall := st.CurrentBet - st.RoundBets[i]
		if toCall < 0 {
			toCall = 0
		}
		var lastAmount *int64
		if st.RoundBets[i] > 0 {
			v := st.RoundBets[i]
			lastAmount = &v
		}
		seats = append(seats, SeatView{
			SeatID:             p.Seat,
			AgentID:            p.ID,
			Stack:              p.Stack,
			StreetContribution: st.RoundBets[i],
			ToCall:             toCall,
			LastAction:         string(p.LastAction),
			LastActionAmount:   lastAmount,
			IsActive:           !p.Folded,
		})
	}

	myBalance := int64(0)
	if st.Players[mySeat] != nil {
		myBalance = st.Players[mySeat].Stack
	}
	out := AgentStateView{
		HandID:           st.HandID,
		Street:           string(st.Street),
		Pot:              st.Pot,
		CommunityCards:   community,
		CurrentActorSeat: st.Players[st.CurrentActor].Seat,
		TurnID:           turnID,
		ActionTimeoutMS:  int64(st.ActionTimeout.Milliseconds()),
		MySeat:           mySeat,
		MyBalance:        myBalance,
		MyHoleCards:      myCards,
		Seats:            seats,
	}
	if includeOthersHole {
		// Intentionally no-op here for now; showdown payload handles full reveal.
	}
	return out
}

func BuildPublicState(st *game.TableState) PublicStateView {
	community := make([]string, 0, len(st.Community))
	for _, c := range st.Community {
		community = append(community, c.String())
	}

	seats := make([]SeatView, 0, len(st.Players))
	for i, p := range st.Players {
		if p == nil {
			continue
		}
		toCall := st.CurrentBet - st.RoundBets[i]
		if toCall < 0 {
			toCall = 0
		}
		var lastAmount *int64
		if st.RoundBets[i] > 0 {
			v := st.RoundBets[i]
			lastAmount = &v
		}
		seats = append(seats, SeatView{
			SeatID:             p.Seat,
			AgentID:            p.ID,
			Stack:              p.Stack,
			StreetContribution: st.RoundBets[i],
			ToCall:             toCall,
			LastAction:         string(p.LastAction),
			LastActionAmount:   lastAmount,
			IsActive:           !p.Folded,
		})
	}
	return PublicStateView{
		HandID:           st.HandID,
		Street:           string(st.Street),
		Pot:              st.Pot,
		CommunityCards:   community,
		CurrentActorSeat: st.Players[st.CurrentActor].Seat,
		ActionTimeoutMS:  int64(st.ActionTimeout.Milliseconds()),
		Seats:            seats,
	}
}
