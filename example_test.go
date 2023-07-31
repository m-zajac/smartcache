//nolint:ineffassign
package smartcache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/m-zajac/smartcache/backend/lru"
)

func ExampleCache() {
	type UserProfile struct {
		ID   int
		Name string
	}

	// An example function that provides data that we want to cache.
	fetchUserProfile := func(ctx context.Context, userID string) (*UserProfile, error) {
		// Simulate a slow database query.
		time.Sleep(500 * time.Millisecond)

		// Fetch the user profile from the database.
		profile := &UserProfile{ID: 1, Name: "John Doe"}

		return profile, nil
	}

	// This will be a fetch wrapper to use for the cache.
	// It adapts our `fetchUserProfile` function to return `smartcache.FetchResult`.
	fetchAdapter := func(ctx context.Context, key string) (*smartcache.FetchResult[UserProfile], error) {
		profile, err := fetchUserProfile(ctx, key)
		if err != nil {
			return nil, err
		}

		return &smartcache.FetchResult[UserProfile]{
			Data: profile,
		}, nil
	}

	// For this example lets use small values.
	primaryTTL, secondaryTTL := time.Second, 3*time.Second

	backend, err := lru.NewBackend[UserProfile](100)
	if err != nil {
		fmt.Println("Creating cache backend:", err)
		return
	}

	cache, err := smartcache.New[UserProfile](
		backend,
		smartcache.WithTTL(primaryTTL, secondaryTTL),
	)
	if err != nil {
		fmt.Println("Creating cache:", err)
		return
	}

	ctx := context.Background()
	userID := "1"

	// Fetch the user profile using the cache.
	profile, _ := cache.Get(ctx, userID, fetchAdapter)

	// Fetch the user profile using the cache again. This time it will be returned from internal memory cache.
	profile, _ = cache.Get(ctx, userID, fetchAdapter)

	// After primaryTTL duration paseses...
	time.Sleep(primaryTTL + time.Second)

	// ...cache will again return data from internal memory cache, but after that the internal value will be updated in the background.
	profile, _ = cache.Get(ctx, userID, fetchAdapter)

	// After secondaryTTL duration paseses...
	time.Sleep(primaryTTL + time.Second)

	// ...cache will fetch fresh data, store it in memory and return the value.
	profile, _ = cache.Get(ctx, userID, fetchAdapter)

	fmt.Printf("User profile: ID=%d, Name=%s\n", profile.ID, profile.Name)
	// Output: User profile: ID=1, Name=John Doe
}
