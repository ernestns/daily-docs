# DailyDocs Product Specification

Version: 0.1 (MVP)

## Vision

DailyDocs recommends one documentation link per topic per day.

Instead of searching for something to read, users receive one documentation link for a topic they care about.

A DailyDocs reading is simply a URL.

Example:

```text
https://dailydocs.dev/go
```

The common URL shows today's reading. A dated URL shows a specific historical reading.

Every visitor receives the same reading for a topic on a given day. Tomorrow the common URL changes.

No accounts. No setup. Bookmark the reading and read.

## Product Philosophy

DailyDocs has a limited scope.

It is not:

- a documentation search engine
- a documentation mirror
- a learning management system
- another social network

It is:

> A deterministic daily reading from software documentation.

The application is designed for repeated daily use.

## Core Principles

### Deterministic

Given a topic and date, DailyDocs always returns the same reading.

This enables:

- teams reading together
- shared discussions
- cache-friendly infrastructure
- reproducible URLs

### Stateless

Version 1 stores no user state.

No accounts, sessions, cookies, or local storage.

The URL is the reading.

### Useful

The application recommends documentation links selected from known documentation sources.

The catalog should stay small unless additional links improve the daily reading experience.

### Official First

Whenever possible, recommendations should come from official documentation.

Community resources are only used when an official source does not exist.

## Goals

- Encourage continuous learning
- Provide one reading per topic per day
- Promote official documentation
- Reduce the need to search for documentation to read
- Enable teams to learn together

## MVP User Flow

User visits `dailydocs.dev`, searches for a topic, then clicks `View Reading`.

Example topics:

- Go
- SQLite
- Docker

The generated reading URL is bookmarkable:

```text
https://dailydocs.dev/sqlite
```

The user visits every morning and reads that day's recommended documentation.

## Daily Reading

Each topic/date pair produces exactly one reading.

Example:

```text
Today's Reading

SQLite
Partial Indexes
12 min
Read
```

The documentation itself is never hosted by DailyDocs. Users are always sent to the source documentation.

## URL Format

Daily readings use path-based URLs:

```text
/{topic}
/{topic}/{date}
```

Examples:

```text
/go
/go/2026-06-26
/sqlite
/sqlite/2026-06-26
/docker
/docker/2026-06-26
```

The topic-only URL is the common bookmarkable URL and resolves to today's reading.

The dated URL is the stable archive URL for a specific reading date.

The topic path segment uses the topic slug. The date path segment uses `YYYY-MM-DD`.

The homepage may redirect a selected topic to the topic-only URL.

## Reading Selection

Each topic has a stable reading order.

During import:

1. Discover documentation pages
2. Filter poor candidates
3. Shuffle once
4. Store reading order

Example:

```text
SQLite

1 WAL Mode
2 Partial Indexes
3 VACUUM
4 Transactions
5 Query Planner
```

Daily readings are stored in a `daily_readings` assignment table rather than recomputed forever from the current page list. This preserves historical accuracy when documentation pages are added, removed, disabled, or reordered.

The application may lazily create a daily assignment on first request for a topic/date pair.

## Functional Requirements

### Search Topics

Autocomplete is supported.

Users search existing topics.

If a topic does not exist, offer `Import Topic`.

### View Reading

Support one topic per reading URL.

Produce a bookmarkable URL.

### Daily Reading

Display:

- title
- estimated reading time
- source
- official badge
- read button

## Import System

The importer is a separate executable.

Purpose: turn a topic into a reading list.

Example:

```text
Import "SQLite"
  -> Discover official documentation
  -> Extract pages
  -> Normalize URLs
  -> Remove duplicates
  -> Estimate reading time
  -> Assign metadata
  -> Shuffle reading order
  -> Store
```

Initially, imports are manually started.

## Link Validation

The validator is a separate executable.

Responsibilities:

- HEAD requests
- redirect handling
- broken link detection
- update `last_checked`
- disable consistently failing links

Broken links should never appear in new recommendations.

## Architecture

### Web Application

Responsibilities:

- topic search
- reading generation
- deterministic selection
- rendering

Technology:

- Go
- Datastar
- SQLite
- Caddy

Single monolith.

### Importer

Separate Go executable.

Responsibilities:

- topic discovery
- scraping
- parsing
- metadata generation
- deduplication
- reading order generation

Runs manually.

### Validator

Separate Go executable.

Responsibilities:

- link verification
- health checks
- redirect updates

Runs manually, scheduled later if desired.

## Data Model

### topics

- id
- slug
- name
- description
- status
- created_at

### pages

- id
- topic_id
- title
- url
- source
- official
- estimated_minutes
- difficulty
- evergreen_score
- reading_order
- active
- last_verified
- created_at
- updated_at

### daily_readings

- id
- topic_id
- reading_date
- page_id
- created_at

Unique constraint:

- topic_id
- reading_date

### imports

- id
- topic
- status
- started_at
- completed_at
- pages_found
- pages_imported
- error

## Deployment Philosophy

Infrastructure should fit on a single VPS.

Single Hetzner VPS, SQLite database, one Go web application, one importer executable, and one validator executable.

The repository is the source of truth.

A brand-new VPS should be recoverable by:

```text
Install Git
  -> Clone repository
  -> Run bootstrap.sh
  -> Restore SQLite backup
  -> Application online
```

Application startup automatically performs database migrations.

## Backups

SQLite operates in WAL mode.

Nightly backups use SQLite's backup mechanism.

Backups are:

- compressed
- uploaded to object storage
- retained daily, weekly, and monthly

Recovery should be a documented, tested process.

## Future Features

- User accounts
- Saved reading lists
- Reading history
- Read status
- Comments
- Community contributions
- Moderation
- Optional AI summaries, quizzes, difficulty estimation, or tagging

AI is never required for the core experience.

## Success Metrics

Primary:

- returning daily visitors
- reading bookmarks
- documentation click-through rate

Secondary:

- supported topics
- indexed pages
- broken link rate
- successful imports

## Guiding Principle

Every design decision should answer one question:

> Does this help someone read one documentation page today?

If the answer is no, it probably does not belong in DailyDocs.
