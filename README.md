# TubeControl API

Backend for TubeControl Windows and Android scanner.

## Required environment variables

- `DB_HOST` — PostgreSQL host, e.g. `192.168.0.4`
- `DB_PORT` — PostgreSQL port, usually `5432`
- `DB_NAME` — database name, e.g. `default_db`
- `DB_USER` — database user, e.g. `gen_user`
- `DB_PASSWORD` — PostgreSQL password
- `DB_SSLMODE` — `disable` for private Timeweb network, or `require` when TLS is enabled
- `DESKTOP_TOKEN` — secret API key used by TubeControl.exe
- `SCANNER_TOKEN` — secret API key used by Android scanner
- `PORT` — optional, default `8080`

Instead of the DB_* variables you may provide `DATABASE_URL`.

## Endpoints

- `GET /health` — health check, no auth
- `POST /api/v1/returns` — Android scanner, header `X-API-Key: <SCANNER_TOKEN>`
- `POST /api/v1/tubes/import` — Windows import, header `X-API-Key: <DESKTOP_TOKEN>`
- `GET /api/v1/tubes` — Windows list, header `X-API-Key: <DESKTOP_TOKEN>`
- `GET /api/v1/tubes/{thu}` — Windows lookup
- `GET /api/v1/sync?since=<RFC3339>` — incremental Windows sync

The server creates its PostgreSQL tables and indexes automatically on startup.

## Import example

```json
{
  "items": [
    {
      "thu": "X0047937",
      "shipped_at": "2026-09-06",
      "store_code": "0358",
      "order_number": "4500123456"
    }
  ]
}
```

## Android return example

```json
{
  "thu": "X0047937"
}
```

Possible responses include `returned`, `already_returned`, and `not_found`.
