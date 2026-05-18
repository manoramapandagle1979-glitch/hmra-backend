package main

import (
	"fmt"
	"log"

	"github.com/healthcare-market-research/backend/internal/config"
	"github.com/healthcare-market-research/backend/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	cfg := config.Load()
	if err := db.Connect(cfg); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var total, withCagr, withPrices, withIndustry int64
	db.DB.Raw("SELECT COUNT(*) FROM reports WHERE deleted_at IS NULL").Scan(&total)
	db.DB.Raw("SELECT COUNT(*) FROM reports WHERE deleted_at IS NULL AND cagr IS NOT NULL").Scan(&withCagr)
	db.DB.Raw("SELECT COUNT(*) FROM reports WHERE deleted_at IS NULL AND prices IS NOT NULL AND prices != '{}'").Scan(&withPrices)
	db.DB.Raw("SELECT COUNT(*) FROM reports WHERE deleted_at IS NULL AND industry IS NOT NULL AND industry != ''").Scan(&withIndustry)
	fmt.Printf("Total reports: %d\n", total)
	fmt.Printf("With CAGR:     %d\n", withCagr)
	fmt.Printf("With Prices:   %d\n", withPrices)
	fmt.Printf("With Industry: %d\n", withIndustry)
}
