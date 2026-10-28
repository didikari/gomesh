# GoMesh Agent Windows Installer & Service Registrar
# Run in Administrator PowerShell:
#   powershell -ExecutionPolicy Bypass -File install-windows.ps1 -ServerUrl "ws://<server-host>:8080/ws/agent" -Token "<token>"

param (
    [string]$ServerUrl = "ws://localhost:8080/ws/agent",
    [string]$Token = "default-token",
    [string]$InstallDir = "C:\Program Files\GoMesh",
    [switch]$Uninstall
)

$ServiceName = "GoMeshAgent"
$DisplayName = "GoMesh Remote Management Agent"
$BinaryName = "gomesh-agent.exe"

# Verify Administrator Privileges
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "[!] Error: Administrator privileges required. Please right-click PowerShell and choose 'Run as Administrator'."
    exit 1
}

if ($Uninstall) {
    Write-Host "[*] Stopping and removing $ServiceName..."
    sc.exe stop $ServiceName | Out-Null
    Start-Sleep -Seconds 2
    sc.exe delete $ServiceName | Out-Null
    Write-Host "[+] $ServiceName uninstalled successfully."
    exit 0
}

Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "   GoMesh Windows Agent Service Installer        " -ForegroundColor Cyan
Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "Target Server : $ServerUrl"
Write-Host "Install Dir   : $InstallDir"
Write-Host "================================================="

# Create directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$AgentPath = Join-Path $InstallDir $BinaryName

# Derive HTTP URL for download
$HttpBase = $ServerUrl -replace "^ws://", "http://" -replace "^wss://", "https://" -replace "/ws/agent$", ""
$DownloadUrl = "$HttpBase/bin/$BinaryName"

Write-Host "[*] Downloading agent binary from $DownloadUrl..."
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $AgentPath -UseBasicParsing -ErrorAction Stop
    Write-Host "[+] Download completed successfully." -ForegroundColor Green
} catch {
    Write-Warning "[!] Could not download from server ($DownloadUrl). Checking local directory..."
    if (Test-Path ".\bin\$BinaryName") {
        Copy-Item ".\bin\$BinaryName" $AgentPath -Force
        Write-Host "[+] Copied binary from local .\bin\$BinaryName" -ForegroundColor Green
    } elseif (Test-Path ".\$BinaryName") {
        Copy-Item ".\$BinaryName" $AgentPath -Force
        Write-Host "[+] Copied binary from local .\$BinaryName" -ForegroundColor Green
    } else {
        Write-Error "[!] Agent binary not found. Please place $BinaryName in $InstallDir"
        exit 1
    }
}

# Stop and remove existing service if present
sc.exe stop $ServiceName 2>$null | Out-Null
sc.exe delete $ServiceName 2>$null | Out-Null
Start-Sleep -Seconds 1

# Register Windows Service cleanly with sc.exe (Antivirus friendly, no packer)
Write-Host "[*] Registering Windows Service ($ServiceName)..."
$binPathArg = "`"$AgentPath`" -server `"$ServerUrl`" -token `"$Token`" -config `"$InstallDir`""

$createRes = sc.exe create $ServiceName binPath= $binPathArg start= auto DisplayName= $DisplayName
if ($LASTEXITCODE -ne 0) {
    Write-Error "[!] Failed to create Windows service: $createRes"
    exit 1
}

# Set service description & failure recovery actions (restart on failure)
sc.exe description $ServiceName "GoMesh background agent for secure remote device management and telemetry."
sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/60000

# Start service
Write-Host "[*] Starting $ServiceName..."
sc.exe start $ServiceName

Write-Host ""
Write-Host "[+] GoMesh Agent service is now installed and running!" -ForegroundColor Green
Write-Host "    Service Name: $ServiceName"
Write-Host "    Check Status: Get-Service $ServiceName"
