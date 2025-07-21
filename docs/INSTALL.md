# Installation Guide

BootScope is a kubectl plugin that works on Linux, macOS, and Windows.

## Prerequisites

- kubectl installed and configured
- Kubernetes cluster access with permissions to read pods and events
- Go 1.23.10+ (only for building from source)

## Install Pre-built Binary

### Linux

```bash
# Download (choose your architecture)
curl -LO https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-linux-amd64
# or for ARM64:
# curl -LO https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-linux-arm64

# Install
chmod +x kubectl-bootscope-linux-*
sudo mv kubectl-bootscope-linux-* /usr/local/bin/kubectl-bootscope

# Verify
kubectl bootscope version
```

### macOS

```bash
# Download (choose your architecture)
# Intel Macs:
curl -LO https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-darwin-amd64
# Apple Silicon (M1/M2):
# curl -LO https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-darwin-arm64

# Install
chmod +x kubectl-bootscope-darwin-*
sudo mv kubectl-bootscope-darwin-* /usr/local/bin/kubectl-bootscope

# Verify
kubectl bootscope version
```

### Windows

PowerShell (as Administrator):

```powershell
# Download
Invoke-WebRequest -Uri "https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-windows-amd64.exe" -OutFile "kubectl-bootscope.exe"

# Create plugin directory
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.kube\plugins\bootscope"

# Move to plugins directory
Move-Item -Path "kubectl-bootscope.exe" -Destination "$env:USERPROFILE\.kube\plugins\bootscope\kubectl-bootscope.exe"

# Add to PATH permanently
[Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";$env:USERPROFILE\.kube\plugins\bootscope", [EnvironmentVariableTarget]::User)

# Restart PowerShell and verify
kubectl bootscope version
```

## Shell Completion

### Bash

```bash
# Linux
kubectl bootscope completion bash | sudo tee /etc/bash_completion.d/kubectl-bootscope > /dev/null

# macOS (requires bash-completion from Homebrew)
kubectl bootscope completion bash > $(brew --prefix)/etc/bash_completion.d/kubectl-bootscope
```

### Zsh

```bash
# Add to ~/.zshrc
echo 'source <(kubectl bootscope completion zsh)' >> ~/.zshrc
source ~/.zshrc
```

### Fish

```bash
kubectl bootscope completion fish > ~/.config/fish/completions/kubectl-bootscope.fish
```

### PowerShell

```powershell
# Add to profile
kubectl bootscope completion powershell >> $PROFILE
```

## Build from Source

```bash
# Using Go
go install github.com/px4n/bootscope/cmd/bootscope@latest

# Or clone and build
git clone https://github.com/px4n/bootscope
cd bootscope
make install
```

## Troubleshooting

### "command not found"

- Ensure the binary is in your PATH: `echo $PATH`
- Check if executable: `ls -la /usr/local/bin/kubectl-bootscope`
- Try full path: `/usr/local/bin/kubectl-bootscope version`

### kubectl doesn't recognize the plugin

The binary must be:

- Named `kubectl-bootscope` (or `kubectl-bootscope.exe` on Windows)
- In your PATH
- Executable

Verify with: `kubectl plugin list`

### Permission denied

```bash
# Linux/macOS
chmod +x /usr/local/bin/kubectl-bootscope

# Windows: Run PowerShell as Administrator
```

### Build for other architectures

```bash
# Examples
GOOS=linux GOARCH=arm64 go build -o kubectl-bootscope-linux-arm64 ./cmd/bootscope
GOOS=freebsd GOARCH=amd64 go build -o kubectl-bootscope-freebsd-amd64 ./cmd/bootscope
```

## Next Steps

- Generate configuration: `kubectl bootscope config generate`
- Analyze a pod: `kubectl bootscope analyze pod <pod-name>`
- See help: `kubectl bootscope --help`
