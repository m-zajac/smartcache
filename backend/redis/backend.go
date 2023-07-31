package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/redis/go-redis/v9"
)

// Backend for cache that stores data in redis.
//
// The data is serialized to JSON and stored as a JSON string. Note that the T type data has to be properly JSON-serializable!
//
// The error stored in cache entries will be stored as a string. That means that it's type be lost,
// and after retrieval it will be a plain new go error.
//
// The client will be closed when the parent cache is closed.
type Backend[T any] struct {
	client    *redis.Client
	keyPrefix string
}

var _ smartcache.Backend[string] = &Backend[string]{}

func NewBackend[T any](client *redis.Client, keyPrefix string) (*Backend[T], error) {
	if client == nil {
		return nil, errors.New("redis client is nil")
	}

	return &Backend[T]{
		client:    client,
		keyPrefix: keyPrefix,
	}, nil
}

func (b *Backend[T]) Get(ctx context.Context, key string) (*smartcache.CacheEntry[T], error) {
	data, err := b.client.Get(ctx, b.keyPrefix+key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("fetching data from redis: %w", err)
	}

	return b.deserialize([]byte(data))
}

func (b *Backend[T]) Set(ctx context.Context, key string, ttl time.Duration, entry *smartcache.CacheEntry[T]) error {
	data, err := b.serialize(entry)
	if err != nil {
		return err
	}

	b.client.Set(ctx, b.keyPrefix+key, string(data), ttl)

	return nil
}

func (b *Backend[T]) Close() {
	_ = b.client.Close()
}

type container[T any] struct {
	Data            *T         `json:"data"`
	Err             string     `json:"err"`
	Created         time.Time  `json:"created"`
	FixedExpiration *time.Time `json:"fixedExpiration,omitempty"`
}

func (b *Backend[T]) serialize(entry *smartcache.CacheEntry[T]) ([]byte, error) {
	errStr := ""
	if entry.Err != nil {
		errStr = entry.Err.Error()
	}
	v, err := json.Marshal(container[T]{
		Data:            entry.Data,
		Err:             errStr,
		Created:         entry.Created,
		FixedExpiration: entry.FixedExpiration,
	})
	if err != nil {
		return nil, fmt.Errorf("serializing to json: %w", err)
	}

	return v, nil
}

func (b *Backend[T]) deserialize(data []byte) (*smartcache.CacheEntry[T], error) {
	var c container[T]
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("deserializing json: %w", err)
	}

	var err error
	if c.Err != "" {
		err = errors.New(c.Err)
	}

	return &smartcache.CacheEntry[T]{
		Data:            c.Data,
		Err:             err,
		Created:         c.Created,
		FixedExpiration: c.FixedExpiration,
	}, nil
}
