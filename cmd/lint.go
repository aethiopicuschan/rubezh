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
	Dir          string
	TestGoFiles  []string
	XTestGoFiles []string
}

func lint(stderr io.Writer, args []string) (violations int, err error) {
	files, patterns := splitArgs(args)
	if len(files) == 0 && len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	packageFiles, err := filesForPatterns(patterns)
	if err != nil {
		return
	}
	files = append(files, packageFiles...)

	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		path, err = filepath.Abs(path)
		if err != nil {
			err = fmt.Errorf("resolve %q: %w", path, err)
			return
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		var violation bool
		violation, err = checkFile(stderr, path)
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

func filesForPatterns(patterns []string) (files []string, err error) {
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
			files = append(files, filepath.Join(pkg.Dir, name))
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

func checkFile(stderr io.Writer, path string) (ng bool, err error) {
	if !strings.HasSuffix(strings.ToLower(path), "_test.go") {
		return
	}
	if filepath.Base(path) == "export_test.go" {
		return
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.PackageClauseOnly)
	if err != nil {
		err = fmt.Errorf("parse %q: %w", path, err)
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
