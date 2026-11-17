package positions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/Godofdeath-dev/avitoLabs/internal/model"
	"github.com/Godofdeath-dev/avitoLabs/internal/worker/positions"
)

const (
	queuedResult  = "queued"
	successResult = "success"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

type harness struct {
	pool            *positions.Pool
	repo            *Mockrepository
	push            *MockpushGateway
	clock           *clockwork.FakeClock
	checks, results chan string
	cancel          context.CancelFunc
}

func newHarness(t *testing.T, workers, size int, tracers ...trace.Tracer) *harness {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	push := NewMockpushGateway(ctrl)
	metrics := NewMockmetrics(ctrl)
	c := clockwork.NewFakeClockAt(time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	checks := make(chan string, 100)
	results := make(chan string, 100)
	metrics.EXPECT().StaleCheck(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s string) { checks <- s }).AnyTimes()
	metrics.EXPECT().PositionRequest(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, s string, _ float64) { results <- s }).AnyTimes()
	tracer := noop.NewTracerProvider().Tracer("test")
	if len(tracers) > 0 {
		tracer = tracers[0]
	}
	p, err := positions.New(repo, push, metrics, c, positions.Config{Interval: time.Minute, StaleAfter: time.Minute, RequestTimeout: time.Minute, Workers: workers, QueueSize: size}, slog.New(slog.NewTextHandler(io.Discard, nil)), tracer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	t.Cleanup(func() { cancel(); waitDone(t, p.Done()) })
	waitCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	if err := c.BlockUntilContext(waitCtx, 1); err != nil {
		t.Fatal(err)
	}
	return &harness{pool: p, repo: repo, push: push, clock: c, checks: checks, results: results, cancel: cancel}
}

func waitDone(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("component did not stop")
	}
}

func receive(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(3 * time.Second):
		t.Fatal("missing event")
		return ""
	}
}
func item() model.StaleTrip { return model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()} }
func stopPool(t *testing.T, h *harness) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.pool.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateTicks(t *testing.T) {
	h := newHarness(t, 1, 2)
	trip := item()
	started := make(chan struct{})
	release := make(chan struct{})
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), time.Minute).Return([]model.StaleTrip{trip}, nil).Times(2)
	h.push.EXPECT().RequestPosition(gomock.Any(), trip).DoAndReturn(func(ctx context.Context, _ model.StaleTrip) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	h.clock.Advance(time.Minute)
	if receive(t, h.checks) != queuedResult {
		t.Fatal("expected queued")
	}
	waitDone(t, started)
	h.clock.Advance(time.Minute)
	if receive(t, h.checks) != "duplicate" {
		t.Fatal("expected duplicate")
	}
	close(release)
	if receive(t, h.results) != successResult {
		t.Fatal("push failed")
	}
	stopPool(t, h)
}

func TestCoordinateCancelsBufferedTask(t *testing.T) {
	h := newHarness(t, 1, 2)
	first, second := item(), item()
	started := make(chan struct{})
	release := make(chan struct{})
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{first, second}, nil)
	h.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.StaleTrip) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	h.clock.Advance(time.Minute)
	receive(t, h.checks)
	receive(t, h.checks)
	waitDone(t, started)
	h.pool.Cancel(second.ID)
	close(release)
	receive(t, h.results)
	stopPool(t, h)
}

func TestOverflowDoesNotBlockScanner(t *testing.T) {
	h := newHarness(t, 1, 1)
	first, second, third := item(), item(), item()
	started := make(chan struct{})
	release := make(chan struct{})
	gomock.InOrder(h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{first}, nil), h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{second, third}, nil))
	h.push.EXPECT().RequestPosition(gomock.Any(), first).DoAndReturn(func(ctx context.Context, _ model.StaleTrip) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	h.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil)
	h.clock.Advance(time.Minute)
	receive(t, h.checks)
	waitDone(t, started)
	h.clock.Advance(time.Minute)
	if receive(t, h.checks) != queuedResult || receive(t, h.checks) != "dropped" {
		t.Fatal("overflow was not reported")
	}
	if receive(t, h.results) != "dropped" {
		t.Fatal("missing drop metric")
	}
	close(release)
	stopPool(t, h)
}

func TestFixedWorkerCountAndDrain(t *testing.T) {
	const workers = 3
	const jobs = 12
	h := newHarness(t, workers, jobs)
	items := make([]model.StaleTrip, jobs)
	for i := range items {
		items[i] = item()
	}
	started := make(chan string, jobs)
	release := make(chan struct{})
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return(items, nil)
	h.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, t model.StaleTrip) error {
		started <- t.ID.String()
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}).Times(jobs)
	h.clock.Advance(time.Minute)
	for range jobs {
		if receive(t, h.checks) != queuedResult {
			t.Fatal("task not queued")
		}
	}
	for range workers {
		receive(t, started)
	}
	queued, busy := h.pool.Snapshot()
	if busy != workers || queued != jobs-workers {
		t.Fatalf("snapshot=%d queued,%d busy", queued, busy)
	}
	select {
	case <-started:
		t.Fatal("more concurrent jobs than workers")
	default:
	}
	close(release)
	stopPool(t, h)
	for range jobs {
		if receive(t, h.results) != successResult {
			t.Fatal("queued task lost during shutdown")
		}
	}
}

func TestPushErrorDoesNotStopWorker(t *testing.T) {
	h := newHarness(t, 1, 2)
	first, second := item(), item()
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{first, second}, nil)
	gomock.InOrder(h.push.EXPECT().RequestPosition(gomock.Any(), first).Return(errors.New("push unavailable")), h.push.EXPECT().RequestPosition(gomock.Any(), second).Return(nil))
	h.clock.Advance(time.Minute)
	if receive(t, h.results) != "error" || receive(t, h.results) != successResult {
		t.Fatal("worker did not recover from error")
	}
	stopPool(t, h)
}

func TestCancellationStopsBusyWorkers(t *testing.T) {
	h := newHarness(t, 1, 1)
	trip := item()
	started := make(chan struct{})
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{trip}, nil)
	h.push.EXPECT().RequestPosition(gomock.Any(), trip).DoAndReturn(func(ctx context.Context, _ model.StaleTrip) error { close(started); <-ctx.Done(); return ctx.Err() })
	h.clock.Advance(time.Minute)
	waitDone(t, started)
	h.cancel()
	waitDone(t, h.pool.Done())
}

func TestShutdownBudgetCancelsPush(t *testing.T) {
	h := newHarness(t, 1, 1)
	trip := item()
	started := make(chan struct{})
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{trip}, nil)
	h.push.EXPECT().RequestPosition(gomock.Any(), trip).DoAndReturn(func(ctx context.Context, _ model.StaleTrip) error { close(started); <-ctx.Done(); return ctx.Err() })
	h.clock.Advance(time.Minute)
	waitDone(t, started)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.pool.Stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("stop=%v", err)
	}
	waitDone(t, h.pool.Done())
}

func TestWorkerSpanLinksScanner(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	h := newHarness(t, 1, 1, provider.Tracer("test"))
	trip := item()
	h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.StaleTrip{trip}, nil)
	h.push.EXPECT().RequestPosition(gomock.Any(), trip).Return(nil)
	h.clock.Advance(time.Minute)
	receive(t, h.results)
	stopPool(t, h)
	var scan, worker sdktrace.ReadOnlySpan
	for _, s := range recorder.Ended() {
		switch s.Name() {
		case "positions.scan":
			scan = s
		case "positions.process":
			worker = s
		}
	}
	if scan == nil || worker == nil {
		t.Fatal("missing scan or worker span")
	}
	if worker.Parent().IsValid() {
		t.Error("worker span must have no parent")
	}
	if len(worker.Links()) != 1 || !worker.Links()[0].SpanContext.Equal(scan.SpanContext()) {
		t.Error("worker is not linked to scan")
	}
	if worker.SpanContext().TraceID() == scan.SpanContext().TraceID() {
		t.Error("async worker must start a new trace")
	}
}

func TestSlowScansDoNotOverlap(t *testing.T) {
	h := newHarness(t, 1, 1)
	first := make(chan struct{})
	release := make(chan struct{})
	second := make(chan struct{})
	gomock.InOrder(h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ time.Time, _ time.Duration) ([]model.StaleTrip, error) {
		close(first)
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}), h.repo.EXPECT().FindStale(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, time.Time, time.Duration) ([]model.StaleTrip, error) {
		close(second)
		return nil, nil
	}))
	h.clock.Advance(time.Minute)
	waitDone(t, first)
	h.clock.Advance(5 * time.Minute)
	select {
	case <-second:
		t.Fatal("overlapping scan")
	default:
	}
	close(release)
	waitDone(t, second)
	stopPool(t, h)
}
