package state

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

type File struct {
	Path   string `json:"-"`
	LastPK int64  `json:"last_pk"`
}

func Load(dir string) (*File, error) {
	p := filepath.Join(dir, "booksync_state.json")
	s := &File{Path: p, LastPK: 0}

	log.Printf("Attempting to load state from: %s", p)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("State file not found at %s. Starting with default state (LastPK=0).", p)
			return s, nil
		}
		log.Printf("Error reading state file %s: %v", p, err)
		return nil, err
	}

	if err := json.Unmarshal(b, s); err != nil {
		log.Printf("Error unmarshalling state file %s: %v. Using default state.", p, err)
		s.LastPK = 0
	}

	log.Printf("Successfully loaded state: LastPK=%d", s.LastPK)
	return s, nil
}

func (f *File) Save() error {
	tmp := f.Path + ".tmp"
	log.Printf("Attempting to save state (LastPK=%d) to temporary file: %s", f.LastPK, tmp)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		log.Printf("Error marshalling state: %v", err)
		return err
	}

	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("Error writing temporary state file %s: %v", tmp, err)
		return err
	}

	log.Printf("Renaming temporary state file %s to %s", tmp, f.Path)
	if err := os.Rename(tmp, f.Path); err != nil {
		log.Printf("Error renaming state file from %s to %s: %v", tmp, f.Path, err)
		return err
	}

	log.Printf("State successfully saved to %s", f.Path)
	return nil
}
