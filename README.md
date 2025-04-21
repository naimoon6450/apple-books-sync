# BookSync (Working Title)

A simple tool to access and potentially export highlights from Apple Books (iBooks) databases on macOS.

## Prerequisites

*   Go (version 1.21 or later recommended)
*   Access to your iBooks database files on macOS.

## Setup

1.  **Clone the repository:**
    ```bash
    git clone <your-repository-url>
    cd booksync
    ```

2.  **Configure Paths:**
    *   Rename `config.sample.yaml` to `config.yaml`.
        ```bash
        cp config.sample.yaml config.yaml
        ```
    *   Edit `config.yaml` with a text editor.
    *   Verify the paths under `paths.source` point correctly to your iBooks `AEAnnotation` and `BKLibrary` database files. Use `~` to represent your home directory. The defaults should work for standard macOS setups.
    *   Verify or change `paths.target.dir`. This is where the tool will copy the databases before reading them (default is a `./data` subdirectory within the project).

3.  **Install Dependencies (Optional but good practice):**
    ```bash
    go mod tidy
    ```

## Running

*   **Run directly:**
    ```bash
    go run ./cmd/booksync/main.go
    ```
    This will:
    1.  Read `config.yaml`.
    2.  Copy the source databases to the target directory (if they don't exist there).
    3.  Connect to the copied databases.
    4.  Fetch the 10 most recent highlights.
    5.  Print the highlights to the console.

*   **Build and Run:**
    ```bash
    # Build the executable (output to bin/booksync by default)
    go build -o bin/booksync ./cmd/booksync/main.go

    # Run the built executable
    ./bin/booksync
    ```

## Configuration (`config.yaml`)

*   `paths.source.base`: The base directory containing the iBooks container data.
*   `paths.source.annotation.dir`/`file`: Subdirectory and filename for the annotations database.
*   `paths.source.library.dir`/`file`: Subdirectory and filename for the library metadata database.
*   `paths.target.dir`: Directory where database copies are stored for processing.

## Notes

*   The application currently only reads the latest 10 highlights and prints them.
*   The database filenames within iBooks might change with future macOS/iBooks updates, requiring adjustments to `config.yaml`. 