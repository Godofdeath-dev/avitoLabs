package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

			if tt.wantErr {
				require.Error(t, err)
				var valErr *trip.ValidationError
				require.ErrorAs(t, err, &valErr, "expected ValidationError")
				assert.Equal(t, tt.wantField, valErr.Field)
			} else {
				require.NoError(t, err)
				assert.Equal(t, trip.StatusActive, got.Status)
			}
		})
	}
}

func TestTrip_Finish(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	coord := trip.Coordinates{Latitude: 55, Longitude: 37}

	tr, err := trip.NewTrip(uuid.New(), uuid.New(), 1000, coord, coord, now)
	require.NoError(t, err)

	// Сценарий 3: завершение активной поездки — успех
	finishTime := now.Add(15 * time.Minute)
	err = tr.Finish(finishTime)
	require.NoError(t, err)
	assert.Equal(t, trip.StatusCompleted, tr.Status)
	assert.NotNil(t, tr.FinishedAt)
	assert.True(t, tr.FinishedAt.Equal(finishTime))

	// Сценарий 4: повторное завершение -> ErrTripCompleted
	err = tr.Finish(finishTime)
	require.ErrorIs(t, err, trip.ErrTripCompleted)
}
