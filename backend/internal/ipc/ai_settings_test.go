package ipc

import "testing"

func TestExtractLocalAISettings(t *testing.T) {
	patch := map[string]any{
		"aiEnabled": true,
		"aiBaseUrl": "http://127.0.0.1:1234/v1",
		"aiModel":   "local",
		"theme":     "dark",
	}
	local := extractLocalAISettings(patch)
	if local["aiEnabled"] != true || local["aiBaseUrl"] != "http://127.0.0.1:1234/v1" || local["aiModel"] != "local" {
		t.Fatalf("local patch = %#v", local)
	}
	if _, ok := patch["aiEnabled"]; ok {
		t.Fatal("aiEnabled remained in the synced settings patch")
	}
	if _, ok := patch["aiBaseUrl"]; ok || patch["theme"] != "dark" {
		t.Fatalf("synced patch = %#v", patch)
	}
}
