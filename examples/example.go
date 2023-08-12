package main

import (
	"context"
	"fmt"
	"time"

	"github.com/m-zajac/smartcache"
	"github.com/m-zajac/smartcache/backend/lru"
)

type UserProfile struct {
	ID   int
	Name string
}

// An example function that provides data that we want to cache.
func fetchUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	// Simulate a slow database query.
	time.Sleep(500 * time.Millisecond)

	// Fetch the user profile from the database.
	profile := &UserProfile{ID: 1, Name: "John Doe"}

	return profile, nil
}

var fetchCounter int

func main() {
	// This will be a fetch wrapper to use for the cache.
	// It adapts our `fetchUserProfile` function to return `smartcache.FetchResult`.
	fetchAdapter := func(ctx context.Context, key string) (*smartcache.FetchResult[UserProfile], error) {
		profile, err := fetchUserProfile(ctx, key)
		if err != nil {
			return nil, err
		}

		fetchCounter++

		return &smartcache.FetchResult[UserProfile]{
			Data: profile,
		}, nil
	}

	// For this example lets use small values.
	primaryTTL, secondaryTTL := 2*time.Second, 4*time.Second

	backend, err := lru.NewBackend[UserProfile](100)
	assert(err == nil)

	cache, err := smartcache.New[UserProfile](
		backend,
		smartcache.WithTTL(primaryTTL, secondaryTTL),
	)
	assert(err == nil)

	ctx := context.Background()
	cacheKey := "testkey"

	result, err := cache.Get(ctx, cacheKey, fetchAdapter)
	assert(err == nil)
	assert(fetchCounter == 1)
	printResult(result, "A first get, fetch function was called once.")

	result, err = cache.Get(ctx, cacheKey, fetchAdapter)
	assert(err == nil)
	assert(fetchCounter == 1)
	printResult(result, "This time there was no fetch call, the result is a hot cache hit.")

	// After primaryTTL duration passes...
	time.Sleep(primaryTTL + time.Second)

	result, err = cache.Get(ctx, cacheKey, fetchAdapter)
	assert(err == nil)
	printResult(result, "PrimaryTTL passed, the result is now a warm hit coming from the cache and the update is beeing made in the background.")

	// Wait a sec for the background update.
	time.Sleep(primaryTTL / 2)
	assert(fetchCounter == 2)
	fmt.Printf("... background update should be complete now, fetch counter is: %d\n\n", fetchCounter)

	// After secondaryTTL duration paseses...
	time.Sleep(secondaryTTL + time.Second)

	// ...cache will fetch fresh data, store it in memory and return the value.
	result, err = cache.Get(ctx, cacheKey, fetchAdapter)
	assert(err == nil)
	printResult(result, "SecondaryTTL passed, data was expired so it was a miss, new fetch had to be made.")
}

func printResult(v smartcache.Result[UserProfile], comment string) {
	assert(v.Data.ID == 1)

	fmt.Printf("FetchFunc calls: %d, cache hit type: %s, cache entry age: %s\n  %s\n\n",
		fetchCounter,
		v.Type.String(),
		v.Age.String(),
		comment,
	)
}

func assert(v bool) {
	if !v {
		panic("assertion failed")
	}
}
