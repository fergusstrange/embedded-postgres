# Native Windows alternative. EP_VERSION pins a release; EP_INSTALL_DIR chooses its destination.
$ErrorActionPreference = 'Stop'
$Repo = 'fergusstrange/embedded-postgres'
$Version = $env:EP_VERSION
if (-not $Version) {
    $Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases?per_page=100" | Where-Object { $_.tag_name -match '^v2\.' } | Select-Object -First 1).tag_name
}
if ($Version -notmatch '^v2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') { throw 'No published v2 release found; set EP_VERSION.' }
$Arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($Arch -eq 'x64') { $Arch = 'amd64' }
if ($Arch -notin @('amd64', 'arm64')) { throw 'Supported architectures are AMD64 and ARM64.' }
$Destination = $env:EP_INSTALL_DIR
if (-not $Destination) { $Destination = Join-Path $env:LOCALAPPDATA 'embedded-postgres\bin' }
New-Item -ItemType Directory -Force -Path $Destination | Out-Null
$Asset = "embedded-postgres_windows_$Arch.exe"
$Base = "https://github.com/$Repo/releases/download/$Version"
$Stage = Join-Path $Destination ([Guid]::NewGuid().ToString() + '.tmp')
try {
    Invoke-WebRequest "$Base/$Asset" -OutFile $Stage
    $Checksums = (Invoke-WebRequest "$Base/checksums.txt").Content
    $Lines = @($Checksums -split "`n" | Where-Object { $_ -match ('^[a-f0-9]{64}\s+' + [regex]::Escape($Asset) + '\s*$') })
    if ($Lines.Count -ne 1) { throw 'Missing or ambiguous SHA-256 checksum.' }
    $Expected = ($Lines[0] -split '\s+')[0]
    if ((Get-FileHash -Algorithm SHA256 $Stage).Hash.ToLowerInvariant() -ne $Expected) { throw 'SHA-256 mismatch; installation refused.' }
    Move-Item -Force $Stage (Join-Path $Destination 'embedded-postgres.exe')
    Write-Output "Installed $Version to $Destination"
} finally {
    if (Test-Path $Stage) { Remove-Item $Stage }
}
