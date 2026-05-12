$env:VIRI_KEY_PASSPHRASE = "test"
$key = (Get-Content -Raw "D:\blockchain\.virilocal\genesis\validator.key").Trim()
$dataDir = "D:\blockchain\.virilocal\data"
$null = New-Item -ItemType Directory -Path $dataDir -Force
$logFile = "$dataDir\node.log"
$errFile = "$dataDir\error.log"
Start-Process -FilePath "D:\blockchain\virid.exe" -ArgumentList @(
    "--genesis", "D:\blockchain\.virilocal\genesis\genesis.json",
    "--private-key", $key,
    "--data-dir", $dataDir,
    "--rpc-port", "8545",
    "--p2p-port", "30303",
    "--name", "dev-node",
    "--log-level", "info",
    "--validator",
    "--no-mdns",
    "--rpc",
    "--api"
) -RedirectStandardOutput $logFile -RedirectStandardError $errFile -WindowStyle Hidden
Start-Sleep -Seconds 5
$proc = Get-Process -Name virid -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne $pid }
if ($proc) {
    Write-Host "OK PID=$($proc.Id) Running=$($proc.Responding)"
} else {
    Write-Host "FAILED"
    if (Test-Path $errFile) { Write-Host "=== STDERR ==="; Get-Content $errFile -Tail 10 }
    if (Test-Path $logFile) { Write-Host "=== STDOUT ==="; Get-Content $logFile -Tail 10 }
}
