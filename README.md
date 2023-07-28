# Smart Cache ![Build](https://github.com/m-zajac/smartcache/workflows/Build/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/m-zajac/smartcache)](https://goreportcard.com/report/github.com/m-zajac/smartcache) [![GoDoc](https://godoc.org/github.com/m-zajac/smartcache?status.svg)](http://godoc.org/github.com/m-zajac/smartcache)

Smartcache package provides a thread-safe, in-memory caching solution with customizable expiration policies and error handling.
It is designed to optimize data retrieval by caching the results of expensive operations, such as database queries or API calls, and serving them quickly when requested.

The Cache type is the main component of the package and supports generic types for caching.
It offers a flexible configuration with primary and secondary TTLs for controlling cache behavior.
The primary TTL determines when the cache should be refreshed, while the secondary TTL defines when the cache can no longer be used to return data.

The cache also supports custom error handling functions to control caching behavior for failed fetches.

## Example usage

```go
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

// Initiate cache.
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
```