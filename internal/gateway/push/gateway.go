package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/Godofdeath-dev/avitoLabs/internal/model"
)

// HTTPError отличает ответ с ошибочным статусом от ошибки соединения.
type HTTPError struct{ Status int }

func (e *HTTPError) Error() string { return fmt.Sprintf("push returned HTTP %d", e.Status) }

type Gateway struct {
	client     *http.Client
	url        string
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

func New(base string, timeout time.Duration, tracer trace.Tracer) (*Gateway, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse push URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("PUSH_HTTP_URL must be an absolute HTTP(S) URL")
	}
	// У клиента свой транспорт: его соединения можно закрыть при остановке сервиса.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	return &Gateway{client: &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, url: u.JoinPath("api/v1/push").String(), tracer: tracer, propagator: propagation.TraceContext{}}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, t model.StaleTrip) (err error) {
	ctx, span := g.tracer.Start(ctx, "push.REQUEST_POSITION", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, "push failed")
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		}
	}()
	payload := struct {
		RequestID   string            `json:"request_id"`
		RecipientID uuid.UUID         `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}{RequestID: uuid.NewString(), RecipientID: t.DriverID, Kind: "REQUEST_POSITION", Data: map[string]string{"trip_id": t.ID.String()}}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode push: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Передаём контекст трейса соседнему сервису через HTTP-заголовки.
	g.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Дочитываем небольшой ответ для повторного использования соединения.
	_, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	if resp.StatusCode != http.StatusAccepted {
		return &HTTPError{Status: resp.StatusCode}
	}
	if readErr != nil {
		return fmt.Errorf("read push response: %w", readErr)
	}
	return nil
}
func (g *Gateway) Close() { g.client.CloseIdleConnections() }
