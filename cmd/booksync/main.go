package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gosimple/slug"
	"github.com/naimoon6450/booksync/internal/annotation"
	"github.com/naimoon6450/booksync/internal/exporter"
	"github.com/naimoon6450/booksync/internal/state"
	"github.com/spf13/viper"
)

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Add other paths if needed, e.g., viper.AddConfigPath("$HOME/.config/booksync")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("Config file 'config.yaml' not found in current directory. Please create one based on config.sample.yaml.")
		} else {
			log.Fatalf("Fatal error config file: %s \n", err)
		}
	}
}

// expandPath replaces ~ with the user's home directory.
func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}

// copyFile copies a file from src to dst. Creates dst directory if needed.
func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", src, err)
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer source.Close()

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("failed to copy from %s to %s: %w", src, dst, err)
	}

	log.Printf("Copied %s to %s", src, dst)
	return nil
}

func main() {
	// --- CLI flags ----------------------------------------------------------
	vault := flag.String("vault", "", "Path to Obsidian vault (Dropbox‑synced)")
	tpl := flag.String("template", "", "Path to Go text/template file")
	flag.Parse()

	basePathRaw := viper.GetString("paths.source.base")
	log.Printf("Read config paths.source.base: %s", basePathRaw)
	srcAnnDir := viper.GetString("paths.source.annotation.dir")
	log.Printf("Read config paths.source.annotation.dir: %s", srcAnnDir)
	srcAnnFile := viper.GetString("paths.source.annotation.file")
	log.Printf("Read config paths.source.annotation.file: %s", srcAnnFile)
	srcLibDir := viper.GetString("paths.source.library.dir")
	log.Printf("Read config paths.source.library.dir: %s", srcLibDir)
	srcLibFile := viper.GetString("paths.source.library.file")
	log.Printf("Read config paths.source.library.file: %s", srcLibFile)
	targetDirRaw := viper.GetString("paths.target.dir")
	log.Printf("Read config paths.target.dir: %s", targetDirRaw)

	if basePathRaw == "" || srcAnnDir == "" || srcAnnFile == "" || srcLibDir == "" || srcLibFile == "" || targetDirRaw == "" {
		log.Fatal("Incomplete path configuration in config.yaml. Please check keys under 'paths.source' and 'paths.target'.")
	}

	if *vault == "" || *tpl == "" {
		log.Fatal("vault and template flags are required")
	}

	srcBasePath, err := expandPath(basePathRaw)
	if err != nil {
		log.Fatalf("Failed to expand source base path '%s': %v", basePathRaw, err)
	}
	targetDir, err := expandPath(targetDirRaw) // Also expand target in case ~ is used
	if err != nil {
		log.Fatalf("Failed to expand target directory '%s': %v", targetDirRaw, err)
	}

	// Construct source and destination paths
	srcAnnPath := filepath.Join(srcBasePath, srcAnnDir, srcAnnFile)
	srcLibPath := filepath.Join(srcBasePath, srcLibDir, srcLibFile)

	dstAnnPath := filepath.Join(targetDir, srcAnnFile)
	dstLibPath := filepath.Join(targetDir, srcLibFile)

	// Check if destination files exist
	_, errAnn := os.Stat(dstAnnPath)
	_, errLib := os.Stat(dstLibPath)

	// Copy files only if they don't exist in the target directory
	if os.IsNotExist(errAnn) || os.IsNotExist(errLib) {
		log.Printf("Destination files not found in %s. Copying from source...", targetDir)
		if err := copyFile(srcAnnPath, dstAnnPath); err != nil {
			log.Fatalf("Failed to copy annotation database: %v", err)
		}
		if err := copyFile(srcLibPath, dstLibPath); err != nil {
			log.Fatalf("Failed to copy library database: %v", err)
		}
		log.Println("Database files copied successfully.")
	} else {
		log.Printf("Using existing database files in %s", targetDir)
	}

	// Use the DESTINATION paths for the store
	log.Printf("Using Annotation Path: %s", dstAnnPath)
	log.Printf("Using Library Path: %s", dstLibPath)

	// --- database handles ---------------------------------------------------
	store, err := annotation.NewStore(dstAnnPath, dstLibPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// --- state file ---------------------------------------------------------
	vaultPath, err := expandPath(*vault)
	if err != nil {
		log.Fatalf("Failed to expand vault path '%s': %v", *vault, err)
	}

	st, err := state.Load(vaultPath)
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	// --- exporter -----------------------------------------------------------
	exp, err := exporter.New(vaultPath, *tpl)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	// --- pull + write -------------------------------------------------------
	highlights, err := store.GetLatestHighlights()
	if err != nil {
		log.Fatalf("Failed to get latest highlights: %v", err)
	}

	if len(highlights) == 0 {
		log.Println("No new highlights found")
		return
	}

	// Group highlights by book
	bookData := make(map[string]*exporter.BookData)

	// Collect all highlights by book
	for _, h := range highlights {
		bookKey := slug.Make(h.BookTitle)

		if _, exists := bookData[bookKey]; !exists {
			bookData[bookKey] = &exporter.BookData{
				Title:      h.BookTitle,
				Author:     h.BookAuthor,
				Highlights: []string{},
			}
		}

		bookData[bookKey].Highlights = append(bookData[bookKey].Highlights, h.HighlightText)

		// Update the last primary key
		st.LastPK++
	}

	// Export each book's highlights
	for _, data := range bookData {
		if err := exp.WriteBook(*data); err != nil {
			log.Printf("Export failed for book %s: %v", data.Title, err)
			continue
		}
	}

	if err := st.Save(); err != nil {
		log.Fatalf("Failed to save state: %v", err)
	}

	log.Printf("Exported %d new highlight(s) across %d books", len(highlights), len(bookData))
}
