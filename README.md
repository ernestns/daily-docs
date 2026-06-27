# DailyDocs

DailyDocs helps developers build a sustainable learning habit by recommending one carefully curated documentation page per topic per day.

The common reading URL is intentionally simple:

```text
https://dailydocs.dev/go
```

That URL resolves to today's reading for Go. A dated URL provides a stable historical reading:

```text
https://dailydocs.dev/go/2026-06-26
```

No accounts. No sessions. No cookies. The URL is the state.

## MVP

DailyDocs is a small Go application backed by SQLite.

The MVP supports:

- topic search
- one daily reading per topic
- deterministic historical readings
- manually curated documentation pages
- official documentation first
- link validation

The documentation itself is never hosted by DailyDocs. Users are sent to the source documentation.

## Architecture

DailyDocs is planned as:

- one Go web application
- one SQLite database
- one manual import executable
- one link validation executable
- Caddy in front of the app

Infrastructure should remain simple enough to run on a single VPS.

## Documentation

- [Product specification](docs/product-spec.md)
- [Implementation strategy](docs/implementation-strategy.md)
- [Decision log](docs/decision-log.md)

## Guiding Principle

Every design decision should answer one question:

> Does this make it easier for someone to learn one useful thing today?
