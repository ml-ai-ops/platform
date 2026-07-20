package store

import "testing"

func TestLifecycleActionsRouteToDedicatedTopics(t *testing.T) {
	cases := map[string]string{
		"pipeline.submitted": "mlaiops.pipeline.commands",
		"model.promoted":     "mlaiops.model.commands",
		"agent.deployed":     "mlaiops.agent.commands",
		"tool.registered":    "mlaiops.tool.commands",
		"connection.created": "mlaiops.connection.commands",
		"access.upserted":    "mlaiops.workspace.commands",
		"project.created":    "",
	}
	for action, expected := range cases {
		if got := topicForAction(action); got != expected {
			t.Errorf("%s: expected %q, got %q", action, expected, got)
		}
	}
}
