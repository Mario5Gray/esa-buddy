package security

import (
	"fmt"

	"github.com/meain/esa/internal/utils"
)

// HumanGate prompts the user for explicit approval.
type HumanGate struct{}

func (HumanGate) Evaluate(intent ToolIntent) (Decision, *SignedIntent, error) {
	response := utils.Confirm(fmt.Sprintf("Execute tool %s with args %s?", intent.ToolName, intent.ArgsJSON))
	if response.Approved {
		return Allow, nil, nil
	}
	return Deny, nil, nil
}
