param(
    [string]$Url = "http://localhost:8080/top?limit=10",
    [int]$Count = 1000
)

for ($i = 0; $i -lt $Count; $i++) {
    Invoke-WebRequest -Uri $Url -UseBasicParsing | Out-Null
}

Write-Host "done"
