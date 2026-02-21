package executor

import (
	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/tools"
)

type DefaultExecutor struct{}

func (DefaultExecutor) Execute(askLevel string, fc agent.FunctionConfig, args string) (bool, string, string, string, error) {
	return tools.ExecuteFunction(askLevel, fc, args)
}
