ALTER TABLE topic_sources ADD COLUMN last_discovered_at TEXT;
ALTER TABLE topic_sources ADD COLUMN discovery_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE topic_sources ADD COLUMN discovery_sample TEXT NOT NULL DEFAULT '[]';
ALTER TABLE topic_sources ADD COLUMN discovery_error TEXT NOT NULL DEFAULT '';
