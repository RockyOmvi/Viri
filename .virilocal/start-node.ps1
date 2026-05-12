$env:VIRI_KEY_PASSPHRASE = "test"
$key = (Get-Content -Raw "D:\blockchain\.virilocal\genesis\validator.key").Trim()
$dataDir = "D:\blockchain\.virilocal\data"
$null = New-Item -ItemType Directory -Path $dataDir -Force
$logFile = "$dataDir\node.log"
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
) -RedirectStandardOutput $logFile -RedirectStandardError $logFile -WindowStyle Hidden
Start-Sleep -Seconds 3
$proc = Get-Process -Name virid -ErrorAction SilentlyContinue
if ($proc) {
    Write-Host "OK PID=$($proc.Id)"
} else {
    Write-Host "FAILED"
    if (Test-Path $logFile) { Get-Content $logFile -Tail 20 }
}
