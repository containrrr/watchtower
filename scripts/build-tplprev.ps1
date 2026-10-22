# build-tplprev.ps1
# Navigate to repository root
$repoRoot = git rev-parse --show-toplevel
Set-Location -Path $repoRoot

# Create docs/assets directory
New-Item -ItemType Directory -Path "./docs/assets" -Force

# Copy wasm_exec.js from GOROOT/lib/wasm
Write-Output "Copying wasm_exec.js..."
$goRoot = go env GOROOT
$wasmExecPath = "$goRoot/lib/wasm/wasm_exec.js"
if (Test-Path $wasmExecPath) {
    Copy-Item -Path $wasmExecPath -Destination "./docs/assets/wasm_exec.js" -Force
    Write-Output "Copied wasm_exec.js to ./docs/assets/"
}
else {
    Write-Output "wasm_exec.js not found at $wasmExecPath. Attempting to download..."
    $wasmExecUrl = "https://raw.githubusercontent.com/golang/go/master/lib/wasm/wasm_exec.js"
    try {
        Invoke-WebRequest -Uri $wasmExecUrl -OutFile "./docs/assets/wasm_exec.js" -ErrorAction Stop
        Write-Output "Downloaded wasm_exec.js from $wasmExecUrl"
    }
    catch {
        Write-Error "Failed to download wasm_exec.js from $wasmExecUrl. Please manually download it."
        exit 1
    }
}

# Build WASM binary from the nested tplprev module
Write-Output "Building tplprev.wasm..."
$version = git describe --tags --always --dirty 2>$null
if (-not $version) { $version = "dev" }
$commit = git rev-parse HEAD 2>$null
if (-not $commit) { $commit = "none" }
$date = git log -1 --format=%cI 2>$null
if (-not $date) { $date = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ") }
$ldflags = "-X github.com/nicholas-fedor/tplprev/internal/metadata.Version=$version -X github.com/nicholas-fedor/tplprev/internal/metadata.Commit=$commit -X github.com/nicholas-fedor/tplprev/internal/metadata.Date=$date"
$env:GOARCH = "wasm"
$env:GOOS = "js"
go -C ./tools/tplprev build -ldflags $ldflags -o ../../docs/assets/tplprev.wasm .
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to build tplprev.wasm"
    exit $LASTEXITCODE
}

# Verify output
Write-Output "Files in ./docs/assets:"
Get-ChildItem -Path "./docs/assets"
