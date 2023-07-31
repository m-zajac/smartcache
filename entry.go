package smartcache

import "time"

type CacheEntry[T any] struct {
	Data            *T
	Err             error
	Created         time.Time
	FixedExpiration *time.Time
}

func newOKCacheItem[T any](data *T) *CacheEntry[T] {
	return &CacheEntry[T]{Data: data, Created: time.Now()}
}

func newErrCacheItem[T any](err error, ttl time.Duration) *CacheEntry[T] {
	exp := time.Now().Add(ttl)
	return &CacheEntry[T]{Err: err, FixedExpiration: &exp}
}

func newEmptyExpiredCacheItem[T any]() *CacheEntry[T] {
	exp := time.Now()
	return &CacheEntry[T]{FixedExpiration: &exp}
}

func (it *CacheEntry[T]) IsExpired(ttl time.Duration) bool {
	now := time.Now()
	if it.FixedExpiration != nil {
		return it.FixedExpiration.Before(now)
	}

	return it.Created.Add(ttl).Before(now)
}
