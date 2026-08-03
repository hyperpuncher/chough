package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	target, err := findTargetBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(target, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = filepath.Dir(target)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: failed to run chough-bin.exe: %v\n", err)
		os.Exit(1)
	}
}

func findTargetBinary() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}
	return findTargetBinaryFrom(execPath)
}

func findTargetBinaryFrom(execPath string) (string, error) {
	candidateDirs := uniqueDirs([]string{filepath.Dir(execPath)})
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		candidateDirs = uniqueDirs(append(candidateDirs, filepath.Dir(resolved)))
	}

	for _, dir := range candidateDirs {
		target := filepath.Join(dir, "chough-bin.exe")
		if _, err := os.Stat(target); err == nil {
			return target, nil
		}
	}

	return "", fmt.Errorf("chough-bin.exe not found (checked %d locations)", len(candidateDirs))
}

func uniqueDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		clean := filepath.Clean(dir)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}
