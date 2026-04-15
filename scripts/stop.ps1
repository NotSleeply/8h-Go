$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

Write-Host "[stop] stopping services..."
docker compose down
Write-Host "[stop] done."
