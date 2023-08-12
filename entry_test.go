package smartcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntryExpired(t *testing.T) {
	tests := []struct {
		name        string
		created     time.Time
		fixedExp    *time.Time
		ttl         time.Duration
		wantExpired bool
	}{
		{
			name:        "zero ttl",
			created:     time.Now(),
			fixedExp:    nil,
			ttl:         0,
			wantExpired: true,
		},
		{
			name:        "non-zero ttl",
			created:     time.Now(),
			fixedExp:    nil,
			ttl:         time.Second,
			wantExpired: false,
		},
		{
			name:        "fixed exp in past, expired",
			created:     time.Now(),
			fixedExp:    timePtr(time.Now().Add(-time.Minute)),
			ttl:         30 * time.Second,
			wantExpired: true,
		},
		{
			name:        "fixed exp in future, not expired",
			created:     time.Now(),
			fixedExp:    timePtr(time.Now().Add(time.Minute)),
			ttl:         0,
			wantExpired: false,
		},
		{
			name:        "created in past, expired",
			created:     time.Now().Add(-time.Minute),
			ttl:         30 * time.Second,
			wantExpired: true,
		},
		{
			name:        "created in past, not expired",
			created:     time.Now().Add(-time.Minute),
			ttl:         2 * time.Minute,
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := CacheEntry[bool]{
				Data:            new(bool),
				Err:             nil,
				Created:         tt.created,
				FixedExpiration: tt.fixedExp,
			}
			assert.Equal(t, tt.wantExpired, e.IsExpired(tt.ttl))
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
