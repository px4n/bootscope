package errors

import "fmt"

// WrapFailure wraps an error with a standardized failure message
func WrapFailure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// WrapContext adds context to an error
func WrapContext(context string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
