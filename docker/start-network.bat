@echo off
REM start-network.bat - Start Viri blockchain network with bootstrapping

echo === Viri Blockchain Network Startup ===
echo.

cd /d "%~dp0.."

set PROJECT_DIR=%CD%

if "%1"=="start" goto start
if "%1"=="stop" goto stop
if "%1"=="restart" goto restart
if "%1"=="clean" goto clean
if "%1"=="status" goto status
if "%1"=="logs" goto logs
if "%1"=="" goto start

echo Usage: %0 {start^|stop^|restart^|clean^|status^|logs [service]}
exit /b 1

:start
echo Building containers...
docker compose -f docker\docker-compose.yml build

echo Starting bootstrap node...
docker compose -f docker\docker-compose.yml up -d bootstrap

echo Waiting for bootstrap node to start...
timeout /t 10 /nobreak >nul

for /f "tokens=2 delims==" %%a in ('docker compose -f docker\docker-compose.yml logs bootstrap 2^>nul ^| findstr "peer_id="') do (
    set BOOTSTRAP_PEER_ID=%%a
    goto got_peer_id
)

echo ERROR: Could not get bootstrap peer ID
exit /b 1

:got_peer_id
echo Bootstrap peer ID: %BOOTSTRAP_PEER_ID%
echo.

echo Starting validators and explorer...
set BOOTSTRAP_PEER_ID=%BOOTSTRAP_PEER_ID%
docker compose -f docker\docker-compose.yml up -d

echo.
echo === Network Started ===
echo Bootstrap: localhost:30300
echo Validator 1: localhost:30301 (RPC: localhost:8541)
echo Validator 2: localhost:30302 (RPC: localhost:8542)
echo Validator 3: localhost:30303 (RPC: localhost:8543)
echo Validator 4: localhost:30304 (RPC: localhost:8544)
echo Explorer: localhost:8545
echo.
echo To check network status:
echo   docker compose -f docker\docker-compose.yml ps
echo   docker compose -f docker\docker-compose.yml logs -f
goto end

:stop
echo Stopping network...
docker compose -f docker\docker-compose.yml down
echo Network stopped.
goto end

:restart
call %0 stop
call %0 start
goto end

:clean
echo Stopping and cleaning network...
docker compose -f docker\docker-compose.yml down -v
docker compose -f docker\docker-compose.yml rm -f
echo Network cleaned.
goto end

:status
echo === Network Status ===
docker compose -f docker\docker-compose.yml ps
echo.
echo === Bootstrap Logs (last 20 lines) ===
docker compose -f docker\docker-compose.yml logs --tail=20 bootstrap
goto end

:logs
docker compose -f docker\docker-compose.yml logs -f %2
goto end

:end
