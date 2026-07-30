package services

import (
	"context"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

func TestInvalidateRestoredDataCache_PreservesSecurityState(t *testing.T) {
	driver := cache.NewMemoryDriver(20)
	ctx := context.Background()
	for key := range map[string]struct{}{
		"articles:list:v0:stale": {},
		"articles:id:1":          {},
		"contenttype:product":    {},
		"jwt:blacklist:token":    {},
		"login:lock:admin":       {},
	} {
		if err := driver.Set(ctx, key, []byte("value"), time.Minute); err != nil {
			t.Fatalf("seed cache key %q: %v", key, err)
		}
	}

	if err := InvalidateRestoredDataCache(ctx, driver); err != nil {
		t.Fatalf("InvalidateRestoredDataCache: %v", err)
	}

	for _, key := range []string{"articles:list:v0:stale", "articles:id:1", "contenttype:product"} {
		if _, err := driver.Get(ctx, key); err != cache.ErrCacheMiss {
			t.Fatalf("database cache key %q was not removed: %v", key, err)
		}
	}
	for _, key := range []string{"jwt:blacklist:token", "login:lock:admin"} {
		if _, err := driver.Get(ctx, key); err != nil {
			t.Fatalf("security key %q should be preserved: %v", key, err)
		}
	}
}
