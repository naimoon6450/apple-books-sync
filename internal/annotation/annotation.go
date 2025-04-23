package annotation

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

//go:embed sql/*.sql
var sqlFS embed.FS

type Highlight struct {
	HighlightText string
	BookTitle     string
	BookAuthor    string
}

type Store struct {
	db *sql.DB
}

func NewStore(annPath, libPath string) (*Store, error) {
	log.Printf("Opening main DB (BKLibrary): %s", libPath)

	// Only the attach alias is configurable, table names are hardcoded
	// as they should be consistent across all macs
	annAttachAlias := viper.GetString("db_objects.annotation_attach_alias")
	if annAttachAlias == "" {
		return nil, fmt.Errorf("annotation_attach_alias not configured in config.yaml under db_objects")
	}

	// Hardcoded table names - these should be consistent across all macs
	const annTable = "ZAEANNOTATION"
	const libAssetTable = "ZBKLIBRARYASSET"

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", libPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open library database: %w", err)
	}

	// Check if the library table exists and has data
	var exists int
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

	// Attach the annotation database
	attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS [%s]", annPath, annAttachAlias)
	log.Printf("Attaching Annotation DB: %s", attachSQL)
	if _, err = db.Exec(attachSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to attach annotation database '%s' AS [%s]: %w", annPath, annAttachAlias, err)
	}

	// Verify the annotation table exists
	var tableName string
	checkAnnQuery := fmt.Sprintf("SELECT name FROM [%s].sqlite_master WHERE type='table' AND name='%s'", annAttachAlias, annTable)
	errCheckAnn := db.QueryRow(checkAnnQuery).Scan(&tableName)
	if errCheckAnn != nil {
		log.Printf("Error checking for table [%s] in attached DB [%s] immediately after attach: %v", annTable, annAttachAlias, errCheckAnn)
	} else {
		log.Printf("Successfully verified table [%s] exists in attached [%s] database.", tableName, annAttachAlias)
	}

	store := &Store{
		db: db,
	}

	return store, nil
}

func (s *Store) Close() error {
	// Detach database
	detachSQL := fmt.Sprintf("DETACH DATABASE [%s]", viper.GetString("db_objects.annotation_attach_alias"))
	if _, err := s.db.Exec(detachSQL); err != nil {
		return fmt.Errorf("error detaching database [%s]: %w",
			viper.GetString("db_objects.annotation_attach_alias"), err)
	}

	// Close database connection
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	return nil
}

// removeComments removes SQL comments from the query
func removeComments(sql string) string {
	// Remove line comments (-- ...)
	lineCommentRegex := regexp.MustCompile(`--.*?(\n|$)`)
	withoutLineComments := lineCommentRegex.ReplaceAllString(sql, "\n")

	// Remove multi-line comments (/* ... */)
	multilineCommentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	withoutComments := multilineCommentRegex.ReplaceAllString(withoutLineComments, "")

	return withoutComments
}

func (s *Store) GetLatestHighlights() ([]*Highlight, error) {
	// Get the query from embedded files
	sqlBytes, err := sqlFS.ReadFile("sql/latest_highlights.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded SQL file: %w", err)
	}

	// Only the attach alias is configurable, table names are hardcoded
	annAttachAlias := viper.GetString("db_objects.annotation_attach_alias")

	// Hardcoded table names - these should be consistent across all macs
	const annTable = "ZAEANNOTATION"
	const libAssetTable = "ZBKLIBRARYASSET"

	// Replace the placeholders in the SQL query
	sqlTemplate := string(sqlBytes)

	// Remove comments before processing
	sqlTemplate = removeComments(sqlTemplate)

	// Replace placeholders
	querySQL := strings.ReplaceAll(sqlTemplate, "[AEAnnotation]", fmt.Sprintf("[%s]", annAttachAlias))
	querySQL = strings.ReplaceAll(querySQL, "[ZAEANNOTATION]", fmt.Sprintf("[%s]", annTable))
	querySQL = strings.ReplaceAll(querySQL, "[ZBKLIBRARYASSET]", fmt.Sprintf("[%s]", libAssetTable))

	// Trim whitespace and make sure it's properly formatted
	querySQL = strings.TrimSpace(querySQL)

	// For debugging
	// log.Printf("Executing query: %s", querySQL)

	rows, err := s.db.Query(querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to execute latest highlights query: %w", err)
	}
	defer rows.Close()

	var highlights []*Highlight
	for rows.Next() {
		var h Highlight
		var highlight, bookTitle, bookAuthor string

		errScan := rows.Scan(
			&highlight,
			&bookTitle,
			&bookAuthor,
		)
		if errScan != nil {
			return nil, fmt.Errorf("failed to scan row: %w", errScan)
		}

		h.HighlightText = highlight
		h.BookTitle = bookTitle
		h.BookAuthor = bookAuthor

		highlights = append(highlights, &h)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return highlights, nil
}
