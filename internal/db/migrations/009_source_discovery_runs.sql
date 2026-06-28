CREATE TABLE source_discovery_runs (
	id INTEGER PRIMARY KEY,
	topic_source_id INTEGER NOT NULL REFERENCES topic_sources(id) ON DELETE CASCADE,
	status TEXT NOT NULL CHECK (status IN ('ready_to_process', 'needs_scope', 'discovery_failed')),
	discovered_count INTEGER NOT NULL DEFAULT 0,
	discovery_sample TEXT NOT NULL DEFAULT '[]',
	discovery_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_source_discovery_runs_source
ON source_discovery_runs(topic_source_id, created_at DESC);
