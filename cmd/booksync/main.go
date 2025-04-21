package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"github.com/spf13/viper"
	"github.com/naimoon6450/booksync/internal/annotation"
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
	basePathRaw := viper.GetString("paths.source.base")
	srcAnnDir := viper.GetString("paths.source.annotation.dir")
	srcAnnFile := viper.GetString("paths.source.annotation.file")
	srcLibDir := viper.GetString("paths.source.library.dir")
	srcLibFile := viper.GetString("paths.source.library.file")
	targetDirRaw := viper.GetString("paths.target.dir")

	if basePathRaw == "" || srcAnnDir == "" || srcAnnFile == "" || srcLibDir == "" || srcLibFile == "" || targetDirRaw == "" {
		log.Fatal("Incomplete path configuration in config.yaml. Please check keys under 'paths.source' and 'paths.target'.")
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

	store, err := annotation.NewStore(dstAnnPath, dstLibPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	highlights, err := store.GetLatestHighlights()
	if err != nil {
		log.Fatalf("Failed to get highlights: %v", err)
	}

	if len(highlights) == 0 {
		log.Println("No highlights found")
		return
	}

	for _, highlight := range highlights {
		// Handle potential NULL values from sql.NullString
		bookTitle := "[Unknown Title]"
		if highlight.BookTitle.Valid {
			bookTitle = highlight.BookTitle.String
		}
		bookAuthor := "[Unknown Author]"
		if highlight.BookAuthor.Valid {
			bookAuthor = highlight.BookAuthor.String
		}
		
		// Use the processed standard strings (bookAuthor, bookTitle) here
		log.Printf("Highlight: [%s - %s] %s",
			bookAuthor,
			bookTitle,  
			highlight.HighlightText,
		)
	}
}
	
	
		
