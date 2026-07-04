<#
.SYNOPSIS
  Aurelia: Beastbound test runner for Windows PowerShell.

.DESCRIPTION
  Native Windows equivalent of the Makefile. Run from the repo root (E:\code\Pokemon).

.PARAMETER Task
  quick   Node-only reference checks (no Go toolchain needed)   [default]
  test    Full Go suite (race detector + coverage gate)
  demo    Play a complete two-bot PvP match end-to-end
  gen     Regenerate the 300-creature seed + balance audit
  ci      vet + test + coverage + node checks
  all     quick + vet + test + demo
  up      Local full stack via Docker Compose

.EXAMPLE
  .\test.ps1            # quick Node checks
  .\test.ps1 test       # full Go suite
  .\test.ps1 all
#>
[CmdletBinding()]
param(
  [ValidateSet('quick','test','demo','gen','ci','all','up','tidy')]
  [string]$Task = 'quick',
  [int]$MinCoverage = 70
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

function Step([string]$name, [scriptblock]$block) {
  Write-Host "==> $name" -ForegroundColor Cyan
  & $block
  if ($LASTEXITCODE -ne 0) { Write-Host "FAILED: $name" -ForegroundColor Red; exit 1 }
}

function Need([string]$exe) {
  if (-not (Get-Command $exe -ErrorAction SilentlyContinue)) {
    Write-Host "Missing required tool: $exe" -ForegroundColor Red; exit 1
  }
}

function Invoke-Quick {
  Need node
  Step 'engine parity'  { node test/reference/verify_engine.js }
  Step 'session parity' { node test/reference/verify_session.mjs }
  Step 'backend parity' { node test/reference/verify_backend.mjs }
  Step 'seed + audit'   { node tools/creaturegen/generate.mjs --seed 42 --count 300 --out data/creatures/seed.json }
}

function Invoke-Gen { Need node; Step 'seed + audit' { node tools/creaturegen/generate.mjs --seed 42 --count 300 --out data/creatures/seed.json } }

function Invoke-Test {
  Need go
  Step 'go vet' { go vet ./... }
  Step 'go test -race' { go test -race -covermode=atomic -coverprofile=cover.out ./... }
  Step 'coverage gate' {
    $line = (go tool cover -func=cover.out | Select-Object -Last 1)
    $pct  = [double]([regex]::Match($line, '([\d.]+)%').Groups[1].Value)
    Write-Host ("coverage: {0}% (min {1}%)" -f $pct, $MinCoverage)
    if ($pct -lt $MinCoverage) { throw "coverage below $MinCoverage%" }
  }
}

function Invoke-Demo { Need go; Step 'e2e battle demo' { go run ./services/battle/cmd/demo } }
function Invoke-Tidy { Need go; Step 'go mod tidy' { go mod tidy } }
function Invoke-Up   { Need docker; Step 'compose up' { docker compose -f deploy/docker/docker-compose.yml up --build } }

switch ($Task) {
  'quick' { Invoke-Quick }
  'gen'   { Invoke-Gen }
  'test'  { Invoke-Test }
  'demo'  { Invoke-Demo }
  'tidy'  { Invoke-Tidy }
  'up'    { Invoke-Up }
  'ci'    { Invoke-Test; Invoke-Quick }
  'all'   { Invoke-Quick; Invoke-Test; Invoke-Demo }
}

Write-Host "`nDONE: $Task" -ForegroundColor Green
