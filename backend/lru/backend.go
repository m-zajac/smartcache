package lru

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/m-zajac/smartcache"
)

// Backend for cache that stores data in-memory using LRU cache.
type Backend[T any] struct {
	cache *lru.Cache[string, *smartcache.CacheEntry[T]]
}

var _ smartcache.Backend[string] = &Backend[string]{}

func NewBackend[T any](size uint) (*Backend[T], error) {
	cache, err := lru.New[string, *smartcache.CacheEntry[T]](int(size))
	if err != nil {
		return nil, fmt.Errorf("creating lru cache: %w", err)
	}

	return &Backend[T]{
		cache: cache,
	}, nil
}

func (b *Backend[T]) Get(ctx context.Context, key string) (*smartcache.CacheEntry[T], error) {
	item, found := b.cache.Get(key)
	if !found {
		return nil, nil
	}

	return item, nil
}

func (b *Backend[T]) Set(ctx context.Context, key string, ttl time.Duration, data *smartcache.CacheEntry[T]) error {
	_ = b.cache.Add(key, data)

	return nil
}

func (b *Backend[T]) Close() {}
