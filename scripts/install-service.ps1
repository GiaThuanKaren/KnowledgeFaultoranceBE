param(
    [string]$Port = "5005"
)

$ErrorActionPreference = "Stop"

$repoDir = (Get-Item $PSScriptRoot).Parent.FullName
$binDir = Join-Path $repoDir "bin"
$exePath = Join-Path $binDir "server.exe"

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "Installing Knowledge Faultorance Backend Service on Windows" -ForegroundColor Cyan
Write-Host "Port:     $Port" -ForegroundColor Cyan
Write-Host "Repo Dir: $repoDir" -ForegroundColor Cyan
Write-Host "Binary:   $exePath" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

# Step 1: Build the Go binary
Write-Host "`n[1/4] Building Go backend binary..." -ForegroundColor Yellow
Set-Location $repoDir
go build -ldflags="-s -w" -o "bin/server.exe" ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Error "Go build failed with code $LASTEXITCODE"
    exit $LASTEXITCODE
}
Write-Host "Build successful: bin/server.exe" -ForegroundColor Green

# Step 2: Stop any currently running instance of server.exe
Write-Host "`n[2/4] Stopping any existing backend instance..." -ForegroundColor Yellow
Get-Process -Name "server" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

# Step 3: Register in Windows Startup folder (runs hidden 100% on Windows login, zero admin required)
Write-Host "`n[3/4] Registering auto-start on Windows login..." -ForegroundColor Yellow
$startupFolder = [Environment]::GetFolderPath('Startup')
$vbsPath = Join-Path $startupFolder "KnowledgeFaultoranceBE.vbs"
$batPath = Join-Path $binDir "run-service.bat"

$batContent = @"
@echo off
cd /d "$repoDir"
bin\server.exe -port $Port > bin\server.log 2>&1
"@
[System.IO.File]::WriteAllText($batPath, $batContent, [System.Text.Encoding]::ASCII)

$vbsContent = @"
Set WshShell = CreateObject("WScript.Shell")
WshShell.CurrentDirectory = "$repoDir"
WshShell.Run """$batPath""", 0, False
"@
[System.IO.File]::WriteAllText($vbsPath, $vbsContent, [System.Text.Encoding]::ASCII)
Write-Host "Created startup launcher at: $vbsPath" -ForegroundColor Green

# Also save a copy inside bin/ for direct manual launch if desired
$localVbsPath = Join-Path $binDir "run-hidden.vbs"
[System.IO.File]::WriteAllText($localVbsPath, $vbsContent, [System.Text.Encoding]::ASCII)

# Step 4: Start the service right now in the background (detached from console)
Write-Host "`n[4/4] Starting backend service in the background..." -ForegroundColor Yellow
Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
    CommandLine      = "cmd.exe /c `"$batPath`""
    CurrentDirectory = $repoDir
} | Out-Null

Write-Host "Waiting for service to initialize..."
Start-Sleep -Seconds 2

# Probe health endpoint
$healthUrl = "http://localhost:$Port/health"
Write-Host "Probing $healthUrl ..."
try {
    $res = Invoke-RestMethod -Uri $healthUrl -Method Get -TimeoutSec 5
    Write-Host "SUCCESS: Backend is running healthy on port $Port! Status: $($res.status)" -ForegroundColor Green
} catch {
    Write-Host "Backend started. (Health probe: $($_.Exception.Message))" -ForegroundColor Yellow
}

Write-Host "`n============================================================" -ForegroundColor Cyan
Write-Host "Installation Complete!" -ForegroundColor Green
Write-Host "- Auto-runs on Windows startup at port $Port" -ForegroundColor Green
Write-Host "- To uninstall anytime, run: .\uninstall.bat or ./uninstall.sh" -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan
