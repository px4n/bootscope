# BootScope Installation Script for Windows
# Automatically detects architecture and installs the appropriate binary

[CmdletBinding()]
param(
    [Parameter(HelpMessage="Version to install (default: latest)")]
    [string]$Version = "latest",

    [Parameter(HelpMessage="Installation directory")]
    [string]$InstallDir = "$env:USERPROFILE\.kube\plugins\bootscope",

    [Parameter(HelpMessage="Skip checksum verification")]
    [switch]$SkipChecksum,

    [Parameter(HelpMessage="Force installation even if already installed")]
    [switch]$Force,

    [Parameter(HelpMessage="Show help information")]
    [switch]$Help
)

# Configuration
$ErrorActionPreference = "Stop"
$ProgressPreference = 'Continue'

$repo = "px4n/bootscope"
$binaryName = "kubectl-bootscope"
$githubApi = "https://api.github.com"
$githubUrl = "https://github.com"

# Show help if requested
if ($Help) {
    Write-Host @"
BootScope Installation Script for Windows

USAGE:
    .\install.ps1 [OPTIONS]

OPTIONS:
    -Version <VERSION>      Install specific version (default: latest)
    -InstallDir <PATH>      Installation directory (default: $env:USERPROFILE\.kube\plugins\bootscope)
    -SkipChecksum          Skip checksum verification
    -Force                 Force installation even if already installed
    -Help                  Show this help message

EXAMPLES:
    # Install latest version
    .\install.ps1

    # Install specific version
    .\install.ps1 -Version v0.2.0

    # Install to custom directory
    .\install.ps1 -InstallDir C:\Tools\kubectl-plugins

    # Skip checksum verification
    .\install.ps1 -SkipChecksum

"@
    exit 0
}

# Helper functions
function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "Warning: $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "Error: $Message" -ForegroundColor Red
    exit 1
}

# Function to test if running as administrator
function Test-Administrator {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Function to get file hash
function Get-FileHashString {
    param([string]$FilePath)
    try {
        $hash = Get-FileHash -Path $FilePath -Algorithm SHA256 -ErrorAction Stop
        return $hash.Hash.ToLower()
    } catch {
        return $null
    }
}

Write-Info "BootScope Installer for Windows"
Write-Host ""

# Check prerequisites
if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
    Write-Warn "kubectl is not installed. BootScope is a kubectl plugin and requires kubectl to function."
    Write-Host "  Visit https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/ for installation instructions."
    Write-Host ""
}

# Detect architecture
$arch = "x86_64"  # Default to x86_64 for Windows
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $arch = "arm64"
} elseif ($env:PROCESSOR_ARCHITECTURE -eq "x86") {
    $arch = "386"
} elseif ($env:PROCESSOR_ARCHITECTURE -eq "ARM") {
    $arch = "arm"
}

Write-Host "  Architecture: $arch"
Write-Host "  Install directory: $InstallDir"
Write-Host "  Version: $Version"
Write-Host ""

# Check if already installed
$existingPath = Join-Path $InstallDir "$binaryName.exe"
if ((Test-Path $existingPath) -and -not $Force) {
    try {
        $existingVersion = & $existingPath version 2>&1
        Write-Warn "BootScope is already installed at $existingPath"
        Write-Host "  Current version: $existingVersion"
        Write-Host "  Use -Force to reinstall"
        exit 0
    } catch {
        Write-Warn "Existing installation found but could not determine version"
    }
}

# Strip 'v' prefix from version if present for filename
$versionNum = $Version -replace '^v', ''

# Construct filenames with version
$filename = "$binaryName-$versionNum-windows-$arch.exe"
$installName = "$binaryName.exe"

# Create temp directory
$tempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
Push-Location $tempDir

try {
    # Get version information
    if ($Version -eq "latest") {
        Write-Info "Fetching latest release information..."

        try {
            $headers = @{
                "Accept" = "application/vnd.github.v3+json"
                "User-Agent" = "BootScope-Installer"
            }

            $releaseInfo = Invoke-RestMethod -Uri "$githubApi/repos/$repo/releases/latest" -Headers $headers -TimeoutSec 30
            $Version = $releaseInfo.tag_name

            if (-not $Version) {
                Write-Error "Could not determine latest version. Please specify a version with -Version"
            }
        } catch {
            Write-Error "Failed to fetch latest release information: $_"
        }
    }

    Write-Info "Installing BootScope $Version..."

    # Construct download URLs
    $downloadUrl = "$githubUrl/$repo/releases/download/$Version/$filename"
    $checksumUrl = "$githubUrl/$repo/releases/download/$Version/checksums.txt"

    # Download binary
    Write-Host "Downloading $filename..."
    $outputPath = Join-Path $tempDir $filename

    try {
        # Show progress while downloading
        $webClient = New-Object System.Net.WebClient
        $webClient.Headers.Add("User-Agent", "BootScope-Installer")

        # Register event for progress updates
        $progressActivity = "Downloading BootScope"
        Register-ObjectEvent -InputObject $webClient -EventName DownloadProgressChanged -Action {
            $percent = $EventArgs.ProgressPercentage
            if ($percent -gt 0) {
                Write-Progress -Activity $progressActivity -Status "$percent% Complete" -PercentComplete $percent
            }
        } | Out-Null

        # Download file
        $webClient.DownloadFile($downloadUrl, $outputPath)
        Write-Progress -Activity $progressActivity -Completed

    } catch {
        Write-Error "Failed to download binary: $_"
    }

    # Verify download
    if (-not (Test-Path $outputPath)) {
        Write-Error "Download failed - file not found"
    }

    # Check file size (should be at least 1MB for a Go binary)
    $fileInfo = Get-Item $outputPath
    if ($fileInfo.Length -lt 1048576) {
        Write-Error "Downloaded file is too small ($($fileInfo.Length) bytes). This may indicate a failed download."
    }

    # Verify checksum if not skipped
    if (-not $SkipChecksum) {
        Write-Info "Verifying checksum..."

        try {
            # Download checksums file
            $checksumPath = Join-Path $tempDir "checksums.txt"
            $webClient.DownloadFile($checksumUrl, $checksumPath)

            # Read checksums file
            $checksums = Get-Content $checksumPath

            # Find the checksum for our file
            $expectedChecksum = $null
            foreach ($line in $checksums) {
                if ($line -match "^([a-f0-9]{64})\s+$([regex]::Escape($filename))$") {
                    $expectedChecksum = $matches[1]
                    break
                }
            }

            if ($expectedChecksum) {
                # Calculate actual checksum
                $actualChecksum = Get-FileHashString -FilePath $outputPath

                if ($actualChecksum -eq $expectedChecksum) {
                    Write-Info "Checksum verified successfully"
                } else {
                    Write-Error "Checksum verification failed. The downloaded file may be corrupted."
                }
            } else {
                Write-Warn "Could not find checksum for $filename in checksums file"
            }

        } catch {
            Write-Warn "Could not download or verify checksums: $_"
            Write-Warn "To enable checksum verification, ensure checksums.txt is available in the release."
        }
    } else {
        Write-Warn "Skipping checksum verification"
    }

    # Verify it's a valid PE executable
    try {
        $bytes = [System.IO.File]::ReadAllBytes($outputPath)
        if ($bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) { # MZ header
            Write-Error "Downloaded file does not appear to be a valid Windows executable"
        }
    } catch {
        Write-Warn "Could not verify executable format"
    }

    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        Write-Info "Creating installation directory: $InstallDir"
        try {
            New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        } catch {
            Write-Error "Failed to create installation directory: $_"
        }
    }

    # Install binary
    Write-Info "Installing to $InstallDir\$installName..."
    $installPath = Join-Path $InstallDir $installName

    try {
        # If file exists, remove it first
        if (Test-Path $installPath) {
            Remove-Item $installPath -Force
        }

        # Move file to installation directory
        Move-Item -Path $outputPath -Destination $installPath -Force

    } catch {
        Write-Error "Failed to install binary: $_"
    }

    # Add to PATH if not already there
    $currentUserPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
    $pathArray = $currentUserPath -split ';' | Where-Object { $_ -ne '' }

    if ($InstallDir -notin $pathArray) {
        Write-Info "Adding to PATH..."

        try {
            # Clean up PATH - remove duplicates and empty entries
            $cleanPath = ($pathArray + $InstallDir | Select-Object -Unique) -join ';'

            [Environment]::SetEnvironmentVariable(
                "PATH",
                $cleanPath,
                [EnvironmentVariableTarget]::User
            )

            # Update current session
            $env:PATH = "$env:PATH;$InstallDir"

            Write-Info "Added $InstallDir to PATH"
        } catch {
            Write-Warn "Failed to update PATH: $_"
            Write-Host "You may need to add $InstallDir to your PATH manually"
        }
    } else {
        Write-Info "Directory already in PATH"
    }

    # Verify installation
    Write-Host ""
    Write-Info "Verifying installation..."

    # Try to run the command
    try {
        $versionOutput = & $installPath version 2>&1
        Write-Info "✅ Installation successful!"
        Write-Host ""
        Write-Host $versionOutput
        Write-Host ""

        Write-Info "To enable PowerShell completion:"
        Write-Host "  kubectl bootscope completion powershell | Out-String | Invoke-Expression"
        Write-Host ""
        Write-Info "To make completion permanent, add the above line to your PowerShell profile:"
        Write-Host "  notepad `$PROFILE"
        Write-Host ""
        Write-Info "Try: kubectl bootscope --help"

    } catch {
        Write-Warn "Installation completed but could not verify."
        Write-Host "You may need to restart your terminal for PATH changes to take effect."
        Write-Host ""
        Write-Host "To verify installation manually, run:"
        Write-Host "  $installPath version"
    }

} catch {
    Write-Error "Installation failed: $_"
} finally {
    # Cleanup
    Pop-Location
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
