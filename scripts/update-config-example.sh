#!/usr/bin/env bash
# Update the example configuration file with the latest from code

set -e

echo "Updating bootscope.toml.example..."

# Use go run to avoid needing to build first
go run ./cmd/bootscope config generate bootscope.toml.example

echo "✅ Updated bootscope.toml.example"
echo ""
echo "Please commit this file if you've made changes to the configuration structure."
