$ErrorActionPreference = 'Stop'

$repository = 'epismoai/cli'
$version = if ($env:EPISMO_VERSION) { $env:EPISMO_VERSION } else { 'latest' }
$installDir = if ($env:EPISMO_INSTALL_DIR) { $env:EPISMO_INSTALL_DIR } else { Join-Path $HOME '.local\bin' }
$architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $_" }
}

if ($version -eq 'latest') {
    $releaseUrl = "https://github.com/$repository/releases/latest/download"
} else {
    if (-not $version.StartsWith('v')) { $version = "v$version" }
    $releaseUrl = "https://github.com/$repository/releases/download/$version"
}
if ($env:EPISMO_RELEASE_BASE_URL) { $releaseUrl = $env:EPISMO_RELEASE_BASE_URL.TrimEnd('/') }

$archive = "epismo_windows_$architecture.zip"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("epismo-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
    Invoke-WebRequest "$releaseUrl/$archive" -OutFile (Join-Path $tempDir $archive)
    Invoke-WebRequest "$releaseUrl/checksums.txt" -OutFile (Join-Path $tempDir 'checksums.txt')
    $checksumLine = Get-Content (Join-Path $tempDir 'checksums.txt') | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" }
    if (-not $checksumLine) { throw "Checksum not found for $archive" }
    $expected = ($checksumLine -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash (Join-Path $tempDir $archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw 'Checksum verification failed' }
    Expand-Archive (Join-Path $tempDir $archive) -DestinationPath $tempDir
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $installedExecutable = Join-Path $installDir 'epismo.exe'
    Copy-Item (Join-Path $tempDir 'epismo.exe') $installedExecutable -Force
    $installedVersion = (& $installedExecutable --version).Trim()
    $receipt = [ordered]@{
        schemaVersion = 1
        method = 'powershell'
        installedVersion = $installedVersion
    } | ConvertTo-Json
    $receiptPath = "$installedExecutable.install.json"
    $receiptTemp = Join-Path $installDir ('.epismo-install-' + [guid]::NewGuid() + '.tmp')
    [System.IO.File]::WriteAllText($receiptTemp, $receipt + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $receiptTemp -Destination $receiptPath -Force
    Write-Host "Installed epismo to $installedExecutable"
} finally {
    if (Test-Path -LiteralPath $tempDir) {
        $resolvedTemp = (Resolve-Path -LiteralPath $tempDir -ErrorAction Stop).Path
        $systemTemp = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
        if (-not $resolvedTemp.StartsWith($systemTemp, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing unsafe temporary directory cleanup: $resolvedTemp"
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force -ErrorAction Stop
    }
}
