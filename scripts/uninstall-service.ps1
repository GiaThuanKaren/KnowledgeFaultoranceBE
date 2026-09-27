$ErrorActionPreference = "SilentlyContinue"

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "Uninstalling Knowledge Faultorance Backend Service" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

# Step 1: Stop running backend process
Write-Host "`n[1/2] Terminating running backend process..." -ForegroundColor Yellow
Get-Process -Name "server" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Write-Host "Backend process terminated." -ForegroundColor Green

# Step 2: Remove from Windows Startup folder
Write-Host "`n[2/2] Removing auto-start launcher from Windows Startup..." -ForegroundColor Yellow
$startupFolder = [Environment]::GetFolderPath('Startup')
$vbsPath = Join-Path $startupFolder "KnowledgeFaultoranceBE.vbs"

if (Test-Path $vbsPath) {
    Remove-Item -Path $vbsPath -Force
    Write-Host "Removed: $vbsPath" -ForegroundColor Green
} else {
    Write-Host "Startup launcher was not found (already clean)." -ForegroundColor Gray
}

Write-Host "`n============================================================" -ForegroundColor Cyan
Write-Host "Uninstallation Complete!" -ForegroundColor Green
Write-Host "- Backend service removed from Windows startup." -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan
