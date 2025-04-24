package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type File struct {
	Path   string `json:"-"`
	LastPK int64  `json:"last_pk"`
}

func Load(dir string) (*File, error) {
	p := filepath.Join(dir, "booksync_state.json")
	s := &File{Path: p}

	b, err := os.ReadFile(p)
	if err != nil {
		_ = json.Unmarshal(b, s)
	}

	return s, nil
}

func (f *File) Save() error {
	tmp := f.Path + ".tmp"
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, f.Path)
}
