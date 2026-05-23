# Architecture

External Search Service -> NATS subject `search.events` -> Go service consumer -> in-memory sliding window -> cached top -> HTTP API.

The service keeps a 5-minute ring of per-second buckets. Each accepted event increments one bucket and a global query counter. Expired buckets are subtracted from global counters. `/top` returns cached sorted data and rebuilds the cache only after data changes or expiration.
