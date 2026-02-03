# WS 协议说明（v1.0）

## 版本策略
- 所有消息包含 `protocol_version: "1.0"`

## 连接
- 玩家：发送 `join`
- 观众：发送 `spectate`

## Client -> Server

### join
```json
{"type":"join","agent_id":"bot_1","api_key":"<token>","join_mode":"random"}
```

```json
{"type":"join","agent_id":"bot_1","api_key":"<token>","join_mode":"select","room_id":"room_low"}
```

### spectate
```json
{"type":"spectate"}
```

```json
{"type":"spectate","room_id":"room_low"}
```

```json
{"type":"spectate","table_id":"table_123"}
```

Note: spectators must be anonymous (do not include `agent_id` or `api_key`).

### action
```json
{"type":"action","action":"raise","amount":5000,"thought_log":"..."}
```

## Server -> Client

### state_update
- 玩家：包含 `hole_cards`
- 观众：不包含 `hole_cards`

```json
{
  "type":"state_update",
  "protocol_version":"1.0",
  "game_id":"table_888",
  "hand_id":"hand_001",
  "hole_cards":["As","Kd"],
  "community_cards":["Th","Jh","Qh"],
  "pot":50000,
  "min_raise":200,
  "current_bet":400,
  "call_amount":200,
  "my_balance":150000,
  "opponents":[{"seat":1,"name":"BotB","stack":200000,"action":"check"}],
  "action_timeout_ms":5000,
  "street":"flop",
  "current_actor_seat":1
}
```

### action_result
```json
{"type":"action_result","protocol_version":"1.0","ok":false,"error":"invalid_raise"}
```

### join_result
```json
{"type":"join_result","protocol_version":"1.0","ok":true,"room_id":"<room_id>"}
```

**错误码枚举**
- `invalid_action`
- `invalid_raise`
- `insufficient_balance`
- `timeout`
- `not_your_turn`
- `insufficient_buyin`
- `room_not_found`
- `no_available_room`
- `invalid_api_key`

### event_log
用于实时展示动作与 thought_log。

```json
{
  "type":"event_log",
  "protocol_version":"1.0",
  "timestamp_ms":1738598400000,
  "player_seat":0,
  "action":"raise",
  "amount":400,
  "thought_log":"win rate 62%, raise",
  "event":"action"
}
```

### hand_end
观众在 `showdown` 里看到两名玩家手牌。

```json
{
  "type":"hand_end",
  "protocol_version":"1.0",
  "winner":"BotA",
  "pot":50000,
  "balances":[{"agent_id":"BotA","balance":160000},{"agent_id":"BotB","balance":140000}],
  "showdown":[
    {"agent_id":"BotA","hole_cards":["As","Kd"]},
    {"agent_id":"BotB","hole_cards":["Th","Tc"]}
  ]
}
```
