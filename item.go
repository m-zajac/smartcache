package smartcache

import "time"

type cacheItem[T any] struct {
	data            *T
	err             error
	created         time.Time
	fixedExpiration *time.Time
}

func newOKCacheItem[T any](data *T) *cacheItem[T] {
	return &cacheItem[T]{data: data, created: time.Now()}
}

func newErrCacheItem[T any](err error, ttl time.Duration) *cacheItem[T] {
	exp := time.Now().Add(ttl)
	return &cacheItem[T]{err: err, fixedExpiration: &exp}
}

func newEmptyExpiredCacheItem[T any]() *cacheItem[T] {
	exp := time.Now()
	return &cacheItem[T]{fixedExpiration: &exp}
}

func (it *cacheItem[T]) isExpired(ttl time.Duration) bool {
	if it.fixedExpiration != nil {
		return it.fixedExpiration.Before(time.Now())
	}

	return it.created.Add(ttl).Before(time.Now())
}

type cacheItemCh[T any] chan *cacheItem[T]
