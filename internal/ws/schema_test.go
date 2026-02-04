package ws

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestWSProtocolSchema(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	data, err := os.ReadFile("../../api/schema/ws_v1.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if err := compiler.AddResource("ws_v1.schema.json", strings.NewReader(string(data))); err != nil {
		t.Fatalf("add resource: %v", err)
	}
	schema, err := compiler.Compile("ws_v1.schema.json")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	samples := []string{
		`{"type":"state_update","protocol_version":"1.0","game_id":"table","hand_id":"hand","community_cards":[],"pot":0,"min_raise":200,"my_balance":1000,"opponents":[],"action_timeout_ms":5000,"street":"preflop","current_actor_seat":0}`,
		`{"type":"action_result","protocol_version":"1.0","request_id":"req_1","ok":true}`,
		`{"type":"event_log","protocol_version":"1.0","timestamp_ms":1,"player_seat":0,"action":"check","event":"action"}`,
		`{"type":"hand_end","protocol_version":"1.0","winner":"p1","pot":100,"balances":[{"agent_id":"p1","balance":200}]}`,
	}

	for i, s := range samples {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatalf("unmarshal sample %d: %v", i, err)
		}
		if err := schema.Validate(v); err != nil {
			t.Fatalf("schema validate sample %d: %v", i, err)
		}
	}
}
