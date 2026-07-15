// Package config resolves where save-game files live on disk.
package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const saveExt = ".db"

// SaveDir returns the directory save-game SQLite files are stored in,
// creating it if necessary. Honors $XDG_DATA_HOME.
func SaveDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "rpg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// SavePath returns the SQLite file path for the given save name.
func SavePath(dir, name string) string {
	return filepath.Join(dir, name+saveExt)
}

// ListSaves returns the names of existing saves in dir, most-recently
// modified first.
func ListSaves(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type save struct {
		name    string
		modTime time.Time
	}
	var saves []save
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), saveExt) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		saves = append(saves, save{name: strings.TrimSuffix(e.Name(), saveExt), modTime: info.ModTime()})
	}
	sort.Slice(saves, func(i, j int) bool { return saves[i].modTime.After(saves[j].modTime) })
	names := make([]string, len(saves))
	for i, s := range saves {
		names[i] = s.name
	}
	return names, nil
}
