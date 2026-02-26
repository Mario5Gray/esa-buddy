package telemetry

import (
	"testing"

	"github.com/gocraft/work"
)

type fakeEnqueuer struct {
	count    int
	lastJob  string
	lastArgs work.Q
}

func (f *fakeEnqueuer) Enqueue(jobName string, args work.Q) (*work.Job, error) {
	f.count++
	f.lastJob = jobName
	f.lastArgs = args
	return &work.Job{Name: jobName, Args: args}, nil
}

type fakeSink struct {
	turnStarted TurnContext
	called      bool
}

func (f *fakeSink) TurnStarted(ctx TurnContext)       { f.turnStarted = ctx; f.called = true }
func (f *fakeSink) TurnCompleted(TurnContext)         {}
func (f *fakeSink) ToolCallStarted(ToolCallContext)   {}
func (f *fakeSink) ToolCallCompleted(ToolCallContext) {}
func (f *fakeSink) Compaction(CompactionContext)      {}
func (f *fakeSink) Retry(RetryContext)                {}
func (f *fakeSink) Error(ErrorContext)                {}

func TestWorkTelemetryEnqueuesTurnStarted(t *testing.T) {
	enqueuer := &fakeEnqueuer{}
	telemetry := NewWorkTelemetry(enqueuer, WorkQueueOptions{Async: false})

	telemetry.TurnStarted(TurnContext{
		TurnIndex:    3,
		MessageCount: 7,
		Provider:     "openai",
		Model:        "gpt-4o",
	})

	if enqueuer.count != 1 {
		t.Fatalf("expected 1 enqueue, got %d", enqueuer.count)
	}
	if enqueuer.lastJob != jobTurnStarted {
		t.Fatalf("expected job %q, got %q", jobTurnStarted, enqueuer.lastJob)
	}
	if enqueuer.lastArgs[argTurnIndex] != int64(3) {
		t.Fatalf("expected turn index arg, got %v", enqueuer.lastArgs[argTurnIndex])
	}
	if enqueuer.lastArgs[argMessageCount] != int64(7) {
		t.Fatalf("expected message count arg, got %v", enqueuer.lastArgs[argMessageCount])
	}
}

func TestWorkConsumerTurnStarted(t *testing.T) {
	sink := &fakeSink{}
	consumer := NewWorkConsumer(sink)
	job := &work.Job{
		Args: work.Q{
			argTurnIndex:    int64(2),
			argMessageCount: int64(5),
			argProvider:     "openai",
			argModel:        "gpt-4o",
		},
	}

	if err := consumer.TurnStarted(job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sink.called {
		t.Fatalf("expected sink to be called")
	}
	if sink.turnStarted.TurnIndex != 2 {
		t.Fatalf("expected turn index 2, got %d", sink.turnStarted.TurnIndex)
	}
}
