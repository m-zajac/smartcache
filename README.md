# Smart Cache ![Build](https://github.com/m-zajac/smartcache/workflows/Build/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/m-zajac/smartcache)](https://goreportcard.com/report/github.com/m-zajac/smartcache) [![GoDoc](https://godoc.org/github.com/m-zajac/smartcache?status.svg)](http://godoc.org/github.com/m-zajac/smartcache)


Smart Cache introduces a caching mechanism that employs two distinct TTL (Time-To-Live) periods, designed to optimize data retrieval speed while managing data freshness.

Traditional caching often faces a dilemma: prioritize speed by serving cached data or ensure freshness by frequently fetching new data. This balance can significantly influence metrics like p95 or p99 processing times, especially when dealing with resource-intensive operations.

With Smart Cache, data within its primary TTL is returned immediately, ensuring quick access. When data surpasses the primary TTL but is still within the secondary TTL, it's served instantly, but a background update is initiated. This background update mechanism aims to improve latency metrics by reducing the immediate load of full data refreshes. However, there's a trade-off to be aware of: data served between the primary and secondary TTL might be slightly older, and the freshest data is only guaranteed on a subsequent fetch after the background update.

In essence, Smart Cache's dual-TTL approach offers a balanced solution for applications that need to maintain high-performance metrics while occasionally serving slightly older data.

To better understand the transitions between data states, check the following diagram:

```mermaid
stateDiagram-v2
    Hot --> Warm: After primary TTL expired
    Warm --> Expired: After secondary TTL expired
    Expired --> Hot: Update
    Warm --> Hot: Update
```

## Example usage

[Check the example on go playground](https://go.dev/play/p/xjl7MwQ8Sjq)

## How it works


```mermaid
sequenceDiagram
    opt First call or after secondary TTL expired
        UserApp->>+SmartCache: Request data
        SmartCache->>+MemoryBackend: Check cache
        MemoryBackend->>-SmartCache: Data absent or secondaryTTL expired
        SmartCache->>+FetchFunction: Fetch fresh data
        FetchFunction->>FetchFunction: ...time-consuming operation...
        FetchFunction->>-SmartCache: Return fresh data
        SmartCache->>MemoryBackend: Cache data
        SmartCache->>-UserApp: Return fresh data
    end

    opt Call within primary TTL
        UserApp->>+SmartCache: Request data
        SmartCache->>+MemoryBackend: Check cache
        MemoryBackend->>-SmartCache: Data available
        SmartCache->>-UserApp: Return hot data
    end

    opt Call after primary TTL but before secondary TTL expired
        UserApp->>+SmartCache: Request data
        SmartCache->>+MemoryBackend: Check cache
        MemoryBackend->>-SmartCache: Data available
        SmartCache->>-UserApp: Return warm data
        SmartCache->>+FetchFunction: Background fetch for data update
        FetchFunction->>FetchFunction: ...time-consuming operation...
        FetchFunction->>-SmartCache: Return updated data
        SmartCache->>MemoryBackend: Refresh cache
    end
```

