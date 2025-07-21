// Package errors provides error handling utilities for BootScope.
//
// This package contains helper functions for wrapping errors with additional
// context, making it easier to trace error origins through the application.
//
// # Error Wrapping
//
// The package provides two main functions for error handling:
//
// WrapFailure adds operation context:
//
//	data, err := readFile("config.toml")
//	if err != nil {
//	    return errors.WrapFailure("read config", err)
//	}
//	// Error: "failed to read config: open config.toml: no such file"
//
// WrapContext adds descriptive context:
//
//	pod, err := client.GetPod(name)
//	if err != nil {
//	    return errors.WrapContext(err, "while fetching pod '%s'", name)
//	}
//	// Error: "while fetching pod 'nginx': pod not found"
//
// # Best Practices
//
// Should use WrapFailure for operation-level errors:
//   - File operations: "read file", "write config"
//   - Network operations: "connect to API", "fetch pod"
//   - Parsing operations: "parse YAML", "decode JSON"
//
// Should use Use WrapContext for additional debugging information:
//   - Variable values
//   - State information
//   - Timing context
package errors
