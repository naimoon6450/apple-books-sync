package annotation

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

//go:embed sql/latest_highlights.sql
var latestHighlightsQueryTemplate string

type Highlight struct {
	BookTitle      sql.NullString
	BookAuthor     sql.NullString
	HighlightText  string
}

type Store struct {
	db *sql.DB
}

func NewStore(annPath, libPath string) (*Store, error) {
	log.Printf("Opening main DB (BKLibrary): %s", libPath)

	// Read database object names from config
	// WARNING: Using these directly in Sprintf creates potential SQL injection vectors
	// if the config file is untrusted. Acceptable for this local tool, but be cautious.
	annAttachAlias := viper.GetString("db_objects.annotation_attach_alias")
	annTable := viper.GetString("db_objects.annotation_table")
	libAssetTable := viper.GetString("db_objects.library_asset_table")
	if annAttachAlias == "" || annTable == "" || libAssetTable == "" {
		return nil, fmt.Errorf("database object names (alias, tables) not fully configured in config.yaml under db_objects")
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", libPath))
	if err != nil {
		var exists int
		// Use Sprintf to insert the configured table name. Caution: Potential injection vector if config is untrusted.
		checkLibQuery := fmt.Sprintf("SELECT 1 FROM [%s] LIMIT 1", libAssetTable)
		errCheckExists := db.QueryRow(checkLibQuery).Scan(&exists)
		if errCheckExists != nil {
			if errCheckExists == sql.ErrNoRows {
				log.Printf("Warning: Table [%s] appears to be empty (no rows found). JOINs will not find book info.", libAssetTable)
			} else {
				log.Printf("Error checking for records in [%s]: %v", libAssetTable, errCheckExists)
				db.Close()
				return nil, fmt.Errorf("failed to check existence in [%s]: %w", libAssetTable, errCheckExists)
			}
		} else {
			log.Printf("Verified table [%s] is not empty.", libAssetTable)
		}
	}

	// Use Sprintf to insert the configured attach alias. Caution: Potential injection vector.
	attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS [%s]", annPath, annAttachAlias)
	log.Printf("Attaching Annotation DB: %s", attachSQL)
	if _, err = db.Exec(attachSQL); err != nil {
		return nil, fmt.Errorf("failed to attach annotation database '%s' AS [%s]: %w", annPath, annAttachAlias, err)
	} else {
		var tableName string
		// Use Sprintf to insert configured alias and table name. Caution: Potential injection.
		checkAnnQuery := fmt.Sprintf("SELECT name FROM [%s].sqlite_master WHERE type='table' AND name='%s'", annAttachAlias, annTable)
		errCheckAnn := db.QueryRow(checkAnnQuery).Scan(&tableName)
		if errCheckAnn != nil {
			log.Printf("Error checking for table [%s] in attached DB [%s] immediately after attach: %v", annTable, annAttachAlias, errCheckAnn)
		} else {
			log.Printf("Successfully verified table [%s] exists in attached [%s] database.", tableName, annAttachAlias)
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	// Use Sprintf to insert the configured attach alias. Caution: Potential injection.
	detachSQL := fmt.Sprintf("DETACH DATABASE [%s]", viper.GetString("db_objects.annotation_attach_alias"))
	_, err := s.db.Exec(detachSQL)
	if err != nil {
		fmt.Printf("Error detaching database [%s]: %v\n", viper.GetString("db_objects.annotation_attach_alias"), err)
	}
	return s.db.Close()
}

func (s *Store) GetLatestHighlights() ([]*Highlight, error) {
	// Read config for substitution
	annAttachAlias := viper.GetString("db_objects.annotation_attach_alias")
	annTable := viper.GetString("db_objects.annotation_table")
	libAssetTable := viper.GetString("db_objects.library_asset_table")
	if annAttachAlias == "" || annTable == "" || libAssetTable == "" {
		return nil, fmt.Errorf("database object names not fully configured for query")
	}

	// Substitute placeholders in the template query
	// WARNING: Using Sprintf/Replace creates potential SQL injection vectors if the config file is untrusted.
	query := latestHighlightsQueryTemplate
	query = strings.ReplaceAll(query, "[AEAnnotation]", annAttachAlias)
	query = strings.ReplaceAll(query, "[ZAEANNOTATION]", annTable)
	query = strings.ReplaceAll(query, "[ZBKLIBRARYASSET]", libAssetTable)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var highlights []*Highlight
	for rows.Next() {
		var h Highlight
		errScan := rows.Scan(
			&h.HighlightText,
			&h.BookTitle,
			&h.BookAuthor,
		)
		if errScan != nil {
			return nil, errScan
		}
		highlights = append(highlights, &h)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return highlights, nil
}


