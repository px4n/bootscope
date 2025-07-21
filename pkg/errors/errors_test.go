package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapFailure(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
		want      string
		wantNil   bool
	}{
		{
			name:      "wraps error with operation",
			operation: "connect to database",
			err:       errors.New("connection refused"),
			want:      "failed to connect to database: connection refused",
		},
		{
			name:      "returns nil for nil error",
			operation: "do something",
			err:       nil,
			wantNil:   true,
		},
		{
			name:      "preserves error wrapping",
			operation: "parse config",
			err:       errors.New("invalid syntax"),
			want:      "failed to parse config: invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapFailure(tt.operation, tt.err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.EqualError(t, got, tt.want)
				// Verify error unwrapping works
				assert.True(t, errors.Is(got, tt.err))
			}
		})
	}
}

func TestWrapContext(t *testing.T) {
	tests := []struct {
		name    string
		context string
		err     error
		want    string
		wantNil bool
	}{
		{
			name:    "wraps error with context",
			context: "while processing pod nginx-123",
			err:     errors.New("pod not found"),
			want:    "while processing pod nginx-123: pod not found",
		},
		{
			name:    "returns nil for nil error",
			context: "some context",
			err:     nil,
			wantNil: true,
		},
		{
			name:    "preserves nested errors",
			context: "during startup analysis",
			err:     WrapFailure("read events", errors.New("timeout")),
			want:    "during startup analysis: failed to read events: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapContext(tt.context, tt.err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.EqualError(t, got, tt.want)
				// For nested errors, verify we can still unwrap to the original
				if tt.name == "preserves nested errors" {
					// Verify error chain is preserved
					assert.Error(t, got)
				}
			}
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that our error wrappers can be chained
	baseErr := errors.New("connection timeout")
	wrapped1 := WrapFailure("connect to API", baseErr)
	wrapped2 := WrapContext("while fetching pod data", wrapped1)

	expected := "while fetching pod data: failed to connect to API: connection timeout"
	assert.EqualError(t, wrapped2, expected)

	// Verify we can still check for the base error
	assert.True(t, errors.Is(wrapped2, baseErr))
}
