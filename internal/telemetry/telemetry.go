package telemetry

import "time"

// TurnContext describes the current conversation position and model context.
type TurnContext struct {
	TurnIndex    int
	MessageCount int
	Provider     string
	Model        string
}

type ToolCallContext struct {
	ToolName string
	ArgsSize int
}

type CompactionContext struct {
	Trigger        string
	MsgCount       int
	CharCount      int
	TokenEstimate  int
	TokenThreshold int
}

type RetryContext struct {
	Attempt int
	Max     int
	Error   string
	Delay   time.Duration
}

type ErrorContext struct {
	Stage string
	Error string
}

// Telemetry captures instrumentation events without binding to a specific backend.
type Telemetry interface {
	TurnStarted(ctx TurnContext)
	TurnCompleted(ctx TurnContext)
	ToolCallStarted(ctx ToolCallContext)
	ToolCallCompleted(ctx ToolCallContext)
	Compaction(ctx CompactionContext)
	Retry(ctx RetryContext)
	Error(ctx ErrorContext)
}

type Noop struct{}

func (Noop) TurnStarted(TurnContext)           {}
func (Noop) TurnCompleted(TurnContext)         {}
func (Noop) ToolCallStarted(ToolCallContext)   {}
func (Noop) ToolCallCompleted(ToolCallContext) {}
func (Noop) Compaction(CompactionContext)      {}
func (Noop) Retry(RetryContext)                {}
func (Noop) Error(ErrorContext)                {}

// Fanout forwards events to multiple telemetry sinks.
type Fanout struct {
	Sinks []Telemetry
}

func (f Fanout) TurnStarted(ctx TurnContext) {
	for _, sink := range f.Sinks {
		sink.TurnStarted(ctx)
	}
}

func (f Fanout) TurnCompleted(ctx TurnContext) {
	for _, sink := range f.Sinks {
		sink.TurnCompleted(ctx)
	}
}

func (f Fanout) ToolCallStarted(ctx ToolCallContext) {
	for _, sink := range f.Sinks {
		sink.ToolCallStarted(ctx)
	}
}

func (f Fanout) ToolCallCompleted(ctx ToolCallContext) {
	for _, sink := range f.Sinks {
		sink.ToolCallCompleted(ctx)
	}
}

func (f Fanout) Compaction(ctx CompactionContext) {
	for _, sink := range f.Sinks {
		sink.Compaction(ctx)
	}
}

func (f Fanout) Retry(ctx RetryContext) {
	for _, sink := range f.Sinks {
		sink.Retry(ctx)
	}
}

func (f Fanout) Error(ctx ErrorContext) {
	for _, sink := range f.Sinks {
		sink.Error(ctx)
	}
}
