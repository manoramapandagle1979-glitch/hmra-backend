package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/healthcare-market-research/backend/internal/config"
	"github.com/healthcare-market-research/backend/internal/db"
)

type summaryEntry struct {
	Slug    string `json:"slug"`
	Summary string `json:"summary"`
}

func main() {
	jsonFile := flag.String("json", "", "Path to reports.summaries.json (required)")
	dryRun := flag.Bool("dry-run", false, "Print what would be updated without writing to DB")
	envFile := flag.String("env-file", ".env", "Path to .env file")
	flag.Parse()

	if *jsonFile == "" {
		log.Fatal("--json is required")
	}

	if err := godotenv.Load(*envFile); err != nil {
		log.Printf("Warning: could not load %s: %v", *envFile, err)
	}

	if !*dryRun {
		cfg := config.Load()
		if err := db.Connect(cfg); err != nil {
			log.Fatalf("DB connect failed: %v", err)
		}
		defer db.Close()
	}

	data, err := os.ReadFile(*jsonFile)
	if err != nil {
		log.Fatalf("Cannot read %s: %v", *jsonFile, err)
	}

	var entries []summaryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Fatalf("Cannot parse JSON: %v", err)
	}

	fmt.Printf("Loaded %d summary entries\n", len(entries))

	var updated, skipped, notFound int

	for _, e := range entries {
		if e.Slug == "" || e.Summary == "" {
			log.Printf("[SKIP] empty slug or summary")
			skipped++
			continue
		}

		if *dryRun {
			fmt.Printf("[DRY-RUN] %s\n", e.Slug)
			continue
		}

		result := db.DB.Exec(
			"UPDATE reports SET summary = ?, updated_at = NOW() WHERE slug = ?",
			e.Summary, e.Slug,
		)
		if result.Error != nil {
			log.Printf("[ERROR] %s: %v", e.Slug, result.Error)
			skipped++
			continue
		}
		if result.RowsAffected == 0 {
			log.Printf("[NOT FOUND] %s", e.Slug)
			notFound++
			continue
		}

		fmt.Printf("[UPDATED] %s\n", e.Slug)
		updated++
	}

	fmt.Printf("\n=== Update Summaries ===\n")
	fmt.Printf("Total entries : %d\n", len(entries))
	fmt.Printf("Updated       : %d\n", updated)
	fmt.Printf("Not found     : %d\n", notFound)
	fmt.Printf("Skipped       : %d\n", skipped)
}
