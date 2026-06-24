<#
.SYNOPSIS
  Builds the Go API and Next.js frontend and (re)installs them as Windows
  services via NSSM, for a native (no Docker) deployment on this machine.

.PREREQUISITES
  - Go and Node.js already installed and on PATH.
  - NSSM (https://nssm.cc/) downloaded and either on PATH or its folder
    passed via -NssmPath.
  - backend\.env created from backend\.env.windows.example, with
    REPLACE_WITH_SERVER_HOST swapped for this machine's real hostname/IP.
  - frontend\.env.local created from frontend\.env.windows.example, same
    REPLACE_WITH_SERVER_HOST requirement (NEXT_PUBLIC_* is build-time only,
    so this must exist before this script runs, not after).
  - Postgres running locally and reachable at backend\.env's DATABASE_URL.
    Migrations run automatically when filemepls-api.exe starts.

.USAGE
  Run from an elevated (Administrator) PowerShell prompt, from the repo
  root:
      .\deploy\windows\deploy.ps1
  Re-run any time to rebuild and redeploy after pulling new code.
#>

[CmdletBinding()]
param(
    [string]$NssmPath = "nssm",
    [string]$RepoRoot = (Resolve-Path "$PSScriptRoot\..\..").Path,
    [string]$ApiServiceName = "FilemeplsAPI",
    [string]$WebServiceName = "FilemeplsWeb",
    [string]$NodeExe = (Get-Command node -ErrorAction Stop).Source
)

$ErrorActionPreference = "Stop"

function Assert-Nssm {
    $resolved = Get-Command $NssmPath -ErrorAction SilentlyContinue
    if (-not $resolved) {
        throw "nssm.exe not found (looked for '$NssmPath' on PATH). Download it from https://nssm.cc/ and either add it to PATH or pass -NssmPath <path-to-nssm.exe>."
    }
    return $resolved.Source
}

function Install-OrUpdateService {
    param(
        [string]$Nssm,
        [string]$Name,
        [string]$Exe,
        [string]$Arguments,
        [string]$WorkingDir,
        [string[]]$ExtraEnv = @()
    )

    $exists = (& $Nssm status $Name 2>$null) -ne $null
    if ($exists) {
        Write-Host "Stopping existing service $Name..."
        & $Nssm stop $Name | Out-Null
        & $Nssm remove $Name confirm | Out-Null
    }

    Write-Host "Installing service $Name -> $Exe $Arguments"
    & $Nssm install $Name $Exe $Arguments
    & $Nssm set $Name AppDirectory $WorkingDir
    & $Nssm set $Name AppStdout "$WorkingDir\service-stdout.log"
    & $Nssm set $Name AppStderr "$WorkingDir\service-stderr.log"
    & $Nssm set $Name AppRotateFiles 1
    & $Nssm set $Name Start SERVICE_AUTO_START
    if ($ExtraEnv.Count -gt 0) {
        & $Nssm set $Name AppEnvironmentExtra ($ExtraEnv -join "`r`n")
    }
    & $Nssm start $Name
}

$nssm = Assert-Nssm

# --- Backend: Go API ---
$backendDir = Join-Path $RepoRoot "backend"
$apiExe = Join-Path $backendDir "filemepls-api.exe"

if (-not (Test-Path (Join-Path $backendDir ".env"))) {
    throw "backend\.env not found. Copy backend\.env.windows.example to backend\.env and fill in real values first."
}

Write-Host "`n=== Building backend ==="
Push-Location $backendDir
try {
    $env:CGO_ENABLED = "0"
    go build -o $apiExe ./cmd/api
} finally {
    Pop-Location
}

Install-OrUpdateService -Nssm $nssm -Name $ApiServiceName -Exe $apiExe -Arguments "" -WorkingDir $backendDir

# --- Frontend: Next.js standalone ---
$frontendDir = Join-Path $RepoRoot "frontend"
$standaloneDir = Join-Path $frontendDir ".next\standalone"

if (-not (Test-Path (Join-Path $frontendDir ".env.local"))) {
    throw "frontend\.env.local not found. Copy frontend\.env.windows.example to frontend\.env.local and fill in real values first (NEXT_PUBLIC_* is baked in at build time)."
}

Write-Host "`n=== Building frontend ==="
Push-Location $frontendDir
try {
    npm ci
    npm run build

    # Next.js standalone output needs static assets and public/ copied in
    # manually — they're not included automatically.
    Copy-Item -Recurse -Force ".next\static" "$standaloneDir\.next\static"
    if (Test-Path "public") {
        Copy-Item -Recurse -Force "public" "$standaloneDir\public"
    }
} finally {
    Pop-Location
}

# Next.js standalone's server.js binds to "localhost" only unless told
# otherwise — that would make it unreachable from any other machine on the
# network even though the port looks open locally.
Install-OrUpdateService -Nssm $nssm -Name $WebServiceName -Exe $NodeExe -Arguments "server.js" -WorkingDir $standaloneDir -ExtraEnv @("HOSTNAME=0.0.0.0", "PORT=3000")

Write-Host "`n=== Done ==="
Write-Host "API:      http://localhost:8080  (service: $ApiServiceName)"
Write-Host "Frontend: http://localhost:3000  (service: $WebServiceName)"
Write-Host "Check logs at <service-workdir>\service-std{out,err}.log if something doesn't come up."
