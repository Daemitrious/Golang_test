# Security notes

Реализация не считает входящие данные доверенными.

Что сделано:

- NATS в локальном docker-compose закрыт простым auth token.
- Событие из брокера валидируется по размеру payload, query, event_id и user_id.
- Query нормализуется перед записью в счётчики.
- Есть дедупликация по event_id.
- Есть базовая anti-abuse логика по user_id + query.
- Stop-list API закрыт заголовком X-Admin-Token, если ADMIN_TOKEN задан.
- HTTP server имеет timeout'ы чтения и записи.
- limit в /top ограничен через MAX_TOP_LIMIT.
- Метрики не содержат user_id, ip_hash, session_id и сырые события.

Что осталось для production:

- TLS/mTLS между сервисами.
- Более строгие ACL в брокере на publish/subscribe.
- Хранение stop-list во внешнем хранилище.
- Distributed rate limit при нескольких репликах.
- Ограничение доступа к /metrics на уровне ingress/network policy.
