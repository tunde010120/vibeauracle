# vibe auracle Windows Installer
# Usage: iex (irm https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.ps1)

$ErrorActionPreference = "Stop"

$Repo = "nathfavour/vibeauracle"
$GithubUrl = "https://github.com/$Repo"

# Detect Architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

$BinaryName = "vibeaura-windows-$Arch.exe"

Write-Host "Detected Platform: Windows/$Arch" -ForegroundColor Cyan

# Get latest release tag
$ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$LatestTag = $ReleaseInfo.tag_name

if (-not $LatestTag) {
    Write-Error "Could not find latest release. Please check $GithubUrl/releases"
}

# Check if vibeaura is already installed and up-to-date
$ExistingVibe = $null
if (Get-Command vibeaura -ErrorAction SilentlyContinue) {
    $ExistingVibe = (Get-Command vibeaura).Source
} elseif (Test-Path "$HOME\.vibeaura\bin\vibeaura.exe") {
    $ExistingVibe = "$HOME\.vibeaura\bin\vibeaura.exe"
}

if ($ExistingVibe) {
    $VersionOutput = (& $ExistingVibe version)
    $VersionLine = ($VersionOutput | Select-String "Version")
    $CommitLine = ($VersionOutput | Select-String "Commit")
    
    if ($VersionLine -and $CommitLine) {
        $LocalVersion = $VersionLine.ToString().Split(":")[1].Trim()
        $LocalCommit = $CommitLine.ToString().Split(":")[1].Trim()
        
        # Resolve the SHA of the latest tag to be sure
        $LatestSHA = $null
        if (Get-Command git -ErrorAction SilentlyContinue) {
            $Match = (git ls-remote --tags "$GithubUrl.git" | Select-String "refs/tags/$LatestTag$")
            if ($Match) {
                $LatestSHA = $Match.ToString().Split("`t")[0].Trim()
            }
        }

        # If the local version matches the latest tag, OR the local commit matches the latest SHA, we can skip
        if (($LocalVersion -eq $LatestTag) -or ($null -ne $LatestSHA -and $LocalCommit -eq $LatestSHA)) {
            Write-Host "Vibe Auracle is already up to date ($LatestTag / $($LocalCommit.Substring(0,7)))." -ForegroundColor Green
            return
        }
    }
}

$DownloadUrl = "$GithubUrl/releases/download/$LatestTag/$BinaryName"

Write-Host "Downloading $BinaryName ($LatestTag)..." -ForegroundColor Cyan

$InstallDir = Join-Path $HOME ".vibeaura\bin"
if (-not (Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory | Out-Null
}

$ExePath = Join-Path $InstallDir "vibeaura.exe"

Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath

# Add to Path for current session
if ($env:Path -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to User Path..." -ForegroundColor Yellow
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    }
    $env:Path += ";$InstallDir"
}

Write-Host "Successfully installed vibe auracle to $ExePath" -ForegroundColor Green
Write-Host "You may need to restart your terminal for changes to take effect." -ForegroundColor Yellow
& "$ExePath" --help
