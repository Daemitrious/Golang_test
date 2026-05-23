# Broker contract

Subject: `search.events`

```json
{
  "event_id": "018f3c0b-2f21-7a9f-b9e8-1f5d7c1b9c01",
  "query": "кроссовки",
  "user_id": "user-123",
  "session_id": "session-456",
  "ip_hash": "iphash-789",
  "timestamp": "2026-05-23T12:00:00Z"
}
```

Fields:
- `event_id` is used for deduplication.
- `query` is used for top calculation.
- `user_id` is used for anti-abuse limits.
- `session_id` is optional and can be used for future anti-abuse rules.
- `ip_hash` is optional and can be used for future anti-abuse rules without storing raw IP.
- `timestamp` is used for the 5-minute event-time window.
