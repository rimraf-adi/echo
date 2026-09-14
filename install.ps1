$ErrorActionPreference = 'Stop'

$Repo = "rimraf-adi/echo"
$BinName = "echo.exe"
$InstallDir = "$env:LOCALAPPDATA\echo\bin"

Write-Host "Detecting latest release for $Repo..." -ForegroundColor Cyan

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $ReleaseUrl = "https://api.github.com/repos/$Repo/releases/latest"
    $Release = Invoke-RestMethod -Uri $ReleaseUrl -UseBasicParsing
    $LatestTag = $Release.tag_name
} catch {
    Write-Error "Could not retrieve latest release from GitHub: $_"
    exit 1
}

$Arch = "amd64"
$ArchiveName = "echo-$LatestTag-windows-$Arch.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$ArchiveName"

Write-Host "Downloading Echo $LatestTag for Windows ($Arch)..." -ForegroundColor Cyan
$TempZip = "$env:TEMP\$ArchiveName"

Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Expand-Archive -Path $TempZip -DestinationPath $InstallDir -Force
Remove-Item -Path $TempZip -Force

Write-Host "Installed $BinName to $InstallDir" -ForegroundColor Green

# Update User PATH environment variable
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = if ([string]::IsNullOrEmpty($UserPath)) { $InstallDir } else { "$UserPath;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::User)
    Write-Host "Added $InstallDir to User PATH." -ForegroundColor Green
}

# Update current session PATH
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

Write-Host "`nEcho installation complete!" -ForegroundColor Green
Write-Host "Open a new terminal or run 'echo --help' to get started."
