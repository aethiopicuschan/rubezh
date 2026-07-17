package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type listedPackage struct {
	ImportPath   string
	Dir          string
	TestGoFiles  []string
	XTestGoFiles []string
}

type target struct {
	path       string
	importPath string
}

func lint(stderr io.Writer, args []string, cfg config) (violations int, err error) {
	files, patterns := splitArgs(args)
	if len(files) == 0 && len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	targets := make([]target, 0, len(files))
	for _, path := range files {
		targets = append(targets, target{path: path})
	}

	packageTargets, err := filesForPatterns(patterns)
	if err != nil {
		return
	}
	targets = append(targets, packageTargets...)

	seen := make(map[string]struct{}, len(targets))
	for _, current := range targets {
		current.path, err = filepath.Abs(current.path)
		if err != nil {
			err = fmt.Errorf("resolve %q: %w", current.path, err)
			return
		}
		if _, ok := seen[current.path]; ok {
			continue
		}
		seen[current.path] = struct{}{}
		if cfg.excludesFile(current.path) {
			continue
		}

		var violation bool
		violation, err = checkFile(stderr, current, cfg)
		if err != nil {
			return
		}
		if violation {
			violations++
		}
	}

	return
}

func splitArgs(args []string) (files, patterns []string) {
	for _, arg := range args {
		if strings.EqualFold(filepath.Ext(arg), ".go") {
			files = append(files, arg)
		} else {
			patterns = append(patterns, arg)
		}
	}
	return files, patterns
}

func filesForPatterns(patterns []string) (targets []target, err error) {
	if len(patterns) == 0 {
		return
	}

	args := []string{"list", "-json"}
	for _, pattern := range patterns {
		args = append(args, normalizePattern(pattern))
	}
	command := exec.Command("go", args...)
	output, err := command.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("prepare go list: %w", err)
		return
	}
	var commandError strings.Builder
	command.Stderr = &commandError
	if err = command.Start(); err != nil {
		err = fmt.Errorf("start go list: %w", err)
		return
	}

	decoder := json.NewDecoder(bufio.NewReader(output))
	for decoder.More() {
		var pkg listedPackage
		if err = decoder.Decode(&pkg); err != nil {
			_ = command.Wait()
			err = fmt.Errorf("decode go list output: %w", err)
			return
		}
		for _, name := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
			targets = append(targets, target{
				path:       filepath.Join(pkg.Dir, name),
				importPath: pkg.ImportPath,
			})
		}
	}
	if err = command.Wait(); err != nil {
		message := strings.TrimSpace(commandError.String())
		if message != "" {
			err = fmt.Errorf("go list: %s", message)
			return
		}
		err = fmt.Errorf("go list: %w", err)
		return
	}

	return
}

func normalizePattern(pattern string) (normalized string) {
	if filepath.IsAbs(pattern) || strings.HasPrefix(pattern, ".") {
		normalized = pattern
		return
	}

	path := strings.TrimSuffix(pattern, "/...")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		normalized = "./" + pattern
		return
	}
	normalized = pattern
	return
}

func checkFile(stderr io.Writer, current target, cfg config) (ng bool, err error) {
	if !strings.HasSuffix(strings.ToLower(current.path), "_test.go") {
		return
	}
	if filepath.Base(current.path) == "export_test.go" {
		return
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, current.path, nil, parser.PackageClauseOnly)
	if err != nil {
		err = fmt.Errorf("parse %q: %w", current.path, err)
		return
	}
	if cfg.excludesPackage(current.importPath, file.Name.Name) {
		return
	}
	if strings.HasSuffix(file.Name.Name, "_test") {
		return
	}

	position := fileSet.Position(file.Name.Pos())
	fmt.Fprintf(stderr, "%s: package %s must end in _test\n", position, file.Name.Name)
	ng = true
	return
}
