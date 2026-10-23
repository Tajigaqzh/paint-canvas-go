# paint-canvas-go

Gin backend starter for the Paint Canvas document API.

## Stack

- Go 1.26
- Gin HTTP router
- GORM with pure-Go SQLite driver
- Zap structured request logging
- SQLite auto-migration on startup

## Run

```powershell
Copy-Item .env.example .env
go mod tidy
go run ./cmd/server
```

The API listens on `:8080` by default and creates `data/paint-canvas.db`. Environment variables are read directly by the process; load `.env` in your shell or IDE.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Process and database health |
| `GET` | `/api/v1/documents?limit=20&offset=0` | Paginated document summaries |
| `POST` | `/api/v1/documents` | Create a canvas document |
| `GET` | `/api/v1/documents/:id` | Read a document |
| `PUT` | `/api/v1/documents/:id` | Replace name and document data |
| `DELETE` | `/api/v1/documents/:id` | Delete a document |

Create body:

```json
{
  "name": "Untitled",
  "data": {
    "activePageId": "page-id",
    "pageIds": ["page-id"],
    "pages": { "page-id": {} }
  }
}
```

Updates must include the latest `revision` returned by the server. The revision increments on success; stale writes return `409 revision_conflict` instead of silently overwriting newer work. Document JSON is limited to 16 MiB.

## Configuration

- `APP_ENV`: `development` or `production`
- `HTTP_ADDR`: listen address, default `:8080`
- `DATABASE_DSN`: SQLite DSN, default `file:./data/paint-canvas.db`
- `CORS_ORIGINS`: comma-separated allowed browser origins; defaults to Vite's local port `5174`

## Verify

```powershell
gofmt -w .
go test ./...
go vet ./...
```
