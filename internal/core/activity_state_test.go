package core

import (
	"testing"
	"time"
)

func TestMysis_ActivityStateAccessor(t *testing.T) {
	m := &Mysis{activityState: ActivityStateTraveling}

	if m.ActivityState() != ActivityStateTraveling {
		t.Errorf("expected traveling, got %v", m.ActivityState())
	}
}

func TestMysis_SetActivityLLM(t *testing.T) {
	m := &Mysis{activityState: ActivityStateIdle}
	m.setActivity(ActivityStateLLMCall, time.Time{})

	if m.activityState != ActivityStateLLMCall {
		t.Errorf("expected llm_call, got %v", m.activityState)
	}
}

func TestMysis_SetActivityMCP(t *testing.T) {
	m := &Mysis{activityState: ActivityStateIdle}
	m.setActivity(ActivityStateMCPCall, time.Time{})

	if m.activityState != ActivityStateMCPCall {
		t.Errorf("expected mcp_call, got %v", m.activityState)
	}
}

func TestActivityStateConstants(t *testing.T) {
	// Verify new constants exist and have expected values
	if ActivityStateLLMCall != "llm_call" {
		t.Errorf("expected llm_call, got %v", ActivityStateLLMCall)
	}
	if ActivityStateMCPCall != "mcp_call" {
		t.Errorf("expected mcp_call, got %v", ActivityStateMCPCall)
	}
}
