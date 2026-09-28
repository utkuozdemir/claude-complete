package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CachePath returns the path of the cached spec, `${XDG_CACHE_HOME:-~/.cache}/claude-complete/spec.json`.
func CachePath() (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to find the cache directory: %w", err)
		}

		dir = filepath.Join(home, ".cache")
	}

	return filepath.Join(dir, "claude-complete", "spec.json"), nil
}

// Load returns the spec of the installed claude, from the cache at path when it is for that version, and from
// a fresh walk otherwise. An empty version is asked from claude.
func Load(ctx context.Context, run Runner, path, version string, refresh bool) (*Spec, error) {
	if version == "" {
		var err error

		if version, err = Version(ctx, run); err != nil {
			return nil, err
		}
	}

	if cached, err := read(path); !refresh && err == nil && cached.Version == version && cached.Root.valid() {
		return cached, nil
	}

	root, err := Walk(ctx, run)
	if err != nil {
		return nil, err
	}

	fresh := &Spec{Version: version, Root: root}

	_ = write(path, fresh) // failing only costs another walk

	return fresh, nil
}

func read(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read the cached spec: %w", err)
	}

	var cached Spec
	if err = json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to decode the cached spec: %w", err)
	}

	return &cached, nil
}

// write replaces the file at path atomically.
func write(path string, fresh *Spec) error {
	data, err := json.Marshal(fresh)
	if err != nil {
		return fmt.Errorf("failed to encode the spec: %w", err)
	}

	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create the cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".spec-*.json")
	if err != nil {
		return fmt.Errorf("failed to write the cached spec: %w", err)
	}

	defer os.Remove(tmp.Name()) // only left over on failure

	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("failed to write the cached spec: %w", err)
	}

	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to replace the cached spec: %w", err)
	}

	return nil
}
