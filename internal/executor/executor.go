package executor

import "github.com/meain/esa/internal/agent"

type Executor interface {
	Execute(askLevel string, fc agent.FunctionConfig, args string) (bool, string, string, string, error)
}
