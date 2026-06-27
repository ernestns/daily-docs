# DailyDocs Implementation Strategy

## Approach

Implement DailyDocs in thin vertical slices.

The core product risk is not rendering a reading page. The core risk is incorrect, stale, or broken links.

## Implementation Order

0. Public hello-world Go app
1. SQLite schema and migrations
2. Topic/page seed importer
3. Daily reading assignment logic with tests
4. Basic reading page rendering
5. Topic search and reading URL generation UI
6. Link validator
7. Backup and restore scripts
8. Semi-automated discovery importer

## Step Zero: Public Hello World

Before adding SQLite, Datastar, migrations, importer logic, or topic routes, prove the deployment path with the smallest useful Go application.

Bare minimum repository pieces:

- `go.mod`
- `cmd/web/main.go`
- `scripts/build.sh` or documented build commands
- deployment notes for systemd and Caddy

Initial routes:

```text
GET /        returns a simple DailyDocs page
GET /health  returns ok
```

Initial configuration:

```text
ADDR=:8080
```

Definition of done:

- `https://dailydocs.dev` loads publicly
- `https://dailydocs.dev/health` returns `ok`
- Caddy terminates TLS and proxies to the Go app
- the app runs under systemd or an equivalent supervisor
- the repository documents enough steps to rebuild the deployment on a fresh VPS

Do not include SQLite, Datastar, migrations, importer commands, validator logic, or topic routes in this milestone.

## Core Domain First

Define the database and selection rules before building much UI.

Initial tables:

- `topics`
- `pages`
- `daily_readings`
- `imports`
- `schema_migrations`

Key constraints:

- `topics.slug` is unique
- `pages(topic_id, url)` is unique
- `daily_readings(topic_id, reading_date)` is unique
- only active pages are eligible for new readings
- historical `daily_readings` rows are preserved

## Daily Assignment

The web app should have one core domain operation:

```text
GetDailyReading(topic, date) -> page
```

Behavior:

1. Check `daily_readings` for the topic/date pair.
2. If present, return the assigned page.
3. If missing, select from active pages.
4. Insert the assignment.
5. Return the assigned page.

This logic should be heavily tested because it is the product.

## Seed Data Before Automation

Do not begin with a complex crawler.

Start with a simple human-readable import format:

```yaml
topic: sqlite
name: SQLite
pages:
  - title: Write-Ahead Logging
    url: https://sqlite.org/wal.html
    source: SQLite Documentation
    official: true
    estimated_minutes: 12
```

Build a command:

```sh
dailydocs import-file topics/sqlite.yaml
```

This lets the product launch with documentation links immediately. Automated discovery can follow after the shape of good data is clearer.

## Web Application

Build a small Go monolith.

Routes:

```text
GET /                         topic picker
GET /{topic}                  today's reading page
GET /{topic}/{date}           daily reading page
GET /topics/search?q=go       autocomplete endpoint
```

The topic-only route is the common product URL:

```text
/sqlite
```

The topic/date route is the stable archive URL:

```text
/sqlite/2026-06-26
```

The homepage shows the topic picker and can send the user to the topic-only URL for the selected topic.

## Datastar Scope

Use Datastar modestly for:

- autocomplete
- selecting one topic
- generating the bookmarkable URL

Do not turn the app into a complex single-page application. The URL is the state.

## Link Validator

The validator is more important than the automated importer.

Command:

```sh
dailydocs validate-links
```

Responsibilities:

- check active pages
- follow redirects
- mark repeated failures
- update `last_verified`
- optionally propose URL updates

Broken links should be detected before broad importer automation.

## Semi-Automated Importer

After manual seeds and validator work, add a review-first discovery command:

```sh
dailydocs discover sqlite https://sqlite.org/docs.html
```

The command should output candidates to a review file, not insert them directly into production data.

Human approval stays in the loop for MVP.

## Deployment

Deploy as one Go binary with SQLite behind Caddy.

Application startup:

1. Open database.
2. Apply migrations.
3. Serve HTTP.

Operational scripts:

- `bootstrap.sh`
- `backup.sh`
- `restore.sh`
- `validate-links`
- `import-file`

## MVP Content Bar

Ship with 5-10 supported topics and documentation links.

This validates the daily reading flow before investing in automation.
