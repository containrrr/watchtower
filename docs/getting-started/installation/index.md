# Installation

## Sources

### Container Image

- [Docker Hub](https://hub.docker.com/r/nickfedor/watchtower){target="_blank" rel="noopener noreferrer"}
- [GitHub Container Registry](https://github.com/nicholas-fedor/watchtower/pkgs/container/watchtower){target="_blank" rel="noopener noreferrer"}

### Binary

- [GitHub Releases](https://github.com/nicholas-fedor/watchtower/releases){target="_blank" rel="noopener noreferrer"}

## CLI Commands

### Pulling the Container Image

=== "Docker Hub"

    ```bash
    docker pull nickfedor/watchtower:latest
    ```

=== "GitHub"

    ```bash
    docker pull ghcr.io/nicholas-fedor/watchtower:latest
    ```

### Downloading the Binary

The following will download and extract the binary to the current directory:

=== "Windows (amd64)"

    ```powershell title="PowerShell"
    iwr (iwr https://api.github.com/repos/nicholas-fedor/watchtower/releases/latest | ConvertFrom-Json).assets.where({$_.name -like "*windows_amd64*.zip"}).browser_download_url -OutFile watchtower.zip; Add-Type -AssemblyName System.IO.Compression.FileSystem; $zip = [System.IO.Compression.ZipFile]::OpenRead("$PWD\watchtower.zip"); $zip.Entries | Where-Object {$_.Name -eq 'watchtower.exe'} | ForEach-Object {[System.IO.Compression.ZipFileExtensions]::ExtractToFile($_, "$PWD\watchtower.exe", $true)}; $zip.Dispose(); Remove-Item watchtower.zip; if (Test-Path ".\watchtower.exe") { Write-Host "Successfully installed watchtower.exe to current directory" } else { Write-Host "Failed to install watchtower.exe" }
    ```

=== "Linux (amd64)"

    ```bash title="Bash"
    curl -L $(curl -s https://api.github.com/repos/nicholas-fedor/watchtower/releases/latest | grep -o 'https://[^"]*linux_amd64[^"]*\.tar\.gz') | tar -xz -C . watchtower && if [ -f ./watchtower ]; then echo "Successfully installed watchtower to current directory"; else echo "Failed to install watchtower"; fi
    ```

=== "macOS (amd64)"

    ```bash title="Bash"
    curl -L $(curl -s https://api.github.com/repos/nicholas-fedor/watchtower/releases/latest | grep -o 'https://[^"]*darwin_amd64[^"]*\.tar\.gz') | tar -xz -C . watchtower && if [ -f ./watchtower ]; then echo "Successfully installed watchtower to current directory"; else echo "Failed to install watchtower"; fi
    ```

!!! Note
    Review the [release page](https://github.com/nicholas-fedor/watchtower/releases){target="_blank" rel="noopener noreferrer"} for additional architectures (e.g., arm, arm64, i386, riscv64).

## Supported Architectures

Watchtower supports the following architectures:

- amd64
- i386
- armhf
- arm64v8
- riscv64
