package smartcache

import (
	"context"
	"sync"
	"time"
)

type FetchFunc[T any] func(ctx context.Context, key string) (*T, error)

type ErrorTTLFunc func(err error) time.Duration

type BackgroundErrorHandler func(err error)

type Cache[T any] struct {
	cache map[string]cacheItemCh[T]
	mu    sync.Mutex

	config config
	// ctx is the parent context of background refreshes.
	// It will be closed when `Close` method is called.
	ctx       context.Context
	ctxCancel func()

	wg sync.WaitGroup
}

func New[T any](options ...Option) *Cache[T] {
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
		o(&cfg)
	}

	// TODO: validate config!

	ctx, cancel := context.WithCancel(context.Background())

	return &Cache[T]{
		cache:     make(map[string]cacheItemCh[T]),
		config:    cfg,
		ctx:       ctx,
		ctxCancel: cancel,
	}
}

func (sc *Cache[T]) Close() {
	sc.ctxCancel()
	sc.wg.Wait()
}

func (sc *Cache[T]) Get(ctx context.Context, key string, fetchFunc FetchFunc[T]) (*T, error) {
	if err := sc.ctx.Err(); err != nil {
		return nil, err
	}

	sc.wg.Add(1)
	defer sc.wg.Done()

	sc.mu.Lock()
	itemCh, found := sc.cache[key]
	sc.mu.Unlock()

	// If no cache found, put an expired item to it.
	// Then it will be handled the same way as if cache was there but expired.
	if !found {
		itemCh = make(cacheItemCh[T], 1)

		sc.mu.Lock()
		sc.cache[key] = itemCh
		sc.mu.Unlock()

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
