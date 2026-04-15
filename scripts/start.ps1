$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
    Write-Host "[start] .env not found, created from .env.example"
}

Write-Host "[start] starting services..."
docker compose up -d

Write-Host "[start] waiting for mysql to be healthy..."
$deadline = (Get-Date).AddMinutes(3)
$healthy = $false
while ((Get-Date) -lt $deadline) {
    $mysqlId = (docker compose ps -q mysql).Trim()
    if (-not [string]::IsNullOrWhiteSpace($mysqlId)) {
        $status = (docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" $mysqlId).Trim()
        if ($status -eq "healthy" -or $status -eq "running") {
            $healthy = $true
            break
        }
    }
    Start-Sleep -Seconds 2
}

if (-not $healthy) {
    throw "[start] mysql is not healthy within timeout."
}

if (Test-Path ".env.local") {
    Write-Host "[start] loading .env.local into current session..."
    Get-Content ".env.local" | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) {
            return
        }
        if ($line.StartsWith("export ")) {
            $line = $line.Substring(7).Trim()
        }
        $parts = $line -split "=", 2
        if ($parts.Length -ne 2) {
            return
        }
        $key = $parts[0].Trim()
        if ([string]::IsNullOrWhiteSpace($key)) {
            return
        }
        if (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($key, "Process"))) {
            return
        }
        $value = $parts[1].Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            if ($value.Length -ge 2) {
                $value = $value.Substring(1, $value.Length - 2)
            }
        }
        Set-Item -Path "Env:$key" -Value $value
    }
}

Write-Host "[start] starting go app..."
go run .
