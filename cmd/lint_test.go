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

func TestLint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		files          map[string]string
		wantViolations int
		wantDiagnostic string
	}{
		{
			name: "internal test package",
			files: map[string]string{
				"example_test.go": "package example\n",
			},
			wantViolations: 1,
			wantDiagnostic: "package example must end in _test",
		},
		{
			name: "external test package",
			files: map[string]string{
				"example_test.go": "package example_test\n",
			},
		},
		{
			name: "test export file with internal package",
			files: map[string]string{
				"export_test.go": "package example\n",
			},
		},
		{
			name: "regular Go file",
			files: map[string]string{
				"example.go": "package example\n",
			},
		},
		{
			name: "multiple files",
			files: map[string]string{
				"internal_test.go": "package example\n",
				"external_test.go": "package example_test\n",
				"example.go":       "package example\n",
			},
			wantViolations: 1,
			wantDiagnostic: "package example must end in _test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			paths := make([]string, 0, len(tt.files))
			for name, contents := range tt.files {
				path := filepath.Join(directory, name)
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
				paths = append(paths, path)
			}

			var stderr bytes.Buffer
			violations, err := cmd.Lint(&stderr, paths, cmd.Config{})

			assert.NoError(t, err)
			assert.Equal(t, tt.wantViolations, violations)
			assert.Contains(t, stderr.String(), tt.wantDiagnostic)
		})
	}
}

func TestNormalizePattern(t *testing.T) {
	t.Parallel()

	directory, err := os.MkdirTemp(".", "normalize-pattern-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(directory))
	})
	directoryName := filepath.Base(directory)

	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{
			name:    "existing relative directory",
			pattern: directoryName + "/",
			want:    "./" + directoryName + "/",
		},
		{
			name:    "existing relative directory recursively",
			pattern: directoryName + "/...",
			want:    "./" + directoryName + "/...",
		},
		{
			name:    "explicit relative pattern",
			pattern: "./cmd/",
			want:    "./cmd/",
		},
		{
			name:    "package import path",
			pattern: "example.com/project/package",
			want:    "example.com/project/package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, cmd.NormalizePattern(tt.pattern))
		})
	}
}
