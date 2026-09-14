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

# Detect Windows architecture (amd64 or arm64)
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

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

# 1. Permanently update User PATH in Windows Registry
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = if ([string]::IsNullOrEmpty($UserPath)) { $InstallDir } else { "$UserPath;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::User)
    Write-Host "Added $InstallDir to Windows User PATH (Registry)." -ForegroundColor Green
}

# 2. Update current PowerShell session so command is immediately available
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

# 3. Broadcast WM_SETTINGCHANGE so running windows/terminals detect new PATH
try {
    Add-Type -Namespace Win32 -Name NativeMethods -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $HWND_BROADCAST = [IntPtr]0xffff
    $WM_SETTINGCHANGE = 0x001A
    $result = [UIntPtr]::Zero
    [Win32.NativeMethods]::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result) | Out-Null
} catch {}

Write-Host "`nEcho installation complete and globally accessible!" -ForegroundColor Green
Write-Host "Run 'echo --help' to get started."
