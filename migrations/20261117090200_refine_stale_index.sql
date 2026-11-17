-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY trips_status_position_time_idx
    ON trips (status, (COALESCE(last_position_at, started_at)), id);
DROP INDEX CONCURRENTLY trips_active_position_time_idx;

-- +goose Down
CREATE INDEX CONCURRENTLY trips_active_position_time_idx
    ON trips ((COALESCE(last_position_at, started_at)), id)
    WHERE status = 'active';
DROP INDEX CONCURRENTLY trips_status_position_time_idx;
