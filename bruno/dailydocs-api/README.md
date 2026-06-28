# DailyDocs Bruno Collection

Open this folder in Bruno:

```text
bruno/dailydocs-api
```

Use the `Local` environment and set:

```text
OPENAI_API_KEY
```

The collection does not store an API key. The request uses:

```text
Authorization: Bearer {{OPENAI_API_KEY}}
```

## Requests

- `OpenAI / DailyDocs Gate Review`

This sends the single-page DailyDocs gate-review request for:

```text
https://go.dev/doc/effective_go
```

It uses:

```text
model: gpt-5-nano
reasoning.effort: low
```
