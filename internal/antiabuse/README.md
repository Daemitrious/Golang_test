Anti-abuse logic is implemented in `internal/storage` to keep event ingestion atomic with counting.
Rule: not more than `MAX_EVENTS_PER_USER_QUERY_PER_MINUTE` equal queries from one `user_id` per minute.
