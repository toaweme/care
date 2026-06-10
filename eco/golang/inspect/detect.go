// Package inspect reads Go project metadata (the module path) from a repository on
// disk.
package inspect

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadModulePath returns the module directive from a repo's go.mod. For
// multi-module workspaces (go.work without a root go.mod), it reads the
// first local submodule's go.mod and strips the subpath to derive the
// root module path.
func ReadModulePath(repo string) (string, error) {
	mod, err := readModuleDirective(filepath.Join(repo, "go.mod"))
	if err == nil {
		return mod, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to open go.mod: %w", err)
	}

	return readModuleFromWorkspace(repo)
}

func readModuleDirective(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to scan go.mod: %w", err)
	}
	return "", errors.New("module directive not found in go.mod")
}

func readModuleFromWorkspace(repo string) (string, error) {
	f, err := os.Open(filepath.Join(repo, "go.work"))
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod or go.work: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "./") {
			continue
		}
		subdir := strings.TrimSuffix(line, ")")
		subdir = strings.TrimSpace(subdir)
		if !strings.HasPrefix(subdir, "./") {
			continue
		}
		mod, err := readModuleDirective(filepath.Join(repo, subdir, "go.mod"))
		if err != nil {
			continue
		}
		suffix := strings.TrimPrefix(subdir, "./")
		if strings.HasSuffix(mod, "/"+suffix) {
			return strings.TrimSuffix(mod, "/"+suffix), nil
		}
		return mod, nil
	}
	return "", errors.New("failed to determine module path from go.work")
}
