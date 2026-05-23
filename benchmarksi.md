# Benchmarks

Команды:

bash:
go test ./...
go test -bench=. -benchmem ./internal/storage

Результат локального запуска в sandbox-среде:

text-ом:

goos: linux
goarch: amd64
pkg: search-trends/internal/storage
cpu: Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz
BenchmarkAddEvent-56       703546      7242 ns/op      178 B/op   7 allocs/op
BenchmarkTopCached-56     3318721       393.7 ns/op    240 B/op   1 allocs/op
PASS


Вывод:

- AddEvent выполняет валидацию, дедупликацию, stop-list, anti-abuse и обновление секундного бакета.
- TopCached отдаёт заранее подготовленный top и не сортирует данные на каждый запрос.
- Основная идея оптимизации: сделать /top дешёвым, потому что чтений ожидается больше, чем входящих событий.

HTTP-нагрузку нужно прогнать локально после запуска Docker Compose:

bash:
./scripts/load-test.sh


или на Windows:

powershell:
./scripts/load-test.ps1

