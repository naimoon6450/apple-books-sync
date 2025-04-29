package watcher

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gosimple/slug"
	"github.com/naimoon6450/booksync/internal/annotation"
	"github.com/naimoon6450/booksync/internal/exporter"
	"github.com/naimoon6450/booksync/internal/state"
)

// WatchAndSync monitors the annotation database file for changes and triggers
// synchronization of new highlights.
func WatchAndSync(
	ctx context.Context,
	store *annotation.Store,
	exp *exporter.Exporter,
	st *state.File,
) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer w.Close()

	if err := w.Add(filepath.Dir(st.Path)); err != nil {
		return fmt.Errorf("failed to add path %s to watcher: %w", st.Path, err)
	}

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	var debounceTimer *time.Timer

	sync := func() {
		log.Println("Sync triggered")
		highlights, err := store.GetHighlightsSince(st.LastPK)
		if err != nil {
			log.Printf("ERROR: Failed to get highlights since PK %d: %v", st.LastPK, err)
			return
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
				log.Printf("ERROR: Failed to write book '%s': %v", data.Title, err)
				exportErrors++
			}
		}

		if exportErrors > 0 {
			log.Printf("Sync completed with %d errors.", exportErrors)
		} else {
			log.Printf("Sync completed successfully for %d book(s).", len(bookDataMap))
		}

		if maxPK > st.LastPK {
			log.Printf("Updating last PK from %d to %d", st.LastPK, maxPK)
			st.LastPK = maxPK
			if err := st.Save(); err != nil {
				log.Printf("ERROR: Failed to save state after update: %v", err)
			} else {
				log.Printf("State saved successfully with last PK %d", st.LastPK)
			}
		} else {
			log.Printf("No update to last PK needed (still %d).", st.LastPK)
		}
	}

	sync()

	for {
		select {
		case ev := <-w.Events:
			if ev.Name == st.Path && ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				log.Printf("File event detected: %s on %s", ev.Op, ev.Name)
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(2*time.Second, sync)
			}
		case err := <-w.Errors:
			log.Printf("Watcher error: %v", err)
		case <-ticker.C:
			log.Println("Periodic sync triggered by ticker")
			sync()
		case <-ctx.Done():
			log.Println("Watcher context cancelled, shutting down.")
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return ctx.Err()
		}
	}
}
