package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gosimple/slug"
	"github.com/naimoon6450/booksync/internal/annotation"
	"github.com/naimoon6450/booksync/internal/exporter"
	"github.com/naimoon6450/booksync/internal/state"
	"github.com/naimoon6450/booksync/internal/watcher"
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
	vault := flag.String("vault", "", "Path to Obsidian vault")
	tpl := flag.String("template", "", "Path to Go text/template file")
	watch := flag.Bool("watch", false, "Enable watch mode to continuously sync changes")
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
	targetDir, err := expandPath(targetDirRaw)
	if err != nil {
		log.Fatalf("Failed to expand target directory '%s': %v", targetDirRaw, err)
	}

	// Construct source and destination paths
	srcAnnPattern := filepath.Join(srcBasePath, srcAnnDir, srcAnnFile)
	srcLibPattern := filepath.Join(srcBasePath, srcLibDir, srcLibFile)

	srcAnnMatches, err := filepath.Glob(srcAnnPattern)
	if err != nil || len(srcAnnMatches) == 0 {
		log.Fatalf("Error finding source annotation file matching pattern '%s' (or no matches found): %v", srcAnnPattern, err)
	}
	srcAnnPath := srcAnnMatches[0]
	actualAnnFilename := filepath.Base(srcAnnPath)

	srcLibMatches, err := filepath.Glob(srcLibPattern)
	if err != nil || len(srcLibMatches) == 0 {
		log.Fatalf("Error finding source library file matching pattern '%s' (or no matches found): %v", srcLibPattern, err)
	}
	srcLibPath := srcLibMatches[0]
	actualLibFilename := filepath.Base(srcLibPath)

	dstAnnPath := filepath.Join(targetDir, actualAnnFilename)
	dstLibPath := filepath.Join(targetDir, actualLibFilename)

	log.Printf("Resolved Source Annotation DB: %s", srcAnnPath)
	log.Printf("Resolved Source Library DB: %s", srcLibPath)
	log.Printf("Using Target Annotation DB: %s", dstAnnPath)
	log.Printf("Using Target Library DB: %s", dstLibPath)

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

	// --- Common Initialization (Store, State, Exporter) ---------------------
	log.Printf("Initializing common components...")
	store, err := annotation.NewStore(dstAnnPath, dstLibPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	vaultPath, err := expandPath(*vault)
	if err != nil {
		log.Fatalf("Failed to expand vault path '%s': %v", *vault, err)
	}

	st, err := state.Load(vaultPath)
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	exp, err := exporter.New(vaultPath, *tpl)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	// --- Mode Selection (Watch or One-off Sync) -----------------------------
	if *watch {
		// --- Watch Mode --- //
		log.Println("Watch mode enabled. Starting watcher...")
		log.Printf("Watching file: %s", dstAnnPath)
		log.Println("Press Ctrl+C to stop.")

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		st.Path = dstAnnPath

		err = watcher.WatchAndSync(ctx, store, exp, st)
		if err != nil && err != context.Canceled {
			log.Fatalf("Watcher failed: %v", err)
		}
		log.Println("Watcher stopped.")

	} else {
		// --- One-off Sync Mode --- //
		log.Println("Performing one-off sync...")
		highlights, err := store.GetHighlightsSince(st.LastPK)
		if err != nil {
			log.Fatalf("Failed to get highlights since PK %d: %v", st.LastPK, err)
		}

		if len(highlights) == 0 {
			log.Printf("No new highlights found since PK %d.", st.LastPK)
			return
		}

		bookDataMap := make(map[string]*exporter.BookData)
		var highlightsCount int
		var maxPK int64 = st.LastPK

		for _, h := range highlights {
			bookKey := slug.Make(h.BookTitle)

			if _, exists := bookDataMap[bookKey]; !exists {
				bookDataMap[bookKey] = &exporter.BookData{
					Title:      h.BookTitle,
					Author:     h.BookAuthor,
					Highlights: []string{},
				}
			}
			bookDataMap[bookKey].Highlights = append(bookDataMap[bookKey].Highlights, h.HighlightText)
			highlightsCount++

			if h.PK > maxPK {
				maxPK = h.PK
			}
		}

		log.Printf("Processing %d new highlight(s) across %d book(s) (max PK: %d)...", highlightsCount, len(bookDataMap), maxPK)

		var exportErrors int
		for _, data := range bookDataMap {
			if err := exp.WriteBook(*data); err != nil {
				log.Printf("ERROR: Export failed for book '%s': %v", data.Title, err)
				exportErrors++
			}
		}

		if maxPK > st.LastPK {
			log.Printf("Updating last PK from %d to %d", st.LastPK, maxPK)
			st.LastPK = maxPK
			if err := st.Save(); err != nil {
				log.Fatalf("FATAL: Failed to save state after update: %v", err)
			} else {
				log.Printf("State saved successfully with last PK %d", st.LastPK)
			}
		} else {
			log.Printf("No update to last PK needed (still %d).", st.LastPK)
		}

		if exportErrors > 0 {
			log.Printf("One-off sync completed with %d errors.", exportErrors)
		} else {
			log.Printf("One-off sync completed successfully for %d book(s).", len(bookDataMap))
		}
	}
}
