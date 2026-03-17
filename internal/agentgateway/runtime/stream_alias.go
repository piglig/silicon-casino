package runtime

import agstream "ai-porker-arena/internal/agentgateway/stream"

type StreamEvent = agstream.StreamEvent
type EventBuffer = agstream.EventBuffer

func NewEventBuffer(max int) *EventBuffer {
	return agstream.NewEventBuffer(max)
}
