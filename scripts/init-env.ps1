$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $projectRoot '.env'
if (Test-Path -LiteralPath $envPath) { throw '.env already exists; edit it manually to preserve the session secret.' }
$bytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$secret = ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
$content = [System.IO.File]::ReadAllText((Join-Path $projectRoot '.env.example')).Replace('SESSION_SECRET=', "SESSION_SECRET=$secret")
[System.IO.File]::WriteAllText($envPath, $content, [System.Text.UTF8Encoding]::new($false))
Write-Output 'Created .env with a random session secret. Run: go run .'

