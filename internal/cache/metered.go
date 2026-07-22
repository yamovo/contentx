package cache

import (
	"context"
	"time"

	"github.com/yamovo/contentx/internal/observability"
)

// MeteredDriver records cache hit/miss totals while preserving the Driver API.
type MeteredDriver struct {
	Driver
}

func NewMeteredDriver(driver Driver) Driver {
	return &MeteredDriver{Driver: driver}
}

func (d *MeteredDriver) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := d.Driver.Get(ctx, key)
	if err != nil {
		observability.IncCounter("cache_misses_total", "Total cache misses")
	} else {
		observability.IncCounter("cache_hits_total", "Total cache hits")
	}
	return value, err
}

func (d *MeteredDriver) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return d.Driver.Set(ctx, key, value, ttl)
}
