# Exercise the native installer with in-process HTTP fixtures; no network access.
$ErrorActionPreference = 'Stop'
$Installer = Join-Path $PSScriptRoot '../install/install.ps1'
$Root = Join-Path ([IO.Path]::GetTempPath()) ('ep-installer-' + [Guid]::NewGuid())
$SavedVersion = $env:EP_VERSION
$SavedDestination = $env:EP_INSTALL_DIR
$FixtureBytes = [Text.Encoding]::UTF8.GetBytes('test binary')
$FixtureHash = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($FixtureBytes)).ToLowerInvariant()
$FixtureArch = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($FixtureArch -eq 'x64') { $FixtureArch = 'amd64' }
$FixtureAsset = "embedded-postgres_windows_$FixtureArch.exe"
function Invoke-RestMethod($Uri) {
    return @([PSCustomObject]@{tag_name='v1.34.0'}, [PSCustomObject]@{tag_name='v2.0.0-alpha.1'})
}
function Invoke-WebRequest($Uri, $OutFile) {
    if ($Uri.EndsWith('/checksums.txt')) {
        $Hash = if ($script:Corrupt) { '0' * 64 } else { $FixtureHash }
        return [PSCustomObject]@{Content="$Hash  $FixtureAsset`n"}
    }
    if (-not $Uri.EndsWith("/$FixtureAsset")) { throw 'Wrong native asset requested' }
    [IO.File]::WriteAllBytes($OutFile, $FixtureBytes)
}
try {
    foreach ($Case in @('success', 'checksum', 'version', 'directory')) {
        $env:EP_INSTALL_DIR = Join-Path $Root "$Case with spaces"
        $env:EP_VERSION = if ($Case -eq 'version') { '../invalid' } else { '' }
        $script:Corrupt = $Case -eq 'checksum'
        $Target = Join-Path $env:EP_INSTALL_DIR 'embedded-postgres.exe'
        if ($Case -eq 'directory') { New-Item -ItemType Directory -Path $Target -Force | Out-Null }
        $Failed = $false
        try { & $Installer } catch { $Failed = $true }
        if ($Case -eq 'success') {
            if ($Failed -or -not (Test-Path -PathType Leaf $Target)) { throw 'Native install failed' }
            if ((Get-FileHash -Algorithm SHA256 $Target).Hash.ToLowerInvariant() -ne $FixtureHash) { throw 'Installed bytes differ' }
        } elseif (-not $Failed -or (Test-Path -PathType Leaf $Target)) {
            throw "Installer did not safely reject $Case"
        }
    }
    Write-Output 'Native PowerShell installer regressions passed'
} finally {
    $env:EP_VERSION = $SavedVersion
    $env:EP_INSTALL_DIR = $SavedDestination
    if (Test-Path $Root) { Remove-Item -Recurse -Force $Root }
}
