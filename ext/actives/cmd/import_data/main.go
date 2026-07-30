package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/actives"
)

func main() {
	dataFile := flag.String("data", "", "exported JSON file path (required)")
	dbPath := flag.String("db", "./actives_data", "PebbleDB data path")
	flag.Parse()

	if *dataFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: import_data -data=exported.json [-db=./actives_data]\n")
		os.Exit(1)
	}

	db, err := pebble.Open(*dbPath, &pebble.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	actives.InitDB(db)

	b, err := os.ReadFile(*dataFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read data: %v\n", err)
		os.Exit(1)
	}
	if err := actives.TryImport(string(b)); err != nil {
		fmt.Fprintf(os.Stderr, "import: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Imported.")
}
