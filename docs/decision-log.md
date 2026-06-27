# DailyDocs Decision Log

## Accepted Decisions

### Store Daily Reading Assignments

Decision: add a `daily_readings` table that records the selected page for each topic/date pair.

Reason: documentation page lists will change. Pages may be added, removed, disabled, or reordered. A stored assignment preserves what DailyDocs recommended on a given day without storing the documentation contents.

Implications:

- Historical reading results remain stable.
- New readings are generated from currently active pages.
- Past assignments are not automatically changed when page metadata changes.
- Admin repair tooling may later replace a broken current-day assignment if needed.

### Remove `Another` From MVP

Decision: exclude the `Another` feature from MVP.

Reason: the MVP promise is one carefully selected reading per topic per day. Offering alternate readings adds product and URL complexity without strengthening the core habit loop.

### Support Single-Topic Reading URLs

Decision: MVP supports one topic per reading URL using path-based routes:

```text
/{topic}
/{topic}/{date}
```

Example:

```text
/sqlite
/sqlite/2026-06-26
```

Reason: single-topic URLs make the product easier to understand and implement. The topic-only URL is the common bookmark for today's reading, while the dated URL gives DailyDocs a stable historical address. The daily assignment model is naturally keyed by one topic and one date, and multi-topic bundles can be deferred until there is evidence users need them.

### Start With Manual Curation

Decision: use manually curated seed files before building automated discovery.

Reason: content quality is the product. Manual curation gets the product to a trustworthy MVP faster than investing early in scraping heuristics.

### Build Validator Before Full Importer Automation

Decision: implement link validation before a broad automated importer.

Reason: broken recommendations are more damaging than a smaller topic catalog. Link health is part of product trust.

## Open Decisions

### Canonical Day Boundary

Question: should DailyDocs use UTC or a configured product timezone for the meaning of "today"?

Recommendation: use UTC for MVP unless the product intentionally wants a local morning-routine boundary.

### Initial Topic Set

Question: which 5-10 topics should launch first?

Recommendation: choose technologies with strong official documentation and broad developer interest, such as Go, SQLite, Docker, PostgreSQL, Git, Python, TypeScript, Kubernetes, Redis, and HTTP.

### Import Review Format

Question: should curated seed/review files use YAML, JSON, or Markdown frontmatter?

Recommendation: use YAML for human-edited topic files unless the Go implementation strongly favors another format.
