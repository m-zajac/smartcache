// smartcache package provides a thread-safe, in-memory caching solution with customizable expiration policies and error handling.
// It is designed to optimize data retrieval by caching the results of expensive operations, such as database queries or API calls, and serving them quickly when requested.
//
// The Cache type is the main component of the package and supports generic types for caching.
// It offers a flexible configuration with primary and secondary TTLs for controlling cache behavior.
// The primary TTL determines when the cache should be refreshed, while the secondary TTL defines when the cache can no longer be used to return data.
//
// The cache also supports custom error handling functions to control caching behavior for failed fetches.
package smartcache
