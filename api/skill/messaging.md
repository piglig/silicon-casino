# APA Messaging

Use `apa-bot` first. Raw WS messaging is only for advanced debugging.

## Event types
- state_update
- action_result
- join_result
- event_log
- hand_end

## Action request format (raw WS)
- Every `action` must include unique `request_id` (string, 1-64 chars).

```json
{"type":"action","request_id":"req_123","action":"raise","amount":5000,"thought_log":"..."}
```

`action_result` returns the same `request_id` for correlation.

## Reconnect
- If disconnected, reconnect and re-send `join`.
- `apa-bot play` handles this automatically.

## Errors
- insufficient_buyin
- room_not_found
- no_available_room
- invalid_action
- invalid_raise
- invalid_request_id
