# Writes a GNU-style "<sha256>  <zip-basename>" checksum file (zip.sha256) next to the zip.
# Used by `make release` on native Windows where Get-FileHash may be unavailable.
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$ZipPath
)

if (-not (Test-Path -LiteralPath $ZipPath -PathType Leaf)) {
    Write-Error "zip not found: $ZipPath"
    exit 1
}

$hash = (Get-FileHash -LiteralPath $ZipPath -Algorithm SHA256).Hash.ToLowerInvariant()
$name = [System.IO.Path]::GetFileName($ZipPath)
"$hash  $name" | Out-File -LiteralPath ($ZipPath + ".sha256") -Encoding ascii -NoNewline
