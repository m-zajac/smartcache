// smartcache package provides a thread-safe, in-memory caching solution with customizable expiration policies and error handling. It is designed to optimize data retrieval by caching the results of expensive operations, such as database queries or API calls, and serving them quickly when requested.
//
// The SmartCache type is the main component of the package and supports generic types for caching. It offers a flexible configuration with primary and secondary TTLs (time-to-live) for controlling cache behavior. The primary TTL determines when the cache should be refreshed, while the secondary TTL defines when the cache can no longer be used to return data. The cache also supports custom error handling functions to control caching behavior for failed fetches.
//
// Example use case:
//
// Suppose we have an application that retrieves user profiles from a slow database. We can use SmartCache to cache the profiles and serve them quickly to reduce the load on the database.
//
// package main
//
// import (
//
//	"context"
//	"fmt"
//	"time"
//
//	"github.com/yourusername/smartcache"
//
// )
//
//	type UserProfile struct {
//		ID   int
//		Name string
//	}
//
//	func fetchUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
//		// Simulate a slow database query.
//		time.Sleep(2 * time.Second)
//
//		// Fetch the user profile from the database.
//		profile := &UserProfile{ID: 1, Name: "John Doe"}
//
//		return profile, nil
//	}
//
//	func main() {
//		cache := smartcache.New[UserProfile](
//			smartcache.PrimaryTTL(5*time.Minute),
//			smartcache.SecondaryTTL(10*time.Minute),
//		)
//
//		ctx := context.Background()
//		userID := "1"
//
//		// Fetch the user profile using the cache.
//		profile, err := cache.Get(ctx, userID, fetchUserProfile)
//		if err != nil {
//			fmt.Println("Error fetching user profile:", err)
//			return
//		}
//
//		fmt.Printf("User profile: ID=%d, Name=%s\n", profile.ID, profile.Name)
//	}
//
// In this example, we create a SmartCache instance with a primary TTL of 5 minutes and a secondary TTL of 10 minutes. When we request a user profile, the cache will first check if it has a valid, non-expired entry. If not, it will fetch the profile from the database and store it in the cache. Subsequent requests for the same user profile within the primary TTL will be served from the cache, reducing the load on the database.
package smartcache
