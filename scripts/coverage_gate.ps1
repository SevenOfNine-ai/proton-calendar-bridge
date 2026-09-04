param(
  [int]$Threshold = 85
)

$ErrorActionPreference = 'Stop'

# The profile path is absolute and derived from this script's own location, not
# from the caller's working directory. `go test -coverprofile=coverage.out`
# writes relative to the process CWD, and `go tool cover` then read it relative
# to the CWD too — agreeing only as long as nothing between them differs. On the
# Windows runner they did not agree: every package reported coverage and the
# next line was `cover: open coverage.out: The system cannot find the file
# specified`. Naming one absolute path removes the assumption rather than
# restating it.
$repoRoot = Split-Path -Parent $PSScriptRoot
$profilePath = Join-Path $repoRoot 'coverage.out'
Remove-Item -LiteralPath $profilePath -ErrorAction SilentlyContinue

go test ./... -coverprofile="$profilePath"
if ($LASTEXITCODE -ne 0) {
  throw "go test failed with exit code $LASTEXITCODE"
}

# `go test` reporting `ok` for every package is not evidence that the profile
# was written; that was exactly the state this script could not distinguish from
# a parse failure, and it reported the wrong cause for months. Check the
# artefact, and say what was actually on disk when it is missing.
if (-not (Test-Path -LiteralPath $profilePath)) {
  Write-Host "Expected coverage profile at: $profilePath"
  Write-Host "Working directory: $(Get-Location)"
  Write-Host "Repo root contents:"
  Get-ChildItem -LiteralPath $repoRoot | Select-Object -ExpandProperty Name | ForEach-Object { Write-Host "  $_" }
  throw "go test reported success but wrote no coverage profile."
}

$coverOutput = go tool cover -func="$profilePath"
if ($LASTEXITCODE -ne 0) {
  throw "go tool cover failed with exit code $LASTEXITCODE"
}

$line = $coverOutput | Select-String 'total:' | Select-Object -Last 1
if (-not $line) {
  Write-Host "go tool cover produced no total: line. Full output follows."
  $coverOutput | ForEach-Object { Write-Host "  $_" }
  throw 'Unable to parse coverage output.'
}

$parts = $line.ToString().Trim() -split '\s+'
$totalText = $parts[-1].TrimEnd('%')
$total = [double]$totalText
Write-Host "Total coverage: $total%"
if ($total -lt $Threshold) {
  throw "Coverage gate failed: $total < $Threshold"
}
