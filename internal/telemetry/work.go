package telemetry

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocraft/work"
)

const (
	jobTurnStarted   = "telemetry.turn_started"
	jobTurnCompleted = "telemetry.turn_completed"
	jobToolStarted   = "telemetry.tool_started"
	jobToolCompleted = "telemetry.tool_completed"
	jobCompaction    = "telemetry.compaction"
	jobRetry         = "telemetry.retry"
	jobError         = "telemetry.error"
)

const (
	argTurnIndex     = "turn_index"
	argMessageCount  = "message_count"
	argProvider      = "provider"
	argModel         = "model"
	argToolName      = "tool_name"
	argArgsSize      = "args_size"
	argTrigger       = "trigger"
	argMsgCount      = "msg_count"
	argCharCount     = "char_count"
	argTokenEstimate = "token_estimate"
	argTokenThresh   = "token_threshold"
	argAttempt       = "attempt"
	argMax           = "max"
	argDelayMs       = "delay_ms"
	argError         = "error"
	argStage         = "stage"
)

// Enqueuer defines the enqueue contract for work-backed telemetry.
type Enqueuer interface {
	Enqueue(name string, args work.Q) (*work.Job, error)
}

// WorkQueueOptions controls how telemetry events are enqueued.
type WorkQueueOptions struct {
	Async        bool
	BufferSize   int
	BlockOnFull  bool
	EnqueueDelay time.Duration
}

// WorkTelemetry enqueues telemetry events into a gocraft/work queue.
// Use Async=true to keep the hot path non-blocking.
type WorkTelemetry struct {
	enqueuer   Enqueuer
	queue      chan workJob
	dropped    uint64
	dropOnFull bool
	wg         sync.WaitGroup
}

type workJob struct {
	name string
	args work.Q
}

// NewWorkTelemetry creates a telemetry sink that enqueues work jobs.
func NewWorkTelemetry(enqueuer Enqueuer, opts WorkQueueOptions) *WorkTelemetry {
	if opts.BufferSize <= 0 {
		opts.BufferSize = 256
	}
	t := &WorkTelemetry{
		enqueuer:   enqueuer,
		dropOnFull: !opts.BlockOnFull,
	}
	if opts.Async {
		t.queue = make(chan workJob, opts.BufferSize)
		t.wg.Add(1)
		go t.run(opts)
	}
	return t
}

// Dropped returns the number of events dropped due to a full queue.
func (t *WorkTelemetry) Dropped() uint64 {
	return atomic.LoadUint64(&t.dropped)
}

func (t *WorkTelemetry) enqueue(name string, args work.Q) {
	if t.queue == nil {
		_, _ = t.enqueuer.Enqueue(name, args)
		return
	}
	job := workJob{name: name, args: args}
	if t.dropOnFull {
		if optsNonBlockingSend(t.queue, job) {
			return
		}
		atomic.AddUint64(&t.dropped, 1)
		return
	}
	t.queue <- job
}

func (t *WorkTelemetry) run(opts WorkQueueOptions) {
	defer t.wg.Done()
	for job := range t.queue {
		_, _ = t.enqueuer.Enqueue(job.name, job.args)
		if opts.EnqueueDelay > 0 {
			time.Sleep(opts.EnqueueDelay)
		}
	}
}

func optsNonBlockingSend(ch chan workJob, job workJob) bool {
	select {
	case ch <- job:
		return true
	default:
		return false
	}
}

func (t *WorkTelemetry) TurnStarted(ctx TurnContext) {
	t.enqueue(jobTurnStarted, work.Q{
		argTurnIndex:    int64(ctx.TurnIndex),
		argMessageCount: int64(ctx.MessageCount),
		argProvider:     ctx.Provider,
		argModel:        ctx.Model,
	})
}

func (t *WorkTelemetry) TurnCompleted(ctx TurnContext) {
	t.enqueue(jobTurnCompleted, work.Q{
		argTurnIndex:    int64(ctx.TurnIndex),
		argMessageCount: int64(ctx.MessageCount),
		argProvider:     ctx.Provider,
		argModel:        ctx.Model,
	})
}

func (t *WorkTelemetry) ToolCallStarted(ctx ToolCallContext) {
	t.enqueue(jobToolStarted, work.Q{
		argToolName: ctx.ToolName,
		argArgsSize: int64(ctx.ArgsSize),
	})
}

func (t *WorkTelemetry) ToolCallCompleted(ctx ToolCallContext) {
	t.enqueue(jobToolCompleted, work.Q{
		argToolName: ctx.ToolName,
		argArgsSize: int64(ctx.ArgsSize),
	})
}

func (t *WorkTelemetry) Compaction(ctx CompactionContext) {
	t.enqueue(jobCompaction, work.Q{
		argTrigger:       ctx.Trigger,
		argMsgCount:      int64(ctx.MsgCount),
		argCharCount:     int64(ctx.CharCount),
		argTokenEstimate: int64(ctx.TokenEstimate),
		argTokenThresh:   int64(ctx.TokenThreshold),
	})
}

func (t *WorkTelemetry) Retry(ctx RetryContext) {
	t.enqueue(jobRetry, work.Q{
		argAttempt: int64(ctx.Attempt),
		argMax:     int64(ctx.Max),
		argError:   ctx.Error,
		argDelayMs: int64(ctx.Delay.Milliseconds()),
	})
}

func (t *WorkTelemetry) Error(ctx ErrorContext) {
	t.enqueue(jobError, work.Q{
		argStage: ctx.Stage,
		argError: ctx.Error,
	})
}

// WorkConsumer processes telemetry jobs and forwards them to a sink.
type WorkConsumer struct {
	Sink Telemetry
}

func NewWorkConsumer(sink Telemetry) *WorkConsumer {
	return &WorkConsumer{Sink: sink}
}

func (c *WorkConsumer) TurnStarted(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := TurnContext{
		TurnIndex:    int(job.ArgInt64(argTurnIndex)),
		MessageCount: int(job.ArgInt64(argMessageCount)),
		Provider:     job.ArgString(argProvider),
		Model:        job.ArgString(argModel),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.TurnStarted(ctx)
	return nil
}

func (c *WorkConsumer) TurnCompleted(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := TurnContext{
		TurnIndex:    int(job.ArgInt64(argTurnIndex)),
		MessageCount: int(job.ArgInt64(argMessageCount)),
		Provider:     job.ArgString(argProvider),
		Model:        job.ArgString(argModel),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.TurnCompleted(ctx)
	return nil
}

func (c *WorkConsumer) ToolCallStarted(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := ToolCallContext{
		ToolName: job.ArgString(argToolName),
		ArgsSize: int(job.ArgInt64(argArgsSize)),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.ToolCallStarted(ctx)
	return nil
}

func (c *WorkConsumer) ToolCallCompleted(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := ToolCallContext{
		ToolName: job.ArgString(argToolName),
		ArgsSize: int(job.ArgInt64(argArgsSize)),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.ToolCallCompleted(ctx)
	return nil
}

func (c *WorkConsumer) Compaction(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := CompactionContext{
		Trigger:        job.ArgString(argTrigger),
		MsgCount:       int(job.ArgInt64(argMsgCount)),
		CharCount:      int(job.ArgInt64(argCharCount)),
		TokenEstimate:  int(job.ArgInt64(argTokenEstimate)),
		TokenThreshold: int(job.ArgInt64(argTokenThresh)),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.Compaction(ctx)
	return nil
}

func (c *WorkConsumer) Retry(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := RetryContext{
		Attempt: int(job.ArgInt64(argAttempt)),
		Max:     int(job.ArgInt64(argMax)),
		Error:   job.ArgString(argError),
		Delay:   time.Duration(job.ArgInt64(argDelayMs)) * time.Millisecond,
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.Retry(ctx)
	return nil
}

func (c *WorkConsumer) Error(job *work.Job) error {
	if c.Sink == nil {
		return nil
	}
	ctx := ErrorContext{
		Stage: job.ArgString(argStage),
		Error: job.ArgString(argError),
	}
	if err := job.ArgError(); err != nil {
		return err
	}
	c.Sink.Error(ctx)
	return nil
}
