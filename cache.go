package smartcache

import (
	"context"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// FetchFunc fetches data to be cached.
type FetchFunc[T any] func(ctx context.Context, key string) (*T, error)

// ErrorTTLFunc defines if and for how long to cache errors returned by `FetchFunc`.
// If it returns 0, error will not be cached.
type ErrorTTLFunc func(err error) time.Duration

// BackgroundErrorHandler is a handler for `FetchFunc` errors, if they happen during a background refresh.
type BackgroundErrorHandler func(err error)

// Cache stores the internal in-memory LRU cache and is responsible for coordinating the cache access.
type Cache[T any] struct {
	cache *lru.Cache[string, cacheItemCh[T]]

	config config
	// ctx is the parent context of background refreshes.
	// It will be closed when `Close` method is called.
	ctx       context.Context
	ctxCancel func()

	wg sync.WaitGroup
}

func New[T any](options ...Option) (*Cache[T], error) {
	// Create an initial config with sane defaults.
	cfg := config{
		primaryTTL:             time.Minute,
		secondaryTTL:           time.Hour,
		backgroundFetchTimeout: time.Minute,
		backgroundErrorHandler: func(err error) {},                         // Empty function to avoid nil checks.
		errorTTLFunc:           func(err error) time.Duration { return 0 }, // Don't cache errors.
		cacheSize:              10000,
	}

	// Apply all user options.
	for _, o := range options {
		if err := o(&cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
	}

	cache, err := lru.New[string, cacheItemCh[T]](int(cfg.cacheSize))
	if err != nil {
		return nil, fmt.Errorf("creating lru cache: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Cache[T]{
		cache:     cache,
		config:    cfg,
		ctx:       ctx,
		ctxCancel: cancel,
	}, nil
}

func (sc *Cache[T]) Close() {
	sc.ctxCancel()
	sc.wg.Wait()
}

// Get retrieves a value from the cache for a given key. If the value is not found or expired, the function fetches the data using the provided fetchFunc and updates the cache accordingly.
func (sc *Cache[T]) Get(ctx context.Context, key string, fetchFunc FetchFunc[T]) (*T, error) {
	if err := sc.ctx.Err(); err != nil {
		return nil, err
	}

	sc.wg.Add(1)
	defer sc.wg.Done()

	itemCh, found := sc.cache.Get(key)

	// If no cache found, put an expired item to it.
	// Then it will be handled the same way as if cache was there but expired.
	if !found {
		itemCh = make(cacheItemCh[T], 1)
		sc.cache.Add(key, itemCh)

		itemCh <- newEmptyExpiredCacheItem[T]()
	}

	item := <-itemCh

	switch {
	// Cached data is fresh.
	case !item.isExpired(sc.config.primaryTTL):
		defer func() { itemCh <- item }()
		return item.data, item.err

	// Cached data is stale.
	case item.isExpired(sc.config.secondaryTTL):
		fetchCtx, cancel := sc.newForegroundContext(ctx)
		defer cancel()

		item, err := sc.fetchToCacheItem(fetchCtx, key, fetchFunc)
		if err != nil {
			itemCh <- item
			return nil, err
		}

		itemCh <- item
		return item.data, item.err

	// Cached data can be returned, but needs a refresh.
	default:
		data, err := item.data, item.err

		// Refresh data in the background.
		// Note that the cache key is stil blocked until this goroutine finishes.
		// Next caller for the same key will have to wait on cache channel.
		sc.wg.Add(1)
		go func() {
			defer sc.wg.Done()

			bkgCtx, cancel := sc.newBackgroundContext()
			defer cancel()

			newItem, err := sc.fetchToCacheItem(bkgCtx, key, fetchFunc)
			if err != nil {
				sc.config.backgroundErrorHandler(err)
				itemCh <- item // Put the old item back to cache, it is still valid.
			}

			itemCh <- newItem
		}()

		return data, err
	}
}

func (sc *Cache[T]) fetchToCacheItem(ctx context.Context, key string, fetchFunc FetchFunc[T]) (*cacheItem[T], error) {
	data, err := fetchFunc(ctx, key)
	if err != nil {
		errTTL := sc.config.errorTTLFunc(err)
		if errTTL == 0 {
			return newEmptyExpiredCacheItem[T](), err
		}

		return newErrCacheItem[T](err, errTTL), nil
	}

	return newOKCacheItem(data), nil
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
