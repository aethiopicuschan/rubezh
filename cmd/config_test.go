package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/aethiopicuschan/rubezh/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigAndExclude(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		configName   string
		config       string
		testFile     string
		packageName  string
		wantFiles    []string
		wantPackages []string
	}{
		{
			name:       "YAML file exclusion",
			configName: ".rubezh.yaml",
			config: `exclude:
  files:
    - "**/generated_test.go"
`,
			testFile:    "generated_test.go",
			packageName: "example",
			wantFiles:   []string{"**/generated_test.go"},
		},
		{
			name:         "JSON package exclusion",
			configName:   ".rubezh.json",
			config:       `{"exclude":{"packages":["example"]}}`,
			testFile:     "example_test.go",
			packageName:  "example",
			wantPackages: []string{"example"},
		},
		{
			name:       "YML file exclusion",
			configName: ".rubezh.yml",
			config: `exclude:
  files:
    - "ignored_test.go"
`,
			testFile:    "ignored_test.go",
			packageName: "example",
			wantFiles:   []string{"ignored_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			configPath := filepath.Join(directory, tt.configName)
			require.NoError(t, os.WriteFile(configPath, []byte(tt.config), 0o600))
			testPath := filepath.Join(directory, tt.testFile)
			require.NoError(t, os.WriteFile(testPath, []byte("package "+tt.packageName+"\n"), 0o600))

			cfg, err := cmd.LoadConfig(configPath)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFiles, cfg.Exclude.Files)
			assert.Equal(t, tt.wantPackages, cfg.Exclude.Packages)

			var stderr bytes.Buffer
			violations, err := cmd.Lint(&stderr, []string{testPath}, cfg)
			assert.NoError(t, err)
			assert.Zero(t, violations)
			assert.Empty(t, stderr.String())
		})
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configName string
		config     string
	}{
		{
			name:       "YAML",
			configName: ".rubezh.yaml",
			config:     "unknown: true\n",
		},
		{
			name:       "JSON",
			configName: ".rubezh.json",
			config:     `{"unknown":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), tt.configName)
			require.NoError(t, os.WriteFile(path, []byte(tt.config), 0o600))

			_, err := cmd.LoadConfig(path)
			assert.Error(t, err)
		})
	}
}

func TestLoadConfigRejectsInvalidGlob(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".rubezh.yaml")
	require.NoError(t, os.WriteFile(path, []byte("exclude:\n  files: ['[']\n"), 0o600))

	_, err := cmd.LoadConfig(path)
	assert.ErrorContains(t, err, "invalid file exclusion pattern")
}
