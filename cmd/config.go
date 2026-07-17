package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

var configNames = []string{
	".rubezh.yaml",
	".rubezh.yml",
	".rubezh.json",
}

type config struct {
	Exclude exclusions `json:"exclude" yaml:"exclude"`
	baseDir string
}

type exclusions struct {
	Files    []string `json:"files" yaml:"files"`
	Packages []string `json:"packages" yaml:"packages"`
}

func loadConfig(path string) (cfg config, err error) {
	if path == "" {
		path, err = findConfig()
		if err != nil {
			return
		}
	}

	if path == "" {
		cfg.baseDir, err = os.Getwd()
		return
	}

	path, err = filepath.Abs(path)
	if err != nil {
		err = fmt.Errorf("resolve config path: %w", err)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("read config %q: %w", path, err)
		return
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		err = decodeJSON(data, &cfg)
	case ".yaml", ".yml":
		err = decodeYAML(data, &cfg)
	default:
		err = fmt.Errorf("unsupported config format %q: use .json, .yaml, or .yml", filepath.Ext(path))
	}
	if err != nil {
		err = fmt.Errorf("parse config %q: %w", path, err)
		return
	}
	if err = cfg.validate(); err != nil {
		err = fmt.Errorf("validate config %q: %w", path, err)
		return
	}

	cfg.baseDir = filepath.Dir(path)
	return
}

func (cfg config) validate() (err error) {
	for _, pattern := range cfg.Exclude.Files {
		if !doublestar.ValidatePattern(filepath.ToSlash(pattern)) {
			err = fmt.Errorf("invalid file exclusion pattern %q", pattern)
			return
		}
	}
	for _, pattern := range cfg.Exclude.Packages {
		if !doublestar.ValidatePattern(pattern) {
			err = fmt.Errorf("invalid package exclusion pattern %q", pattern)
			return
		}
	}
	return
}

func findConfig() (path string, err error) {
	for _, name := range configNames {
		_, statErr := os.Stat(name)
		if statErr == nil {
			path = name
			return
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			err = fmt.Errorf("find config %q: %w", name, statErr)
			return
		}
	}
	return
}

func decodeJSON(data []byte, cfg *config) (err error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(cfg); err != nil {
		return
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return
	}
	err = nil
	return
}

func decodeYAML(data []byte, cfg *config) (err error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err = decoder.Decode(cfg); err != nil {
		return
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple YAML documents")
		}
		return
	}
	err = nil
	return
}

func (cfg config) excludesFile(path string) (excluded bool) {
	path, err := filepath.Abs(path)
	if err != nil {
		return
	}
	relative, err := filepath.Rel(cfg.baseDir, path)
	if err != nil {
		return
	}
	relative = filepath.ToSlash(relative)
	absolute := filepath.ToSlash(path)

	for _, pattern := range cfg.Exclude.Files {
		candidate := relative
		if filepath.IsAbs(pattern) {
			candidate = absolute
		}
		pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
		if excluded, _ = doublestar.Match(pattern, candidate); excluded {
			return
		}
	}
	return
}

func (cfg config) excludesPackage(importPath, name string) (excluded bool) {
	for _, pattern := range cfg.Exclude.Packages {
		if excluded, _ = doublestar.Match(pattern, importPath); excluded {
			return
		}
		if excluded, _ = doublestar.Match(pattern, name); excluded {
			return
		}
	}
	return
}
