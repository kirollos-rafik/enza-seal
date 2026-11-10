# enza-seal installer for Windows (no admin rights needed).
#   irm https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main/install.ps1 | iex
#
# What it does: downloads the release for this PC, checks its SHA-256 against
# checksums.txt, copies enza-seal.exe to %LOCALAPPDATA%\enza-seal, and adds
# that folder to your user PATH.
#
# Optional settings (environment variables):
#   ENZA_SEAL_VERSION      install a specific version, e.g. 1.0.0 (default: latest)
#   ENZA_SEAL_INSTALL_DIR  install somewhere else
#   ENZA_SEAL_BASE_URL     download from another location (a URL or a folder)
#   ENZA_SEAL_NO_PATH=1    do not change your PATH

$ErrorActionPreference = 'Stop'
try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch {}
$ProgressPreference = 'SilentlyContinue'

$repo = 'kirollos-rafik/enza-seal'

switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "Unsupported processor: $($env:PROCESSOR_ARCHITECTURE)" }
}

$base = $env:ENZA_SEAL_BASE_URL
if (-not $base) {
    if ($env:ENZA_SEAL_VERSION) {
        $base = "https://github.com/$repo/releases/download/v$($env:ENZA_SEAL_VERSION.TrimStart('v'))"
    } else {
        $base = "https://github.com/$repo/releases/latest/download"
    }
}

$installDir = $env:ENZA_SEAL_INSTALL_DIR
if (-not $installDir) { $installDir = Join-Path $env:LOCALAPPDATA 'enza-seal' }

$asset = "enza-seal_windows_$arch.zip"
$work = Join-Path ([IO.Path]::GetTempPath()) ("enza-seal-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null

function Get-Asset($name) {
    $dest = Join-Path $work $name
    if ($base -match '^https?://') {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile $dest
    } else {
        Copy-Item -LiteralPath (Join-Path $base $name) -Destination $dest
    }
    return $dest
}

try {
    Write-Host "Downloading enza-seal for windows/$arch ..."
    $zip  = Get-Asset $asset
    $sums = Get-Asset 'checksums.txt'

    $line = Get-Content $sums | Where-Object { $_ -match ("\s" + [regex]::Escape($asset) + "$") } | Select-Object -First 1
    if (-not $line) { throw "No checksum listed for $asset. Stopping." }
    $expected = ($line -split '\s+')[0].ToLower()
    $actual   = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLower()
    if ($expected -ne $actual) { throw "Checksum does not match. The download may be damaged or tampered with. Stopping." }
    Write-Host 'Checksum OK'

    Expand-Archive -LiteralPath $zip -DestinationPath $work -Force
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $target = Join-Path $installDir 'enza-seal.exe'
    if (Test-Path $target) {
        # A running copy cannot be overwritten, but it can be renamed.
        Move-Item -Force -LiteralPath $target -Destination "$target.old"
    }
    Copy-Item -Force -LiteralPath (Join-Path $work 'enza-seal.exe') -Destination $target
    Remove-Item -Force -ErrorAction SilentlyContinue "$target.old"
    Write-Host "Installed to $installDir"

    if ($env:ENZA_SEAL_NO_PATH -ne '1') {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $parts = @()
        if ($userPath) { $parts = $userPath.Split(';') | Where-Object { $_ } }
        if ($parts -notcontains $installDir) {
            [Environment]::SetEnvironmentVariable('Path', (($parts + $installDir) -join ';'), 'User')
            Write-Host 'Added to your PATH. Open a new terminal window to use it.'
        }
        if (($env:Path.Split(';')) -notcontains $installDir) { $env:Path = "$env:Path;$installDir" }
    }

    Write-Host ''
    & $target --version
    Write-Host 'enza-seal installed. Run: enza-seal init'
    Write-Host 'Note: if Windows shows "Windows protected your PC", click More info, then Run anyway.'
}
finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $work
}
