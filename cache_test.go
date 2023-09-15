package smartcache_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/m-zajac/smartcache/backend/lru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Parallel()

	key := "some-key"
	data := "some data"

	// This function will be used as a fetch function
	// in Get calls.
	callTokens := make(chan struct{}, 1)
	fetchFunc := func(ctx context.Context, key string) (*smartcache.FetchResult[string], error) {
		select {
		case <-callTokens:
		case <-time.After(time.Second):
			panic("unexpected call to fetchFunc")
		}

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context error: %w", err)
		}

		return &smartcache.FetchResult[string]{
			Data: &data,
		}, nil
	}

	// When Get function is called with a missing key,
	// it should fetch the data, and then return it.
	t.Run("missing key", func(t *testing.T) {
		backend, err := lru.NewBackend[string](100)
		require.NoError(t, err)

		cache, err := smartcache.New[string](backend)
		require.NoError(t, err)

		ctx := context.Background()

		callTokens <- struct{}{}
		result, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, data, *result.Data)
		assert.Equal(t, smartcache.Miss, result.Type)
	})

	// When Get function is called with a present key,
	// it should return the previously fetched data.
	t.Run("present key", func(t *testing.T) {
		backend, err := lru.NewBackend[string](100)
		require.NoError(t, err)

		const primTTL = 500 * time.Millisecond
		const secTTL = 2 * time.Second
		cache, err := smartcache.New[string](
			backend,
			smartcache.WithTTL(primTTL, secTTL),
		)
		require.NoError(t, err)

		ctx := context.Background()

		// Miss.
		callTokens <- struct{}{}
		result1, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, data, *result1.Data)
		assert.Equal(t, smartcache.Miss, result1.Type)

		// Hot hit, no call to fetchFunc.
		result2, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, *result1.Data, *result2.Data)
		assert.Equal(t, smartcache.HotHit, result2.Type)

		// Warm hit after primary TTL expiration.
		// No immediate fetchFunc call.
		time.Sleep(primTTL + time.Millisecond)
		result3, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, *result1.Data, *result3.Data)
		assert.Equal(t, smartcache.WarmHit, result3.Type)

		// Now the fetchFunc will be called in bg to update the cache
		callTokens <- struct{}{}

		// Miss after secondary TTL expiration.
		time.Sleep(secTTL + time.Millisecond)
		callTokens <- struct{}{}
		result4, err := cache.Get(ctx, key, fetchFunc)
		assert.NoError(t, err)
		assert.Equal(t, *result1.Data, *result4.Data)
		assert.Equal(t, smartcache.Miss, result4.Type)

		select {
		case <-callTokens:
			t.Fatal("fetchFunc called unexpected number of times")
		default:
		}
	})

	// When Get function is called with a context that was already cancelled,
	// it should return an error immediately.
	t.Run("cancelled parent context", func(t *testing.T) {
		backend, err := lru.NewBackend[string](100)
		require.NoError(t, err)

		cache, err := smartcache.New[string](backend)
		require.NoError(t, err)

		parentCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = cache.Get(parentCtx, key, fetchFunc)
		assert.Error(t, err)
	})

	// When a Cache object is closed, all goroutines should be cleaned up.
	t.Run("close", func(t *testing.T) {
		backend, err := lru.NewBackend[string](100)
		require.NoError(t, err)

		cache, err := smartcache.New[string](backend)
		require.NoError(t, err)

		ready := make(chan struct{})

		var calls atomic.Int32
		fetchFunc := func(ctx context.Context, key string) (*smartcache.FetchResult[string], error) {
			calls.Add(1)

			<-ready

			return &smartcache.FetchResult[string]{
				Data: &data,
			}, nil
		}

		// Trigger a lot of goroutines fetching data for 2 different keys.
		numCalls := 1000
		for i := 0; i < numCalls; i++ {
			go func() {
				_, _ = cache.Get(context.Background(), key, fetchFunc)
			}()
			go func() {
				_, _ = cache.Get(context.Background(), key+"2", fetchFunc)
			}()
		}
		time.Sleep(time.Second)

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

func TestCache_FailureOnBackgroundUpdate(t *testing.T) {
	t.Parallel()

	key := "some-key"
	data := "some data"

	// This function will be used as a fetch function
	// in Get calls.
	callTokens := make(chan struct{}, 1)
	shouldFail := false
	fetchFunc := func(ctx context.Context, key string) (*smartcache.FetchResult[string], error) {
		select {
		case <-callTokens:
		case <-time.After(time.Second):
			panic("unexpected call to fetchFunc")
		}

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context error: %w", err)
		}

		if shouldFail {
			return nil, errors.New("failed")
		}

		return &smartcache.FetchResult[string]{
			Data: &data,
		}, nil
	}

	backend, err := lru.NewBackend[string](100)
	require.NoError(t, err)

	const primTTL = 500 * time.Millisecond
	const secTTL = 2 * time.Second
	cache, err := smartcache.New[string](
		backend,
		smartcache.WithTTL(primTTL, secTTL),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// First call, cache updated.
	shouldFail = false
	callTokens <- struct{}{}
	result1, err := cache.Get(ctx, key, fetchFunc)
	require.NoError(t, err)
	assert.Equal(t, data, *result1.Data)
	assert.Equal(t, smartcache.Miss, result1.Type)

	// Wait till primary TTL expires.
	time.Sleep(primTTL + time.Millisecond)

	// Now the fetch func will fail, the stale data should be returned with no error.
	shouldFail = true
	callTokens <- struct{}{}
	result2, err := cache.Get(ctx, key, fetchFunc)
	require.NoError(t, err)
	assert.Equal(t, *result1.Data, *result2.Data)
	assert.Equal(t, smartcache.WarmHit, result2.Type)

	// Subsequent calls should also be ok.
	callTokens <- struct{}{}
	result3, err := cache.Get(ctx, key, fetchFunc)
	require.NoError(t, err)
	assert.Equal(t, *result1.Data, *result3.Data)
	assert.Equal(t, smartcache.WarmHit, result3.Type)
}
