package trip_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Godofdeath-dev/avitoLabs/internal/domain/trip"
)

func TestNewPosition(t *testing.T) {
	t.Parallel()
	const coordinatesField = "coordinates"
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	cases := []struct {
		name        string
		coordinates trip.Coordinates
		recordedAt  time.Time
		field       string
	}{
		{name: "zero coordinates", recordedAt: now},
		{name: "boundary coordinates", coordinates: trip.Coordinates{Latitude: -90, Longitude: 180}, recordedAt: now},
		{name: "delayed position", recordedAt: now.Add(-24 * time.Hour)},
		{name: "exactly one minute ahead", recordedAt: now.Add(time.Minute)},
		{name: "too far ahead", recordedAt: now.Add(time.Minute + time.Nanosecond), field: "recorded_at"},
		{name: "missing timestamp", field: "recorded_at"},
		{name: "latitude out of range", coordinates: trip.Coordinates{Latitude: 91}, recordedAt: now, field: coordinatesField},
		{name: "longitude out of range", coordinates: trip.Coordinates{Longitude: -181}, recordedAt: now, field: coordinatesField},
		{name: "NaN", coordinates: trip.Coordinates{Latitude: math.NaN()}, recordedAt: now, field: coordinatesField},
		{name: "infinity", coordinates: trip.Coordinates{Longitude: math.Inf(1)}, recordedAt: now, field: coordinatesField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := trip.NewPosition(id, tc.coordinates, tc.recordedAt, now)
			if tc.field != "" {
				var validation *trip.ValidationError
				require.ErrorAs(t, err, &validation)
				require.Equal(t, tc.field, validation.Field)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, id, got.TripID)
			require.Equal(t, tc.coordinates, got.Coordinates)
			require.True(t, got.RecordedAt.Equal(tc.recordedAt))
			require.Equal(t, time.UTC, got.RecordedAt.Location())
		})
	}
}
