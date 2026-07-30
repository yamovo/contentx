package services

import (
	"context"
	"errors"

	"github.com/yamovo/contentx/internal/cache"
)

var restoredDataCachePrefixes = []string{
	"articles:",
	"contenttype:",
}

// InvalidateRestoredDataCache removes database-derived cache entries after a
// restore while preserving unrelated Redis state such as JWT revocations,
// login throttling, and distributed locks.
func InvalidateRestoredDataCache(ctx context.Context, driver cache.Driver) error {
	if driver == nil {
		return nil
	}

	var errs []error
	for _, prefix := range restoredDataCachePrefixes {
		if err := driver.DeletePrefix(ctx, prefix); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
