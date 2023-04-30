package smartcache

import (
	"context"
	"sync"
	"time"
)

type FetchFunc[T any] func(ctx context.Context, key string) (*T, error)

type CacheErrorFunc func(err error) time.Duration

type item[T any] struct {
	data       *T
	expiration time.Time
	err        error
	errExpiry  time.Time
}

type SmartCache[T any] struct {
	cache        map[string]*item[T]
	mutex        sync.Mutex
	primaryTTL   time.Duration
	secondaryTTL time.Duration
	cacheErrFunc CacheErrorFunc
}

func NewSmartCache[T any](primaryTTL, secondaryTTL time.Duration, cacheErrFunc CacheErrorFunc) *SmartCache[T] {
	return &SmartCache[T]{
		cache:        make(map[string]*item[T]),
		primaryTTL:   primaryTTL,
		secondaryTTL: secondaryTTL,
		cacheErrFunc: cacheErrFunc,
	}
}

func (sc *SmartCache[T]) isExpired(it *item[T]) bool {
	now := time.Now()
	return it.expiration.Before(now) || it.errExpiry.Before(now)
}

func (sc *SmartCache[T]) Get(ctx context.Context, key string, fetchFunc FetchFunc[T]) (*T, error) {
	sc.mutex.Lock()
	it, found := sc.cache[key]
	sc.mutex.Unlock()

	if found && !sc.isExpired(it) {
		return it.data, it.err
	}

	data, err := fetchFunc(ctx, key)
	if err != nil {
		ttl := sc.cacheErrFunc(err)
		if ttl > 0 {
			sc.mutex.Lock()
			sc.cache[key] = &item[T]{data: it.data, expiration: time.Now().Add(sc.secondaryTTL), err: err, errExpiry: time.Now().Add(ttl)}
			sc.mutex.Unlock()
			return it.data, err
		}
		return nil, err
	}

	sc.mutex.Lock()
	sc.cache[key] = &item[T]{data: data, expiration: time.Now().Add(sc.primaryTTL), err: nil, errExpiry: time.Time{}}
	sc.mutex.Unlock()

	return data, nil
}
