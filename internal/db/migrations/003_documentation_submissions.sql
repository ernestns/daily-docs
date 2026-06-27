CREATE TABLE documentation_submissions (
	id INTEGER PRIMARY KEY,
	submitted_url TEXT NOT NULL,
	normalized_url TEXT NOT NULL UNIQUE,
	source_host TEXT NOT NULL,
	suggested_topic TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'candidates_ready', 'active', 'rejected', 'failed')),
	visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'hidden')),
	request_count INTEGER NOT NULL DEFAULT 1,
	submitter_ip_hash TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	first_submitted_at TEXT NOT NULL DEFAULT (datetime('now')),
	last_submitted_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_documentation_submissions_status ON documentation_submissions(status, last_submitted_at);
CREATE INDEX idx_documentation_submissions_public ON documentation_submissions(visibility, last_submitted_at);
