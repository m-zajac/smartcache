package smartcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type ResultType int

// Result types.
const (
	Miss ResultType = iota
	WarmHit
	HotHit
)

type Result[T any] struct {
	Data *T
	Type ResultType
}

// Backend can store and retrieve cache data by key.
type Backend[T any] interface {
	// Get returns cache data by key.
	Get(ctx context.Context, key string) (*CacheEntry[T], error)
	// Set stores cache data by key.
	Set(ctx context.Context, key string, ttl time.Duration, data *CacheEntry[T]) error
	// Closes the backend.
	Close()
}

// FetchFunc fetches data to be cached.
type FetchFunc[T any] func(ctx context.Context, key string) (*FetchResult[T], error)

// FetchResult is a container for cached item.
type FetchResult[T any] struct {
	// Data contains the result to store in cache.
	Data *T
	// CreatedAt is a time when the data was fetched as fresh.
	// Optional, defaults to the function call time.
	CreatedAt time.Time
	// Type of the result.
	Type ResultType
}

// ErrorTTLFunc defines if and for how long to cache errors returned by `FetchFunc`.
// If it returns 0, error will not be cached.
type ErrorTTLFunc func(err error) time.Duration

// BackgroundErrorHandler is a handler for `FetchFunc` errors, if they happen during a background refresh.
type BackgroundErrorHandler func(err error)

type request struct {
	requests      uint
	lock          chan struct{}
	updatePending bool
}

// Cache stores the internal in-memory LRU cache and is responsible for coordinating the cache access.
type Cache[T any] struct {
	backend  Backend[T]
	requests chan map[string]*request

	config config

	// ctx is the parent context of background refreshes.
	// It will be closed when `Close` method is called.
	ctx       context.Context
	ctxCancel func()

	wg sync.WaitGroup
}

// New creates a new cache.
func New[T any](backend Backend[T], options ...Option) (*Cache[T], error) {
	if backend == nil {
		return nil, errors.New("backend is nil")
	}

	// Create an initial config with sane defaults.
	cfg := config{
		primaryTTL:             time.Minute,
		secondaryTTL:           time.Hour,
		backgroundFetchTimeout: time.Minute,
		backgroundErrorHandler: func(err error) {},                         // Empty function to avoid nil checks.
		errorTTLFunc:           func(err error) time.Duration { return 0 }, // Don't cache errors.
	}

	// Apply all user options.
	for _, o := range options {
		if err := o(&cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
	}

	requestsCh := make(chan map[string]*request, 1)
	requestsCh <- make(map[string]*request)

	ctx, cancel := context.WithCancel(context.Background())
	return &Cache[T]{
		backend:   backend,
		requests:  requestsCh,
		config:    cfg,
		ctx:       ctx,
		ctxCancel: cancel,
	}, nil
}

// Close closes the cache.
func (sc *Cache[T]) Close() {
	sc.ctxCancel()
	sc.wg.Wait()
}

// Get retrieves a value from the cache for a given key. If the value is not found or expired, the function fetches the data using the provided fetchFunc and updates the cache accordingly.
func (sc *Cache[T]) Get(ctx context.Context, key string, fetchFunc FetchFunc[T]) (Result[T], error) {
	var result Result[T]

	if err := sc.ctx.Err(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	sc.wg.Add(1)
	defer sc.wg.Done()

	// Obtain request with a lock, increase requests count for the key, obtain a lock for the key.
	requests := <-sc.requests
	req, found := requests[key]
	if found {
		req.requests++
	} else {
		req = &request{
			requests: 1,
			lock:     make(chan struct{}, 1),
		}
		req.lock <- struct{}{}
		requests[key] = req
	}
	lockCh := req.lock
	sc.requests <- requests

	// On finish: decrease requests count for key, remove the entry if count goes to 0, release the lock.
	defer func() {
		requests := <-sc.requests
		requests[key].requests--
		if requests[key].requests == 0 {
			delete(requests, key)
		}
		sc.requests <- requests

		lockCh <- struct{}{}
	}()

	<-lockCh

	entry, err := sc.backend.Get(ctx, key)
	if err != nil {
		return result, fmt.Errorf("cache backend failed for key '%s': %w", key, err)
	}

	switch {
	// Cached data is stale or missing, fetchFunc has to be called immmediately.
	case entry == nil || entry.IsExpired(sc.config.secondaryTTL):
		result.Type = Miss

		fetchCtx, cancel := sc.newForegroundContext(ctx)
		defer cancel()

		item, err := sc.fetchToCacheEntry(fetchCtx, key, fetchFunc)
		if err != nil {
			return result, err
		}

		if err := sc.backend.Set(ctx, key, sc.config.secondaryTTL, item); err != nil {
			return result, fmt.Errorf("failed to update cache for key '%s': %w", key, err)
		}

		result.Data = item.Data

		return result, item.Err

	// Cached data is fresh.
	case !entry.IsExpired(sc.config.primaryTTL):
		result.Type = HotHit
		result.Data = entry.Data

		return result, entry.Err

	// Cached data can be returned, but needs a refresh in the background.
	default:
		result.Type = WarmHit
		result.Data = entry.Data

		requests := <-sc.requests
		defer func() { sc.requests <- requests }()

		if requests[key].updatePending {
			// There's already a background update pending,
			// the data can be returned immediately.
			return result, entry.Err
		}

		// Initiate data refresh in the background.
		requests[key].updatePending = true
		requests[key].requests++

		sc.wg.Add(1)
		go func() {
			defer sc.wg.Done()

			bkgCtx, cancel := sc.newBackgroundContext()
			defer cancel()

			item, err := sc.fetchToCacheEntry(bkgCtx, key, fetchFunc)
			if err != nil {
				sc.config.backgroundErrorHandler(err)
			}

			if err := sc.backend.Set(ctx, key, sc.config.secondaryTTL, item); err != nil {
				sc.config.backgroundErrorHandler(fmt.Errorf("failed to update cache for key '%s': %w", key, err))
			}

			requests := <-sc.requests
			requests[key].requests--
			requests[key].updatePending = false
			if requests[key].requests == 0 {
				delete(requests, key)
			}
			sc.requests <- requests
		}()

		return result, entry.Err
	}
}

func (sc *Cache[T]) fetchToCacheEntry(ctx context.Context, key string, fetchFunc FetchFunc[T]) (*CacheEntry[T], error) {
	data, err := fetchFunc(ctx, key)
	if err != nil {
		errTTL := sc.config.errorTTLFunc(err)
		if errTTL == 0 {
			return newEmptyExpiredCacheItem[T](), err
		}

		return newErrCacheItem[T](err, errTTL), nil
	}

	return newOKCacheItem(data.Data), nil
}

func (sc *Cache[T]) newBackgroundContext() (ctx context.Context, cancel func()) {
	if sc.config.backgroundFetchTimeout > 0 {
		return context.WithTimeout(sc.ctx, sc.config.backgroundFetchTimeout)
	}

	return context.WithCancel(sc.ctx)
}

func (sc *Cache[T]) newForegroundContext(ctx context.Context) (fgCtx context.Context, cancel func()) {
	fgCtx, cancel = context.WithCancel(ctx)
	go func() {
		select {
		// If the cache context was cancelled, returned context should also be cancelled.
		case <-sc.ctx.Done():
			cancel()
		case <-fgCtx.Done():
		}
	}()

	return fgCtx, cancel
}
