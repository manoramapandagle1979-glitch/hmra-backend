package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	"github.com/healthcare-market-research/backend/internal/config"
	"github.com/healthcare-market-research/backend/internal/db"
	"github.com/healthcare-market-research/backend/internal/domain/category"
	"github.com/healthcare-market-research/backend/internal/domain/report"
)

// WP JSON structure
type wpReport struct {
	ID             int      `json:"id"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	Excerpt        string   `json:"excerpt"`
	Industry       string   `json:"industry"`
	Categories     []string `json:"categories"`
	Tags           []string `json:"tags"`
	Regions        []string `json:"regions"`
	PublishedDate  string   `json:"publishedDate"`
	Pages          string   `json:"pages"`
	CAGR           *float64 `json:"cagr"`
	BaseYear       *int     `json:"baseYear"`
	YearStart      *int     `json:"yearStart"`
	YearEnd        *int     `json:"yearEnd"`
	StudyPeriod    string   `json:"studyPeriod"`
	Code           *string  `json:"code"`
	Prices         wpPrices `json:"prices"`
	Modified       string   `json:"modified"`
	Description    string   `json:"description"`
	Segmentation   string   `json:"segmentation"`
	TableOfContents string  `json:"tableOfContents"`
	Methodology    string   `json:"methodology"`
	KeyPlayers     string   `json:"keyPlayers"`
	FAQs           []wpFAQ  `json:"faqs"`
	SEO            wpSEO    `json:"seo"`
}

type wpPrices struct {
	Single     float64 `json:"single"`
	Team       float64 `json:"team"`
	Enterprise float64 `json:"enterprise"`
	DataPack   float64 `json:"dataPack"`
}

type wpFAQ struct {
	Q string `json:"q"`
	A string `json:"a"`
}

type wpSEO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func slugify(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	s := b.String()
	// collapse multiple dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func parsePageCount(s string) int {
	// "220+" → 220, "220" → 220, "" → 0
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// strip non-numeric suffix
	numStr := strings.TrimRight(s, "+abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ,")
	n, _ := strconv.Atoi(numStr)
	return n
}

func getOrCreateCategory(name string) (uint, error) {
	if name == "" {
		name = "Uncategorized"
	}
	slug := slugify(name)

	var cat category.Category
	err := db.DB.Where("slug = ?", slug).First(&cat).Error
	if err == nil {
		return cat.ID, nil
	}

	// Create new
	cat = category.Category{
		Name:     name,
		Slug:     slug,
		IsActive: true,
	}
	if err := db.DB.Create(&cat).Error; err != nil {
		return 0, fmt.Errorf("create category %q: %w", name, err)
	}
	return cat.ID, nil
}

func mapWPToReport(wp *wpReport, catID uint) report.Report {
	// Geography
	geo := make(report.StringSlice, 0, len(wp.Regions))
	for _, r := range wp.Regions {
		if r != "" {
			geo = append(geo, r)
		}
	}
	if len(geo) == 0 {
		geo = report.StringSlice{"Global"}
	}

	// Tags
	tags := make(report.StringSlice, 0, len(wp.Tags))
	tags = append(tags, wp.Tags...)

	// Summary: first 500 chars of stripped description
	summary := truncate(stripHTML(wp.Description), 500)
	if summary == "" {
		summary = truncate(stripHTML(wp.Excerpt), 500)
	}
	if summary == "" {
		summary = wp.Title
	}

	// Publish date
	var publishDate *time.Time
	if wp.PublishedDate != "" {
		if t, err := time.Parse("2006-01-02", wp.PublishedDate); err == nil {
			publishDate = &t
		}
	}

	// FAQs
	faqs := make(report.FAQs, 0, len(wp.FAQs))
	for _, f := range wp.FAQs {
		faqs = append(faqs, report.FAQ{
			Question: f.Q,
			Answer:   f.A,
		})
	}

	// Sections
	sections := report.ReportSections{
		KeyPlayers:      wp.KeyPlayers,
		TableOfContents: wp.TableOfContents,
		MarketDetails:   "",
	}

	// Prices
	prices := report.Prices{
		Single:     wp.Prices.Single,
		Team:       wp.Prices.Team,
		Enterprise: wp.Prices.Enterprise,
		DataPack:   wp.Prices.DataPack,
	}

	// MarketMetrics — computed from WP fields
	cagrStr := ""
	if wp.CAGR != nil {
		// Format like "57.54%" — round to 2 decimal places
		cagrStr = fmt.Sprintf("%.2f%%", *wp.CAGR)
	}
	var cagrStartYear, cagrEndYear, currentYear, forecastYear int
	if wp.YearStart != nil {
		cagrStartYear = *wp.YearStart
	}
	if wp.YearEnd != nil {
		cagrEndYear = *wp.YearEnd
		forecastYear = *wp.YearEnd
	}
	if wp.BaseYear != nil {
		currentYear = *wp.BaseYear
	}
	mm := &report.MarketMetrics{
		CurrentYear:   currentYear,
		ForecastYear:  forecastYear,
		CAGR:          cagrStr,
		CAGRStartYear: cagrStartYear,
		CAGREndYear:   cagrEndYear,
	}
	// Only set if we have meaningful values
	if currentYear == 0 && forecastYear == 0 && cagrStr == "" {
		mm = nil
	}

	// KeyPlayers — empty structured array
	keyPlayers := report.KeyPlayers{}

	// Formats — default PDF
	formats := report.StringSlice{"PDF", "Excel"}

	rep := report.Report{
		CategoryID:   catID,
		Slug:         wp.Slug,
		Title:        wp.Title,
		Excerpt:      wp.Excerpt,
		Description:  wp.Description,
		Summary:      summary,
		Price:        wp.Prices.Single,
		Prices:       prices,
		PageCount:    parsePageCount(wp.Pages),
		Formats:      formats,
		Geography:    geo,
		Status:       "published",
		IsFeatured:   false,
		PublishDate:  publishDate,
		MarketMetrics: mm,
		KeyPlayers:   keyPlayers,
		Sections:     sections,
		FAQs:         faqs,
		MetaTitle:    wp.SEO.Title,
		MetaDescription: wp.SEO.Description,
		InternalLinks: report.InternalLinks{},
		// New WP fields
		Industry:     wp.Industry,
		Tags:         tags,
		Code:         wp.Code,
		StudyPeriod:  wp.StudyPeriod,
		BaseYear:     wp.BaseYear,
		YearStart:    wp.YearStart,
		YearEnd:      wp.YearEnd,
		CAGR:         wp.CAGR,
		Segmentation: wp.Segmentation,
		Methodology:  wp.Methodology,
	}

	return rep
}

func main() {
	dir := flag.String("dir", "", "Path to directory containing WP JSON files (required)")
	clear := flag.Bool("clear", false, "Delete all reports before importing")
	dryRun := flag.Bool("dry-run", false, "Parse and validate without writing to DB")
	envFile := flag.String("env-file", ".env", "Path to .env file")
	flag.Parse()

	if *dir == "" {
		log.Fatal("--dir is required")
	}

	// Load env
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

	// Walk JSON files
	entries, err := os.ReadDir(*dir)
	if err != nil {
		log.Fatalf("Cannot read dir %s: %v", *dir, err)
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(*dir, e.Name()))
		}
	}

	fmt.Printf("Found %d JSON files in %s\n", len(jsonFiles), *dir)

	// Apply schema migration for new WP columns
	if !*dryRun {
		fmt.Println("Applying schema migration 027...")
		migrations := []string{
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS excerpt TEXT",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS industry VARCHAR(255)",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS code VARCHAR(100)",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS study_period VARCHAR(50)",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS base_year INTEGER",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS year_start INTEGER",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS year_end INTEGER",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS cagr DECIMAL(10,4)",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS prices JSONB DEFAULT '{}'",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS segmentation TEXT",
			"ALTER TABLE reports ADD COLUMN IF NOT EXISTS methodology TEXT",
			"CREATE INDEX IF NOT EXISTS idx_reports_tags ON reports USING GIN (tags)",
			"CREATE INDEX IF NOT EXISTS idx_reports_industry ON reports (industry)",
			"CREATE INDEX IF NOT EXISTS idx_reports_cagr ON reports (cagr)",
			"CREATE INDEX IF NOT EXISTS idx_reports_year_range ON reports (year_start, year_end)",
		}
		for _, m := range migrations {
			if err := db.DB.Exec(m).Error; err != nil {
				log.Fatalf("Migration failed (%s): %v", m, err)
			}
		}
		fmt.Println("Schema migration complete.")
	}

	if !*dryRun && *clear {
		fmt.Println("--clear: deleting all reports and related data...")
		// Delete in dependency order to satisfy foreign keys
		for _, stmt := range []string{
			"DELETE FROM report_versions",
			"DELETE FROM report_images",
			"DELETE FROM chart_metadata",
			"DELETE FROM reports",
		} {
			if err := db.DB.Exec(stmt).Error; err != nil {
				log.Fatalf("Failed during clear (%s): %v", stmt, err)
			}
		}
		fmt.Println("Reports cleared.")
	}

	var created, updated, skipped int

	for _, path := range jsonFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[SKIP] %s: read error: %v", filepath.Base(path), err)
			skipped++
			continue
		}

		var wp wpReport
		if err := json.Unmarshal(data, &wp); err != nil {
			log.Printf("[SKIP] %s: parse error: %v", filepath.Base(path), err)
			skipped++
			continue
		}

		if wp.Slug == "" {
			log.Printf("[SKIP] %s: empty slug", filepath.Base(path))
			skipped++
			continue
		}

		if *dryRun {
			fmt.Printf("[DRY-RUN] %s\n", wp.Slug)
			continue
		}

		// Get or create category
		catName := ""
		if len(wp.Categories) > 0 {
			catName = wp.Categories[0]
		}
		catID, err := getOrCreateCategory(catName)
		if err != nil {
			log.Printf("[SKIP] %s: category error: %v", wp.Slug, err)
			skipped++
			continue
		}

		rep := mapWPToReport(&wp, catID)

		// Check if exists
		var existing report.Report
		existsErr := db.DB.Unscoped().Where("slug = ?", rep.Slug).First(&existing).Error

		if existsErr != nil {
			// Create new
			if err := db.DB.Create(&rep).Error; err != nil {
				log.Printf("[SKIP] %s: create error: %v", rep.Slug, err)
				skipped++
				continue
			}
			fmt.Printf("[CREATED] %s\n", rep.Slug)
			created++
		} else {
			// Update existing — preserve ID, created_by, updated_by
			rep.ID = existing.ID
			rep.CreatedBy = existing.CreatedBy
			rep.DeletedAt = existing.DeletedAt

			// Use Save to update all fields
			if err := db.DB.Save(&rep).Error; err != nil {
				log.Printf("[SKIP] %s: update error: %v", rep.Slug, err)
				skipped++
				continue
			}
			fmt.Printf("[UPDATED] %s\n", rep.Slug)
			updated++
		}
	}

	total := created + updated + skipped
	fmt.Printf("\n=== Seeder Summary ===\n")
	fmt.Printf("Total files : %d\n", total)
	fmt.Printf("Created     : %d\n", created)
	fmt.Printf("Updated     : %d\n", updated)
	fmt.Printf("Skipped     : %d\n", skipped)

	if skipped > 0 {
		os.Exit(1)
	}
}

