# Viri Blockchain - Windows Testnet Launcher
# Launches a single-node validator for local testing
param(
    [string]$Mode = "single",
    [int]$Nodes = 3,
    [int]$BaseP2PPort = 30303,
    [int]$BaseRPCPort = 8545,
    [string]$DataDir = ".viri-testnet"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Viri Testnet Launcher" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Clean up previous runs
function Stop-Nodes {
    Write-Host "Stopping existing nodes..." -ForegroundColor Yellow
    Get-Process -Name "virid" -ErrorAction SilentlyContinue | Stop-Process -Force
    Write-Host "Done." -ForegroundColor Green
}

# Start a single validator node
function Start-SingleNode {
    $dir = Join-Path $RootDir $DataDir
    if (Test-Path $dir) {
        Write-Host "Cleaning previous data..." -ForegroundColor Yellow
        Remove-Item -Recurse -Force $dir
    }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null

    Write-Host "Starting single validator node..." -ForegroundColor Green
    Write-Host "  Data dir: $dir" -ForegroundColor Gray
    Write-Host "  P2P port: $BaseP2PPort" -ForegroundColor Gray
    Write-Host "  RPC port: $BaseRPCPort" -ForegroundColor Gray
    Write-Host ""

    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = Join-Path $RootDir "virid.exe"
    $startInfo.Arguments = "--validator --data-dir $dir --p2p-port $BaseP2PPort --rpc-port $BaseRPCPort --chain-id 1337 --log-level debug"
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true

    $process = [System.Diagnostics.Process]::Start($startInfo)

    Write-Host "Node PID: $($process.Id)" -ForegroundColor Green
    Write-Host "Waiting for node to initialize..." -ForegroundColor Yellow

    # Wait for RPC to be ready
    $maxWait = 30
    $waited = 0
    while ($waited -lt $maxWait) {
        try {
            $response = Invoke-RestMethod -Uri "http://localhost:${BaseRPCPort}" -Method Post -Body '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -ContentType "application/json" -ErrorAction Stop
            Write-Host "Node ready! Block number: $($response.result)" -ForegroundColor Green
            break
        } catch {
            Start-Sleep -Seconds 1
            $waited++
        }
    }

    if ($waited -ge $maxWait) {
        Write-Host "Warning: Node did not become ready within ${maxWait}s" -ForegroundColor Red
    }

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  Node is running!" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  RPC:  http://localhost:${BaseRPCPort}" -ForegroundColor White
    Write-Host "  API:  http://localhost:$($BaseRPCPort + 1)" -ForegroundColor White
    Write-Host "  WS:   ws://localhost:$($BaseRPCPort + 2)" -ForegroundColor White
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor Yellow
    Write-Host "  Check block: curl http://localhost:${BaseRPCPort} -X POST -d '{`"jsonrpc`":`"2.0`",`"method`":`"eth_blockNumber`",`"params`":[],`"id`":1}'" -ForegroundColor Gray
    Write-Host "  Check peers: curl http://localhost:${BaseRPCPort} -X POST -d '{`"jsonrpc`":`"2.0`",`"method`":`"net_peerCount`",`"params`":[],`"id`":1}'" -ForegroundColor Gray
    Write-Host "  Stop:       Ctrl+C in node window or run: .\start-testnet.ps1 stop" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Press Ctrl+C to stop all nodes..." -ForegroundColor Yellow

    # Stream output
    $process.StandardOutput.BaseStream.CopyTo([System.Console]::OpenStandardOutput())
    $process.WaitForExit()
}

# Start multi-node testnet
function Start-MultiNode {
    Write-Host "Starting ${Nodes}-node testnet..." -ForegroundColor Green
    Write-Host ""

    $bootnodeMultiaddr = ""

    for ($i = 0; $i -lt $Nodes; $i++) {
        $dir = Join-Path $RootDir "${DataDir}/node-$i"
        if (Test-Path $dir) {
            Remove-Item -Recurse -Force $dir
        }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null

        $p2pPort = $BaseP2PPort + $i
        $rpcPort = $BaseRPCPort + ($i * 10)

        $args = "--validator --data-dir $dir --p2p-port $p2pPort --rpc-port $rpcPort --chain-id 1337 --log-level info --name node-$i"

        if ($bootnodeMultiaddr -ne "") {
            $args += " --bootnodes $bootnodeMultiaddr"
        }

        Write-Host "  Starting node-$i (P2P:$p2pPort RPC:$rpcPort)..." -ForegroundColor Gray

        $startInfo = New-Object System.Diagnostics.ProcessStartInfo
        $startInfo.FileName = Join-Path $RootDir "virid.exe"
        $startInfo.Arguments = $args
        $startInfo.UseShellExecute = $false
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true

        $process = [System.Diagnostics.Process]::Start($startInfo)
        Write-Host "    PID: $($process.Id)" -ForegroundColor Gray

        if ($i -eq 0) {
            # Wait for bootnode to get its peer ID (from output)
            Start-Sleep -Seconds 5
            # Parse peer ID from node output or use a placeholder
            $bootnodeMultiaddr = "/ip4/127.0.0.1/tcp/$p2pPort"
            Write-Host "  Bootnode multiaddr: $bootnodeMultiaddr" -ForegroundColor Cyan
            Write-Host "  Note: Multi-node testnet requires manual peer ID exchange for full connectivity" -ForegroundColor Yellow
        }
    }

    Write-Host ""
    Write-Host "All ${Nodes} nodes started!" -ForegroundColor Green
    Write-Host ""

    # Monitor
    Write-Host "Monitoring block production (30 seconds)..." -ForegroundColor Yellow
    for ($s = 1; $s -le 30; $s++) {
        try {
            $response = Invoke-RestMethod -Uri "http://localhost:${BaseRPCPort}" -Method Post -Body '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -ContentType "application/json" -ErrorAction Stop
            Write-Host "  [$s] Block: $($response.result)" -ForegroundColor Green
        } catch {
            Write-Host "  [$s] Node not ready..." -ForegroundColor Red
        }
        Start-Sleep -Seconds 1
    }
}

# Handle commands
switch ($Mode.ToLower()) {
    "stop" {
        Stop-Nodes
    }
    "single" {
        Start-SingleNode
    }
    "multi" {
        Start-MultiNode
    }
    default {
        Write-Host "Usage: .\start-testnet.ps1 [-Mode single|multi|stop] [-Nodes N]" -ForegroundColor Yellow
        Write-Host "  single  - Start a single validator node (default)" -ForegroundColor Gray
        Write-Host "  multi   - Start a multi-node testnet" -ForegroundColor Gray
        Write-Host "  stop    - Stop all running nodes" -ForegroundColor Gray
    }
}
