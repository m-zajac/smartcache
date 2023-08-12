package lru_test

import (
	"context"
	"testing"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/m-zajac/smartcache/backend/lru"
	"github.com/stretchr/testify/assert"
)

func TestBackend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	backend, err := lru.NewBackend[string](100)
	assert.NoError(t, err)

	entry := smartcache.CacheEntry[string]{
		Data:    ptr("testvalue"),
		Created: time.Now().Add(-time.Minute),
	}

	key := "test"
	err = backend.Set(ctx, key, time.Minute, &entry)
	assert.NoError(t, err)

	gotEntry, err := backend.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, &entry, gotEntry)
}

func ptr[T any](v T) *T {
	return &v
}
