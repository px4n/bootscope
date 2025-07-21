# Commit Message Guidelines

I've written this to allow other developers to commit in a unified way and also to remind myself. This project uses [Conventional Commits](https://www.conventionalcommits.org/) to standardize commit messages and enable automated versioning.

## Quick Setup

1. Install pre-commit (one-time setup):

```bash
# macOS
brew install pre-commit golangci-lint

# Or using pip and go
pip install pre-commit
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

2. Install the git hooks:

```bash
pre-commit install
pre-commit install --hook-type commit-msg
```

Once done the hooks will now run automatically before each commit.

## Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc)
- **refactor**: Code changes that neither fix bugs nor add features
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **chore**: Changes to build process or auxiliary tools
- **ci**: CI/CD configuration changes
- **revert**: Reverting a previous commit

### Examples

```bash
# Simple commits
git commit -m "feat: add support for StatefulSet analysis"
git commit -m "fix: correct phase timing calculation"
git commit -m "docs: update installation guide for Windows"

# With scope
git commit -m "feat(analyzer): detect volume mount delays"
git commit -m "fix(output): handle nil pod status gracefully"

# Breaking changes (triggers major version)
git commit -m "feat!: change config format to TOML"
git commit -m "fix(api)!: rename StartupProfile to PodProfile"

# Multi-line with body
git commit -m "fix: handle pods with no events

Some pods may have their events garbage collected.
Added nil checks and fallback behavior."
```

## Pre-commit Checks

The following checks run automatically at different stages:

**Before commit (pre-commit stage):**

- `gofmt` - Formats Go code
- `go vet` - Reports suspicious constructs
- `golangci-lint` - Comprehensive Go linting
- `go test -short` - Runs unit tests
- Trailing whitespace removal
- End-of-file fixer
- YAML syntax validation
- Large file prevention (>1MB)

**After entering commit message (commit-msg stage):**

- Conventional commit format validation

This staged approach ensures each check runs only once at the appropriate time.

### Bypassing Checks (Emergency Only)

```bash
# Skip pre-commit hooks
git commit --no-verify -m "fix: emergency patch"

# But please run checks manually afterward:
pre-commit run --all-files
```

## Troubleshooting

If commit hooks aren't working:

```bash
# Reinstall hooks
pre-commit uninstall
pre-commit install
pre-commit install --hook-type commit-msg

# Update pre-commit
pre-commit autoupdate

# Run manually
pre-commit run --all-files
```
