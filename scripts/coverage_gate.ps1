param(
  [int]$Threshold = 85
)

$ErrorActionPreference = 'Stop'
go test ./... -coverprofile=coverage.out
$line = go tool cover -func=coverage.out | Select-String 'total:' | Select-Object -Last 1
if (-not $line) { throw 'Unable to parse coverage output.' }
$parts = $line.ToString().Trim() -split '\s+'
$totalText = $parts[-1].TrimEnd('%')
$total = [double]$totalText
Write-Host "Total coverage: $total%"
if ($total -lt $Threshold) {
  throw "Coverage gate failed: $total < $Threshold"
}
