package security

import "testing"

type stubGate struct {
	decision Decision
}

func (s stubGate) Evaluate(intent ToolIntent) (Decision, *SignedIntent, error) {
	return s.decision, nil, nil
}

func TestGateChainOrder(t *testing.T) {
	chain := GateChain{
		Gates: []Gate{
			stubGate{decision: Abstain},
			stubGate{decision: Allow},
			stubGate{decision: Deny}, // should not be reached
		},
	}

	decision, _, err := chain.Evaluate(ToolIntent{ToolName: "x", ArgsJSON: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != Allow {
		t.Fatalf("expected Allow, got %v", decision)
	}
}

func TestGateChainDefaultDeny(t *testing.T) {
	chain := GateChain{
		Gates: []Gate{
			stubGate{decision: Abstain},
			stubGate{decision: Abstain},
		},
	}

	decision, _, err := chain.Evaluate(ToolIntent{ToolName: "x", ArgsJSON: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != Deny {
		t.Fatalf("expected Deny, got %v", decision)
	}
}

func TestDenyGate(t *testing.T) {
	gate := DenyGate{}
	decision, _, err := gate.Evaluate(ToolIntent{ToolName: "x", ArgsJSON: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != Deny {
		t.Fatalf("expected Deny, got %v", decision)
	}
}
