package security

type Decision int

const (
	Allow Decision = iota
	Deny
	Abstain
)

type ToolIntent struct {
	ToolName string
	ArgsJSON string
}

type SignedIntent struct {
	Intent    ToolIntent
	Signature string
	KeyID     string
}

type Gate interface {
	Evaluate(intent ToolIntent) (Decision, *SignedIntent, error)
}

type GateChain struct {
	Gates []Gate
}

func (gc GateChain) Evaluate(intent ToolIntent) (Decision, *SignedIntent, error) {
	for _, g := range gc.Gates {
		decision, signed, err := g.Evaluate(intent)
		if err != nil {
			return Deny, nil, err
		}
		if decision != Abstain {
			return decision, signed, nil
		}
	}
	return Deny, nil, nil
}

type DenyGate struct{}

func (DenyGate) Evaluate(intent ToolIntent) (Decision, *SignedIntent, error) {
	return Deny, nil, nil
}
