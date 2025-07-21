# BootScope

> **⚠️ WARNING: ALPHA SOFTWARE - NOT FOR PRODUCTION USE**
>
> This is still in ALPHA stage and may contain bugs, incomplete features, and breaking changes.
> Use at your own risk and please [report any issues](https://github.com/px4n/bootscope/issues).

A kubectl plugin that analyzes pod startup times to identify bottlenecks. It shows why pods take time to start and suggests improvements.

## What it does

- Shows each phase of pod startup (scheduling, image pull, init containers, etc.)
- Identifies which phases are taking the longest
- Provides suggestions for common issues
- Can analyze single pods or entire deployments
- Supports JSON/YAML output for scripting
- Has a watch mode to follow pods as they start

## Installation

```bash
# Quick install (Linux/macOS)
curl -LO https://github.com/px4n/bootscope/releases/latest/download/kubectl-bootscope-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/')
chmod +x kubectl-bootscope-*
sudo mv kubectl-bootscope-* /usr/local/bin/kubectl-bootscope

# Verify
kubectl bootscope version
```

For Windows and detailed instructions, see [docs/INSTALL.md](docs/INSTALL.md)

## Usage

```bash
# Analyze a single pod
kubectl bootscope analyze pod nginx-deployment-7d4b7c6-x2f3h

# Analyze all pods in a deployment
kubectl bootscope analyze deployment nginx-deployment

# Watch a pod as it starts
kubectl bootscope analyze pod my-pod --watch

# Get simple, developer-friendly output
kubectl bootscope analyze pod my-pod --simple

# Output as JSON for processing
kubectl bootscope analyze pod my-pod -o json
```

### Common Scenarios

**Pod stuck starting?**

```bash
kubectl bootscope analyze pod stuck-pod
```

**Deployment slow to roll out?**

```bash
kubectl bootscope analyze deployment my-deployment
```

**Want real-time analysis?**

```bash
kubectl bootscope analyze pod my-pod --watch
```

## Configuration

BootScope uses TOML configuration files.
Generate a documented default by:

```bash
kubectl bootscope config generate
```

The tool searches for config in: `./bootscope.toml`, `~/.kube/bootscope.toml`, or via `--config` flag.

Key settings include:

- **Thresholds**: When to flag phases as slow (image pulls, init containers, etc.)
- **Display**: Output formatting and color coding
- **Registry**: Patterns for detecting local vs remote registries

See the generated file for all options with documentation.

## Example Output

**Single Container Pod:**

```
Pod Startup Profile: default/nginx-7d4b7c6-x2f3h
Total Time: 12s ✅
Status: Running

Phase Breakdown:
├─ Scheduling: 150ms (1%)
├─ ImagePull: 8s (67%) ⚠️
└─ ApplicationStart: 3s (25%) - Application initialization (nginx)
```

**Multi-Container Pod with Init Containers:**

```
Pod Startup Profile: default/payment-service-7d4b7c6-x2f3h
Total Time: 2m7s 🐌
Status: Running

Phase Breakdown:
├─ Scheduling: 234ms (0%)
├─ ImagePull: 1m18s (61%) ⚠️
├─ InitContainers: 43s (34%) ⚠️
│  ├─ db-migration: 37s
│  └─ config-gen: 6s
└─ ApplicationStart: 4s (3%) - Application initialization (api, worker, metrics)

Bottlenecks Identified:
🚨 ImagePull took 61% of total startup time
⚠️ InitContainers took 34% of total startup time

Recommendations:
1. Optimize Image Pull Time
   - Consider using a local registry or pre-pulled images
   - Optimize image size with multi-stage builds
   Impact: Could save ~39s
```

## Limitations

- Requires Kubernetes events (which are deleted after ~1 hour)
- Can only estimate image sizes based on pull time
- Cannot detect runtime CPU/memory throttling
- Only works within a single pod lifecycle (not across pod deletions)

## Contributing

Contributions are welcome! Please:

1. Install pre-commit hooks: `pre-commit install`
2. Follow [Conventional Commits](https://www.conventionalcommits.org/)
3. Run tests: `make test`
4. Check code quality: `make check`

See [docs/COMMIT_GUIDE.md](docs/COMMIT_GUIDE.md) for detailed guidelines.

## License

Apache License - see [LICENSE](LICENSE) file for details.
