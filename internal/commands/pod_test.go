package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateKubernetesName(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		resourceType string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid pod name",
			input:        "my-pod-123",
			resourceType: "pod",
			wantErr:      false,
		},
		{
			name:         "valid namespace",
			input:        "default",
			resourceType: "namespace",
			wantErr:      false,
		},
		{
			name:         "single character valid",
			input:        "a",
			resourceType: "pod",
			wantErr:      false,
		},
		{
			name:         "empty name",
			input:        "",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "cannot be empty",
		},
		{
			name:         "name with uppercase",
			input:        "MyPod",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "must be lowercase alphanumeric",
		},
		{
			name:         "name with spaces",
			input:        "my pod",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "must be lowercase alphanumeric",
		},
		{
			name:         "name with special characters",
			input:        "my_pod!",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "must be lowercase alphanumeric",
		},
		{
			name:         "name starting with dash",
			input:        "-mypod",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "must be lowercase alphanumeric",
		},
		{
			name:         "name ending with dash",
			input:        "mypod-",
			resourceType: "pod",
			wantErr:      true,
			errContains:  "must be lowercase alphanumeric",
		},
		{
			name:         "name too long",
			input:        string(make([]byte, 254)),
			resourceType: "namespace",
			wantErr:      true,
			errContains:  "too long",
		},
		{
			name:         "max length name",
			input:        "a" + strings.Repeat("b", 251) + "c", // 253 chars
			resourceType: "namespace",
			wantErr:      false,
		},
		{
			name:         "DNS subdomain format",
			input:        "my-app-v1-2-3",
			resourceType: "pod",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKubernetesName(tt.input, tt.resourceType)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
