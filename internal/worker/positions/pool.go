package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

type Config struct {
	Interval, StaleAfter, RequestTimeout time.Duration
	Workers, QueueSize                   int
}
type task struct {
	trip   model.StaleTrip
	link   trace.SpanContext
	cancel context.CancelFunc
}

// Pool объединяет сканер, ограниченную очередь и фиксированное число воркеров.
type Pool struct {
	repo    repository
	push    pushGateway
	metrics metrics
	clock   clock
	cfg     Config
	logger  *slog.Logger
	tracer  trace.Tracer
	queue   chan *task
	mu      sync.Mutex
	// pending хранит задачи в очереди и в обработке; доступ защищён mu.
	pending                 map[uuid.UUID]*task
	active                  int64
	scanCancel, forceCancel context.CancelFunc
	done                    chan struct{}
	once                    sync.Once
}

func New(repo repository, push pushGateway, m metrics, c clock, cfg Config, l *slog.Logger, tracer trace.Tracer) (*Pool, error) {
	if cfg.Workers <= 0 || cfg.QueueSize <= 0 || cfg.Interval <= 0 || cfg.StaleAfter <= 0 || cfg.RequestTimeout <= 0 {
		return nil, fmt.Errorf("positions pool requires positive sizes and durations")
	}
	return &Pool{repo: repo, push: push, metrics: m, clock: c, cfg: cfg, logger: l, tracer: tracer, queue: make(chan *task, cfg.QueueSize), pending: make(map[uuid.UUID]*task), done: make(chan struct{})}, nil
}

func (p *Pool) Start(ctx context.Context) {
	// Повторный вызов Start не создаст ещё один набор горутин.
	p.once.Do(func() {
		// Сканер останавливаем отдельно, чтобы воркеры успели дочитать очередь.
		scanCtx, scanCancel := context.WithCancel(ctx)
		workCtx, forceCancel := context.WithCancel(ctx)
		p.scanCancel = scanCancel
		p.forceCancel = forceCancel
		var wg sync.WaitGroup
		wg.Add(p.cfg.Workers + 1)
		// Каждый воркер живёт до остановки пула и обрабатывает задачи по одной.
		for i := 0; i < p.cfg.Workers; i++ {
			go func() { defer wg.Done(); p.work(workCtx) }()
		}
		// Только сканер пишет в канал, поэтому он же закрывает его при выходе.
		go func() { defer wg.Done(); defer close(p.queue); p.scan(scanCtx) }()
		go func() {
			wg.Wait()
			// После принудительной отмены освобождаем оставшиеся ссылки на задачи.
			for range p.queue {
			}
			p.mu.Lock()
			clear(p.pending)
			p.mu.Unlock()
			close(p.done)
		}()
	})
}

// Stop прекращает постановку задач и ждёт воркеров в пределах общего бюджета.
func (p *Pool) Stop(ctx context.Context) error {
	p.scanCancel()
	select {
	case <-p.done:
		p.forceCancel()
		return nil
	case <-ctx.Done():
		p.forceCancel()
		return ctx.Err()
	}
}
func (p *Pool) Done() <-chan struct{} { return p.done }
func (p *Pool) Snapshot() (int64, int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int64(len(p.queue)), p.active
}

// Cancel делает задачу неактуальной; уже выполняемый HTTP-запрос тоже отменяется.
func (p *Pool) Cancel(id uuid.UUID) {
	p.mu.Lock()
	t := p.pending[id]
	var cancel context.CancelFunc
	if t != nil {
		cancel = t.cancel
	}
	delete(p.pending, id)
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (p *Pool) scan(ctx context.Context) {
	p.logger.InfoContext(ctx, "positions scanner started")
	defer p.logger.InfoContext(ctx, "positions scanner stopped")
	ticker := p.clock.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			// Обход выполняется здесь же: два обхода не могут идти одновременно.
			p.scanOnce(ctx)
		}
	}
}

func (p *Pool) scanOnce(ctx context.Context) {
	ctx, span := p.tracer.Start(ctx, "positions.scan")
	defer span.End()
	items, err := p.repo.FindStale(ctx, p.clock.Now(), p.cfg.StaleAfter)
	if err != nil {
		if ctx.Err() == nil {
			p.logger.ErrorContext(ctx, "scan stale trips failed", "error", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "scan failed")
			span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		}
		return
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		result := p.enqueue(item, span.SpanContext())
		p.metrics.StaleCheck(ctx, result)
		if result == "dropped" {
			p.metrics.PositionRequest(ctx, result, 0)
			p.logger.WarnContext(ctx, "position queue full", "trip_id", item.ID)
		}
	}
}

func (p *Pool) enqueue(item model.StaleTrip, link trace.SpanContext) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pending[item.ID]; ok {
		return "duplicate"
	}
	t := &task{trip: item, link: link}
	p.pending[item.ID] = t
	// default не даёт сканеру зависнуть, если очередь заполнена.
	select {
	case p.queue <- t:
		return "queued"
	default:
		delete(p.pending, item.ID)
		return "dropped"
	}
}

func (p *Pool) work(ctx context.Context) {
	p.logger.InfoContext(ctx, "positions worker started")
	defer p.logger.InfoContext(ctx, "positions worker stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-p.queue:
			// После закрытия канала сначала дочитываем буфер, затем выходим.
			if !ok {
				return
			}
			p.process(ctx, t)
		}
	}
}

func (p *Pool) process(ctx context.Context, t *task) {
	if ctx.Err() != nil {
		return
	}
	p.mu.Lock()
	// Отменённая задача может ещё лежать в канале, но отправлять её уже нельзя.
	if p.pending[t.trip.ID] != t {
		p.mu.Unlock()
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	t.cancel = cancel
	p.active++
	p.mu.Unlock()
	defer func() {
		cancel()
		p.mu.Lock()
		p.active--
		// Старая задача не должна удалить новую задачу той же поездки.
		if p.pending[t.trip.ID] == t {
			delete(p.pending, t.trip.ID)
		}
		p.mu.Unlock()
	}()
	// Обработка получает свой трейс, связанный со сканером через span link.
	requestCtx, span := p.tracer.Start(requestCtx, "positions.process", trace.WithNewRoot(), trace.WithLinks(trace.Link{SpanContext: t.link}))
	defer span.End()
	start := p.clock.Now()
	err := p.push.RequestPosition(requestCtx, t.trip)
	result := "success"
	if err != nil {
		result = "error"
		p.logger.WarnContext(requestCtx, "request position failed", "trip_id", t.trip.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "push failed")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
	}
	p.metrics.PositionRequest(requestCtx, result, p.clock.Now().Sub(start).Seconds())
}
