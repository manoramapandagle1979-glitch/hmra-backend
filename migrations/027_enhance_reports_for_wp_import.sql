-- Add new columns for WordPress JSON import
ALTER TABLE reports ADD COLUMN IF NOT EXISTS excerpt TEXT;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS industry VARCHAR(255);
ALTER TABLE reports ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]';
ALTER TABLE reports ADD COLUMN IF NOT EXISTS code VARCHAR(100);
ALTER TABLE reports ADD COLUMN IF NOT EXISTS study_period VARCHAR(50);
ALTER TABLE reports ADD COLUMN IF NOT EXISTS base_year INTEGER;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS year_start INTEGER;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS year_end INTEGER;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS cagr DECIMAL(10,4);
ALTER TABLE reports ADD COLUMN IF NOT EXISTS prices JSONB DEFAULT '{}';
ALTER TABLE reports ADD COLUMN IF NOT EXISTS segmentation TEXT;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS methodology TEXT;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_reports_tags ON reports USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_reports_industry ON reports (industry);
CREATE INDEX IF NOT EXISTS idx_reports_cagr ON reports (cagr);
CREATE INDEX IF NOT EXISTS idx_reports_year_range ON reports (year_start, year_end);
