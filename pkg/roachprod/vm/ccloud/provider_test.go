// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package ccloud

import (
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderName(t *testing.T) {
	p := &Provider{}
	assert.Equal(t, ProviderName, p.Name())
}

func TestProviderActive(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "active with API key",
			apiKey:   "test-api-key",
			expected: true,
		},
		{
			name:     "inactive without API key",
			apiKey:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				apiKey: tt.apiKey,
			}
			assert.Equal(t, tt.expected, p.Active())
		})
	}
}

func TestNewProvider(t *testing.T) {
	// Save and restore original environment
	originalAPIKey := os.Getenv(apiKeyEnvVar)
	defer func() {
		if originalAPIKey != "" {
			os.Setenv(apiKeyEnvVar, originalAPIKey)
		} else {
			os.Unsetenv(apiKeyEnvVar)
		}
	}()

	t.Run("with API key", func(t *testing.T) {
		os.Setenv(apiKeyEnvVar, "test-api-key")
		p, err := NewProvider()
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "test-api-key", p.apiKey)
	})

	t.Run("without API key", func(t *testing.T) {
		os.Unsetenv(apiKeyEnvVar)
		p, err := NewProvider()
		require.Error(t, err)
		assert.Nil(t, p)
	})
}

func TestIsCentralizedProvider(t *testing.T) {
	p := &Provider{}
	assert.True(t, p.IsCentralizedProvider())
}

func TestProjectActive(t *testing.T) {
	p := &Provider{}
	// Cockroach Cloud uses single account model
	assert.True(t, p.ProjectActive("any-project"))
}

func TestSupportsSpotVMs(t *testing.T) {
	p := &Provider{}
	assert.False(t, p.SupportsSpotVMs())
}

func TestProviderInterface(t *testing.T) {
	// Verify that Provider implements vm.Provider interface
	var _ vm.Provider = &Provider{}
}
