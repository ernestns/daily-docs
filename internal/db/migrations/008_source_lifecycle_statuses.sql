PRAGMA writable_schema = ON;

UPDATE sqlite_schema
SET sql = replace(
	sql,
	"status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'needs_scope'))",
	"status TEXT NOT NULL DEFAULT 'pending_discovery' CHECK (status IN ('pending_discovery', 'ready_to_process', 'processing', 'candidates_ready', 'needs_scope', 'discovery_failed', 'disabled'))"
)
WHERE type = 'table'
	AND name = 'topic_sources';

PRAGMA writable_schema = OFF;

UPDATE topic_sources
SET status = CASE
	WHEN status = 'active' AND discovery_count > 0 THEN 'ready_to_process'
	WHEN status = 'active' THEN 'pending_discovery'
	ELSE status
END;
