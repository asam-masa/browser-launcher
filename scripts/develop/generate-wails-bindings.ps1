$ErrorActionPreference = 'Stop'

$repositoryRoot = Resolve-Path (Join-Path $PSScriptRoot '../..')
$frontendDist = Join-Path $repositoryRoot 'frontend/dist'
$placeholder = Join-Path $frontendDist 'index.html'

Push-Location $repositoryRoot
try {
    if (-not (Test-Path $placeholder)) {
        New-Item -ItemType Directory -Force -Path $frontendDist | Out-Null
        Set-Content -Path $placeholder -Value '<!doctype html><title>binding generation placeholder</title>'
    }

    go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 generate module
    if ($LASTEXITCODE -ne 0) {
        throw "Wails binding generation failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}
