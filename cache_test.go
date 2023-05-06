package smartcache_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	key := "some-key"
	data := "some data"

	// This function will be used as a fetch function
	// in Get calls.
	var calls atomic.Int32
	fetchFunc := func(ctx context.Context, key string) (*string, error) {
		calls.Add(1)

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context error: %w", err)
		}

		return &data, nil
	}

	// When Get function is called with a missing key,
	// it should fetch the data, and then return it.
	t.Run("missing key", func(t *testing.T) {
		calls.Store(0)

		cache := smartcache.New[string]()

		ctx := context.Background()

		actual, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, data, *actual)
		assert.EqualValues(t, 1, calls.Load())
	})

	// When Get function is called with a present key,
	// it should return the previously fetched data.
	t.Run("present key", func(t *testing.T) {
		calls.Store(0)

		cache := smartcache.New[string]()

		ctx := context.Background()

		actual1, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, data, *actual1)
		assert.EqualValues(t, 1, calls.Load())

		actual2, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, *actual1, *actual2)
		assert.EqualValues(t, 1, calls.Load())
	})

	// When Get function is called with a context that was already cancelled,
	// it should return an error immediately.
	t.Run("cancelled parent context", func(t *testing.T) {
		cache := smartcache.New[string]()

		parentCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := cache.Get(parentCtx, key, fetchFunc)
		assert.Error(t, err)
	})

	// When a Cache object is closed, all goroutines should be cleaned up.
	t.Run("close", func(t *testing.T) {
		cache := smartcache.New[string]()

		ready := make(chan struct{})

		calls.Store(0)
		fetchFunc := func(ctx context.Context, key string) (*string, error) {
			calls.Add(1)

			<-ready

			return &data, nil
		}

		// Trigger a lot of goroutines fetching data for 2 different keys.
		numCalls := 1000
		for i := 0; i < numCalls; i++ {
			go func() {
				cache.Get(context.Background(), key, fetchFunc)
			}()
			go func() {
				cache.Get(context.Background(), key+"2", fetchFunc)
			}()
		}

		// Close the cache.
		cacheClosed := make(chan struct{})
		go func() {
			cache.Close()
			close(cacheClosed)
		}()

		// Unblock fetchFunc.
		close(ready)

		// If cache was closed properly, number of fetchFunc calls should be exactly 2 (we used 2 keys!).
		select {
		case <-time.After(5 * time.Second):
			t.Error("timeout")
		case <-cacheClosed:
			assert.EqualValues(t, 2, calls.Load())
		}
	})
}
