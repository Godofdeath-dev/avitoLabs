package config

import (
	"fmt"
	"time"
)

func (c *Config) loadPositions() error {
	var err error
	if c.PositionCheckInterval, err = getDurationOrDefault("POSITION_CHECK_INTERVAL", 30*time.Second); err != nil {
		return err
	}
	if c.PositionStaleAfter, err = getDurationOrDefault("POSITION_STALE_AFTER", 60*time.Second); err != nil {
		return err
	}
	if c.PushRequestTimeout, err = getDurationOrDefault("PUSH_REQUEST_TIMEOUT", time.Second); err != nil {
		return err
	}
	workers, err := getInt32OrDefault("POSITION_WORKERS", 4)
	if err != nil {
		return err
	}
	queueSize, err := getInt32OrDefault("POSITION_QUEUE_SIZE", 100)
	if err != nil {
		return err
	}
	if workers <= 0 || queueSize <= 0 {
		return fmt.Errorf("POSITION_WORKERS and POSITION_QUEUE_SIZE must be positive")
	}
	c.PositionWorkers = int(workers)
	c.PositionQueueSize = int(queueSize)
	c.PushHTTPURL, err = getEnvRequired("PUSH_HTTP_URL")
	return err
}
