# DailyDocs

DailyDocs recommends one documentation link per topic per day.

The common reading URL is:

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
- documentation links
- official documentation first
- link validation

The documentation itself is never hosted by DailyDocs. Users are sent to the source documentation.

## Architecture

DailyDocs is planned as:

- one Go web application
- one SQLite database
- one import executable
- one link validation executable
- Caddy in front of the app

Infrastructure runs on a single VPS.

## Documentation

- [Product specification](docs/product-spec.md)
- [Implementation strategy](docs/implementation-strategy.md)
- [Decision log](docs/decision-log.md)

## Guiding Principle

Every design decision should answer one question:

> Does this help someone read one documentation page today?
