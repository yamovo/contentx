package handlers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

type restoreContextCache struct {
	prefixes    []string
	sawDeadline bool
}

func (c *restoreContextCache) Get(context.Context, string) ([]byte, error) {
	return nil, cache.ErrCacheMiss
}

func (c *restoreContextCache) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (c *restoreContextCache) Delete(context.Context, string) error {
	return nil
}

func (c *restoreContextCache) DeletePrefix(ctx context.Context, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, c.sawDeadline = ctx.Deadline()
	c.prefixes = append(c.prefixes, prefix)
	return nil
}

func (c *restoreContextCache) Flush(context.Context) error {
	return nil
}

func TestBackupHandlerInvalidateRestoredCachesDetachesRequestCancellation(t *testing.T) {
	driver := &restoreContextCache{}
	authInvalidated := false
	h := NewBackupHandler(nil, nil).
		WithCache(driver).
		WithAuthCacheInvalidator(func() { authInvalidated = true })

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.invalidateRestoredCaches(requestCtx); err != nil {
		t.Fatalf("invalidateRestoredCaches: %v", err)
	}

	if !authInvalidated {
		t.Fatal("authentication user cache was not invalidated")
	}
	if !driver.sawDeadline {
		t.Fatal("data cache invalidation context should have a deadline")
	}
	want := []string{"articles:", "contenttype:"}
	if !reflect.DeepEqual(driver.prefixes, want) {
		t.Fatalf("prefixes = %v, want %v", driver.prefixes, want)
	}
}
