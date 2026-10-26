package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

func TestNewTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	validCoord := trip.Coordinates{Latitude: 55.75, Longitude: 37.61}

	tests := []struct {
		name      string
		price     int64
		start     trip.Coordinates
		end       trip.Coordinates
		wantErr   bool
		wantField string
	}{
		{
			name:    "valid trip",
			price:   1500,
			start:   validCoord,
			end:     validCoord,
			wantErr: false,
		},
		{
			name:      "zero price",
			price:     0,
			start:     validCoord,
			end:       validCoord,
			wantErr:   true,
			wantField: "price",
		},
		{
			name:      "negative price",
			price:     -100,
			start:     validCoord,
			end:       validCoord,
			wantErr:   true,
			wantField: "price",
		},
		{
			name:      "invalid start latitude",
			price:     500,
			start:     trip.Coordinates{Latitude: 95, Longitude: 30},
			end:       validCoord,
			wantErr:   true,
			wantField: "start_point",
		},
		{
			name:      "invalid end longitude",
			price:     500,
			start:     validCoord,
			end:       trip.Coordinates{Latitude: 55, Longitude: 190},
			wantErr:   true,
			wantField: "end_point",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := trip.NewTrip(uuid.New(), uuid.New(), tt.price, tt.start, tt.end, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTrip() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var valErr *trip.ValidationError
				if !errors.As(err, &valErr) {
					t.Fatalf("expected ValidationError, got %T", err)
				}
				if valErr.Field != tt.wantField {
					t.Errorf("field = %s, want %s", valErr.Field, tt.wantField)
				}
				return
			}
			if got.Status != trip.StatusActive {
				t.Errorf("status = %s, want %s", got.Status, trip.StatusActive)
			}
		})
	}
}

func TestTrip_Finish(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	coord := trip.Coordinates{Latitude: 55, Longitude: 37}
	tr, err := trip.NewTrip(uuid.New(), uuid.New(), 1000, coord, coord, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Сценарий 3: завершение активной поездки — успех
	finishTime := now.Add(15 * time.Minute)
	if err := tr.Finish(finishTime); err != nil {
		t.Fatalf("Finish() unexpected error = %v", err)
	}
	if tr.Status != trip.StatusCompleted {
		t.Errorf("status = %s, want %s", tr.Status, trip.StatusCompleted)
	}
	if tr.FinishedAt == nil || !tr.FinishedAt.Equal(finishTime) {
		t.Errorf("finishedAt = %v, want %v", tr.FinishedAt, finishTime)
	}

	// Сценарий 4: повторное завершение -> ErrTripCompleted
	if err := tr.Finish(finishTime); !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("expected ErrTripCompleted, got %v", err)
	}
}
