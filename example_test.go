//nolint:ineffassign
package smartcache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/m-zajac/smartcache"
)

func ExampleCache() {
	type UserProfile struct {
		ID   int
		Name string
	}

	fetchUserProfile := func(ctx context.Context, userID string) (*UserProfile, error) {
		// Simulate a slow database query.
		time.Sleep(500 * time.Millisecond)

		// Fetch the user profile from the database.
		profile := &UserProfile{ID: 1, Name: "John Doe"}

		return profile, nil
	}

	// For this example lets use small values.
	primaryTTL, secondaryTTL := time.Second, 3*time.Second

	cache, err := smartcache.New[UserProfile](
		smartcache.WithCacheSize(1000),
		smartcache.WithTTL(primaryTTL, secondaryTTL),
	)
	if err != nil {
		fmt.Println("Creating cache:", err)
		return
	}

	ctx := context.Background()
	userID := "1"

	// Fetch the user profile using the cache.
	profile, _ := cache.Get(ctx, userID, fetchUserProfile)

	// Fetch the user profile using the cache again. This time it will be returned from internal memory cache.
	profile, _ = cache.Get(ctx, userID, fetchUserProfile)

	// After primaryTTL duration paseses...
	time.Sleep(primaryTTL + time.Second)

	// ...cache will again return data from internal memory cache, but after that the internal value will be updated in the background.
	profile, _ = cache.Get(ctx, userID, fetchUserProfile)

	// After secondaryTTL duration paseses...
	time.Sleep(primaryTTL + time.Second)

	// ...cache will fetch fresh data, store it in memory and return the value.
	profile, _ = cache.Get(ctx, userID, fetchUserProfile)

	fmt.Printf("User profile: ID=%d, Name=%s\n", profile.ID, profile.Name)
	// Output: User profile: ID=1, Name=John Doe
}
