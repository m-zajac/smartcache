package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/m-zajac/smartcache"
	redisbackend "github.com/m-zajac/smartcache/backend/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestBackend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := miniredis.RunT(t)

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	cmd := rdb.Ping(ctx)
	assert.NoError(t, cmd.Err())

	backend, err := redisbackend.NewBackend[string](rdb, "testprefix")
	assert.NoError(t, err)

	tests := []struct {
		name        string
		entry       smartcache.CacheEntry[string]
		ttl         time.Duration
		wantExpired bool
	}{
		{
			name: "simple",
			entry: smartcache.CacheEntry[string]{
				Data:    ptr("testvalue"),
				Created: time.Now().Add(-time.Minute),
			},
			ttl:         time.Minute,
			wantExpired: false,
		},
		{
			name: "simple, expired",
			entry: smartcache.CacheEntry[string]{
				Data:    ptr("testvalue"),
				Created: time.Now().Add(-time.Minute),
			},
			ttl:         time.Nanosecond,
			wantExpired: true,
		},
		{
			name: "with error",
			entry: smartcache.CacheEntry[string]{
				Created: time.Now().Add(-time.Minute),
				Err:     errors.New("test error"),
			},
			ttl:         time.Minute,
			wantExpired: false,
		},
		{
			name: "with fixed expiration",
			entry: smartcache.CacheEntry[string]{
				Data:            ptr("testvalue"),
				Created:         time.Now().Add(-time.Minute),
				FixedExpiration: ptr(time.Now().Add(time.Hour)),
			},
			ttl:         time.Minute,
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test" + tt.name
			err = backend.Set(ctx, key, tt.ttl, &tt.entry)
			assert.NoError(t, err)

			s.FastForward(time.Second)

			gotEntry, err := backend.Get(ctx, key)
			assert.NoError(t, err)
			if tt.wantExpired {
				assert.Nil(t, gotEntry)
			} else {
				assert.Equal(t, tt.entry.Created.Unix(), gotEntry.Created.Unix())
				assert.Equal(t, tt.entry.Data, gotEntry.Data)
				assert.Equal(t, tt.entry.Err, gotEntry.Err)
				if tt.entry.FixedExpiration == nil {
					assert.Nil(t, gotEntry.FixedExpiration)
				} else {
					assert.Equal(t, tt.entry.FixedExpiration.Unix(), gotEntry.FixedExpiration.Unix())
				}
			}
		})
	}

}

func ptr[T any](v T) *T {
	return &v
}
