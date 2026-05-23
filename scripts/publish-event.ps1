param(
    [string]$Query = "кроссовки",
    [string]$UserId = "user-1"
)

docker compose run --rm demo-producer /app/app -once -query $Query -user $UserId
