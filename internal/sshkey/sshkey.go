package sshkey

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ListPublicKeys() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	if err != nil {
		return nil, fmt.Errorf("list public keys: %w", err)
	}

	sort.Strings(matches)
	return matches, nil
}

func ReadPublicKey(path string) (string, error) {
	resolved, err := ExpandPath(path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read SSH public key %s: %w", resolved, err)
	}

	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("SSH public key %s is empty", resolved)
	}

	return value, nil
}

func ExpandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}

		if path == "~" {
			return home, nil
		}

		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}

	return path, nil
}
