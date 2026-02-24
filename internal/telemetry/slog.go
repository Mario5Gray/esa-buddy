package telemetry

import "log/slog"

// SlogAdapter emits telemetry events as structured slog records.
type SlogAdapter struct {
	logger *slog.Logger
}

func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

func (s *SlogAdapter) TurnStarted(ctx TurnContext) {
	if s.logger == nil {
		return
	}
	s.logger.Info("turn.start",
		"turn_index", ctx.TurnIndex,
		"message_count", ctx.MessageCount,
		"provider", ctx.Provider,
		"model", ctx.Model,
	)
}

func (s *SlogAdapter) TurnCompleted(ctx TurnContext) {
	if s.logger == nil {
		return
	}
	s.logger.Info("turn.complete",
		"turn_index", ctx.TurnIndex,
		"message_count", ctx.MessageCount,
		"provider", ctx.Provider,
		"model", ctx.Model,
	)
}

func (s *SlogAdapter) ToolCallStarted(ctx ToolCallContext) {
	if s.logger == nil {
		return
	}
	s.logger.Info("tool.start",
		"tool", ctx.ToolName,
		"args_size", ctx.ArgsSize,
	)
}

func (s *SlogAdapter) ToolCallCompleted(ctx ToolCallContext) {
	if s.logger == nil {
		return
	}
	s.logger.Info("tool.complete",
		"tool", ctx.ToolName,
		"args_size", ctx.ArgsSize,
	)
}

func (s *SlogAdapter) Compaction(ctx CompactionContext) {
	if s.logger == nil {
		return
	}
	s.logger.Info("compaction",
		"trigger", ctx.Trigger,
		"msg_count", ctx.MsgCount,
		"char_count", ctx.CharCount,
		"token_estimate", ctx.TokenEstimate,
		"token_threshold", ctx.TokenThreshold,
	)
}

func (s *SlogAdapter) Retry(ctx RetryContext) {
	if s.logger == nil {
		return
	}
	s.logger.Warn("retry",
		"attempt", ctx.Attempt,
		"max", ctx.Max,
		"error", ctx.Error,
		"delay_ms", ctx.Delay.Milliseconds(),
	)
}

func (s *SlogAdapter) Error(ctx ErrorContext) {
	if s.logger == nil {
		return
	}
	s.logger.Error("error", "stage", ctx.Stage, "error", ctx.Error)
}
