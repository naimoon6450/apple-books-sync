package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gosimple/slug"
)

// Highlight struct is no longer needed directly by the exporter logic
// keeping it here might be useful if other parts of the codebase use it
// If not, it can be removed later.
// type Highlight struct {
// 	ID         int64
// 	BookTitle  string
// 	BookAuthor string
// 	Text       string
// }

// BookData represents all highlights for a book
type BookData struct {
	Title      string
	Author     string
	Highlights []string
}

type Exporter struct {
	vaultDir string
	tpl      *template.Template
}

func New(vault, tplPath string) (*Exporter, error) {
	tplBytes, err := os.ReadFile(tplPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", tplPath, err)
	}
	t, err := template.New("note").Parse(string(tplBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return &Exporter{
		vaultDir: vault,
		tpl:      t,
	}, nil
}

// WriteBook creates a directory for the book and writes its highlights to a highlights.md file.
func (e *Exporter) WriteBook(bookData BookData) error {
	bookKey := slug.Make(bookData.Title)

	// Create apple_books_sync base folder if it doesn't exist
	appleBooksBaseDir := filepath.Join(e.vaultDir, "apple_books_sync")
	if err := os.MkdirAll(appleBooksBaseDir, 0o755); err != nil {
		return fmt.Errorf("failed to create apple_books_sync base directory %s: %w", appleBooksBaseDir, err)
	}

	// Path to the book's markdown file within the base directory
	bookFile := filepath.Join(appleBooksBaseDir, bookKey+".md")

	// Open the file in create/truncate mode
	// This will overwrite the file completely each time
	f, err := os.OpenFile(bookFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open book file %s: %w", bookFile, err)
	}
	defer f.Close()

	// Execute the template with the book data
	err = e.tpl.Execute(f, bookData)
	if err != nil {
		return fmt.Errorf("failed to execute template for book %s: %w", bookData.Title, err)
	}

	return nil
}
