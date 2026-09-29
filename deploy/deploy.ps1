param(
    [string]$Server = 'el1s',
    [string]$Go = 'go',
    [string]$ReleaseTag = (Get-Date -Format 'yyyyMMdd-HHmmss')
)
$ErrorActionPreference = 'Stop'
if ($ReleaseTag -notmatch '^[a-zA-Z0-9-]+$') { throw 'Invalid release tag' }
if ($Server -notmatch '^[a-zA-Z0-9_.@-]+$') { throw 'Invalid SSH host' }
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot
& $Go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
& $Go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
$stage = Join-Path $projectRoot "bin/deploy-$ReleaseTag"
New-Item -ItemType Directory -Force -Path $stage | Out-Null
$oldOS = $env:GOOS; $oldArch = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
try {
    $env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    & $Go build -trimpath '-ldflags=-s -w' -o (Join-Path $stage 'tksu-pl') ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'Linux build failed' }
} finally {
    $env:GOOS = $oldOS; $env:GOARCH = $oldArch; $env:CGO_ENABLED = $oldCGO
}
Copy-Item -LiteralPath 'deploy/Dockerfile.runtime' -Destination $stage
Copy-Item -LiteralPath 'deploy/compose.production.yaml' -Destination $stage
git rev-parse HEAD 2>$null | Set-Content -LiteralPath (Join-Path $stage 'REVISION')
$archive = Join-Path $projectRoot "bin/tksu-pl-$ReleaseTag.tar.gz"
tar -czf $archive -C $stage .
if ($LASTEXITCODE -ne 0) { throw 'Archive failed' }
scp $archive "${Server}:/tmp/tksu-pl-$ReleaseTag.tar.gz"
if ($LASTEXITCODE -ne 0) { throw 'Upload failed' }
scp 'deploy/remote-deploy.sh' "${Server}:/tmp/tksu-pl-remote-deploy.sh"
if ($LASTEXITCODE -ne 0) { throw 'Script upload failed' }
ssh $Server "sudo -n bash /tmp/tksu-pl-remote-deploy.sh $ReleaseTag"
if ($LASTEXITCODE -ne 0) { throw 'Remote deployment failed; previous image is retained for rollback.' }

