.PHONY: test up demo down logs publish load-test smoke

test:
	go test ./...

up:
	docker compose up --build -d nats service prometheus grafana

demo:
	docker compose --profile demo up --build

down:
	docker compose down -v

logs:
	docker compose logs -f service

publish:
	./scripts/publish-event.sh

load-test:
	./scripts/load-test.sh

smoke:
	./tests/integration/smoke.sh
