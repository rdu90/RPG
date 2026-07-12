// Command dbquery runs a single read-only SQL statement against a save file
// and prints the results as a tab-separated table. It exists so inspecting a
// save doesn't require the system sqlite3 CLI to be installed — the project
// already depends on modernc.org/sqlite (pure Go, no CGO) specifically to
// avoid environment dependencies like that, and make db-shell inherited one
// anyway. Run via `make db-query SAVE=<name> SQL="<statement>"`.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: dbquery <db-path> <sql>")
		os.Exit(1)
	}
	path, query := os.Args[1], os.Args[2]

	db, err := sql.Open("sqlite", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		fmt.Fprintf(os.Stderr, "columns: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(cols, "\t"))

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			os.Exit(1)
		}
		cells := make([]string, len(vals))
		for i, v := range vals {
			cells[i] = fmt.Sprintf("%v", v)
		}
		fmt.Println(strings.Join(cells, "\t"))
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "rows: %v\n", err)
		os.Exit(1)
	}
}
