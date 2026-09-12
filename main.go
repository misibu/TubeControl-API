package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db           *pgxpool.Pool
	desktopToken string
	scannerToken string
	ready        atomic.Bool
}

type Tube struct {
	THU         string     `json:"thu"`
	ShippedAt   string     `json:"shipped_at"`
	StoreCode   string     `json:"store_code"`
	OrderNumber string     `json:"order_number"`
	ReturnedAt  *time.Time `json:"returned_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ImportItem struct {
	THU         string `json:"thu"`
	ShippedAt   string `json:"shipped_at"`
	StoreCode   string `json:"store_code"`
	OrderNumber string `json:"order_number"`
}

type ImportRequest struct {
	Items []ImportItem `json:"items"`
}
type ReturnRequest struct {
	THU       string     `json:"thu"`
	ScannedAt *time.Time `json:"scanned_at,omitempty"`
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL())
	if err != nil {
		log.Fatalf("database config error: %v", err)
	}
	defer pool.Close()

	app := &App{
		db:           pool,
		desktopToken: strings.TrimSpace(os.Getenv("DESKTOP_TOKEN")),
		scannerToken: strings.TrimSpace(os.Getenv("SCANNER_TOKEN")),
	}
	if app.desktopToken == "" || app.scannerToken == "" {
		log.Fatal("DESKTOP_TOKEN and SCANNER_TOKEN must be set")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("GET /ready", app.readiness)
	mux.Handle("POST /api/v1/returns", app.requireScanner(app.requireDB(http.HandlerFunc(app.registerReturn))))
	mux.Handle("POST /api/v1/tubes/import", app.requireDesktop(app.requireDB(http.HandlerFunc(app.importTubes))))
	mux.Handle("GET /api/v1/tubes", app.requireDesktop(app.requireDB(http.HandlerFunc(app.listTubes))))
	mux.Handle("GET /api/v1/tubes/{thu}", app.requireDesktop(app.requireDB(http.HandlerFunc(app.getTube))))
	mux.Handle("GET /api/v1/sync", app.requireDesktop(app.requireDB(http.HandlerFunc(app.syncTubes))))
	registerDeviceRoutes(mux, app)
	registerSiteRoutes(mux, app)
	registerStatusRoutes(mux, app)

	port := envDefault("PORT", "8080")
	addr := "0.0.0.0:" + port
	server := &http.Server{
		Addr: addr, Handler: requestLog(mux),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("TubeControl API listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	go app.initializeDatabase(ctx)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func (a *App) initializeDatabase(ctx context.Context) {
	for {
		checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := a.db.Ping(checkCtx)
		if err == nil {
			err = migrate(checkCtx, a.db)
		}
		if err == nil {
			err = a.ensureBootstrapActivation(checkCtx)
		}
		cancel()
		if err == nil {
			a.ready.Store(true)
			log.Printf("PostgreSQL connected; schema ready")
			return
		}
		a.ready.Store(false)
		log.Printf("PostgreSQL not ready: %v; retrying in 5s", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func databaseURL() string {
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return raw
	}
	host := envDefault("DB_HOST", "127.0.0.1")
	port := envDefault("DB_PORT", "5432")
	name := envDefault("DB_NAME", "default_db")
	user := envDefault("DB_USER", "postgres")
	password := os.Getenv("DB_PASSWORD")
	sslmode := envDefault("DB_SSLMODE", "disable")
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: host + ":" + port, Path: "/" + name}
	q := u.Query()
	q.Set("sslmode", sslmode)
	q.Set("connect_timeout", "5")
	u.RawQuery = q.Encode()
	return u.String()
}

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS tubes (
    thu TEXT PRIMARY KEY,
    shipped_at DATE NOT NULL,
    store_code TEXT NOT NULL DEFAULT '',
    order_number TEXT NOT NULL DEFAULT '',
    returned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tubes_shipped_at ON tubes(shipped_at);
CREATE INDEX IF NOT EXISTS idx_tubes_returned_at ON tubes(returned_at);
CREATE INDEX IF NOT EXISTS idx_tubes_updated_at ON tubes(updated_at);
CREATE TABLE IF NOT EXISTS scan_events (
    id BIGSERIAL PRIMARY KEY,
    thu TEXT NOT NULL,
    scanned_at TIMESTAMPTZ NOT NULL,
    result TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'android',
    device_id TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE scan_events ADD COLUMN IF NOT EXISTS device_id TEXT NULL;
CREATE INDEX IF NOT EXISTS idx_scan_events_thu ON scan_events(thu);
CREATE INDEX IF NOT EXISTS idx_scan_events_scanned_at ON scan_events(scanned_at);
`)
	if err != nil {
		return err
	}
	return migrateDevices(ctx, db)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "TubeControl API", "time": time.Now().UTC()})
}

func (a *App) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil || !a.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "database": "ok"})
}

func (a *App) requireDB(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.ready.Load() {
			writeError(w, http.StatusServiceUnavailable, "database_not_ready", "Database is not ready")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) registerReturn(w http.ResponseWriter, r *http.Request) {
	var req ReturnRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.THU = normalizeTHU(req.THU)
	if req.THU == "" {
		writeError(w, http.StatusBadRequest, "thu_required", "THU is required")
		return
	}
	scannedAt := time.Now().UTC()
	if req.ScannedAt != nil {
		scannedAt = req.ScannedAt.UTC()
	}
	deviceID := requestDeviceID(r)
	tx, err := a.db.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	var existing *time.Time
	err = tx.QueryRow(r.Context(), `SELECT returned_at FROM tubes WHERE thu=$1 FOR UPDATE`, req.THU).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = tx.Exec(r.Context(), `INSERT INTO scan_events(thu,scanned_at,result,device_id) VALUES($1,$2,'not_found',$3)`, req.THU, scannedAt, nullableString(deviceID))
		_ = tx.Commit(r.Context())
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "not_found", "thu": req.THU})
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not read THU")
		return
	}
	if existing != nil {
		_, _ = tx.Exec(r.Context(), `INSERT INTO scan_events(thu,scanned_at,result,device_id) VALUES($1,$2,'already_returned',$3)`, req.THU, scannedAt, nullableString(deviceID))
		if err := tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_error", "Could not save scan")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "already_returned", "thu": req.THU, "returned_at": existing.UTC()})
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE tubes SET returned_at=$2, updated_at=NOW() WHERE thu=$1`, req.THU, scannedAt)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO scan_events(thu,scanned_at,result,device_id) VALUES($1,$2,'returned',$3)`, req.THU, scannedAt, nullableString(deviceID))
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not register return")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not commit return")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "returned", "thu": req.THU, "returned_at": scannedAt})
}

func (a *App) importTubes(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items_required", "At least one item is required")
		return
	}
	if len(req.Items) > 5000 {
		writeError(w, http.StatusBadRequest, "too_many_items", "Maximum 5000 items per request")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not start import")
		return
	}
	defer tx.Rollback(r.Context())
	processed := 0
	for i, item := range req.Items {
		item.THU = normalizeTHU(item.THU)
		if item.THU == "" {
			writeError(w, http.StatusBadRequest, "invalid_item", fmt.Sprintf("Item %d has empty THU", i+1))
			return
		}
		shippedAt, err := time.Parse("2006-01-02", strings.TrimSpace(item.ShippedAt))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_date", fmt.Sprintf("Item %d shipped_at must be YYYY-MM-DD", i+1))
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO tubes(thu,shipped_at,store_code,order_number) VALUES($1,$2,$3,$4)
ON CONFLICT (thu) DO UPDATE SET shipped_at=EXCLUDED.shipped_at, store_code=EXCLUDED.store_code, order_number=EXCLUDED.order_number, updated_at=NOW()`, item.THU, shippedAt, strings.TrimSpace(item.StoreCode), strings.TrimSpace(item.OrderNumber))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_error", fmt.Sprintf("Could not import item %d", i+1))
			return
		}
		processed++
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not commit import")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "processed": processed})
}

func (a *App) listTubes(w http.ResponseWriter, r *http.Request) {
	query := `SELECT thu,shipped_at,store_code,order_number,returned_at,created_at,updated_at FROM tubes WHERE 1=1`
	args := []any{}
	if v := r.URL.Query().Get("returned"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_returned", "returned must be true or false")
			return
		}
		if b {
			query += ` AND returned_at IS NOT NULL`
		} else {
			query += ` AND returned_at IS NULL`
		}
	}
	for _, p := range []struct{ name, op string }{{"from", ">="}, {"to", "<="}} {
		if raw := strings.TrimSpace(r.URL.Query().Get(p.name)); raw != "" {
			d, err := time.Parse("2006-01-02", raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_date", p.name+" must be YYYY-MM-DD")
				return
			}
			args = append(args, d)
			query += fmt.Sprintf(" AND shipped_at %s $%d", p.op, len(args))
		}
	}
	limit := 1000
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 5000 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 5000")
			return
		}
		limit = n
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY shipped_at DESC, thu ASC LIMIT $%d`, len(args))
	rows, err := a.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not list tubes")
		return
	}
	defer rows.Close()
	items := make([]Tube, 0)
	for rows.Next() {
		t, err := scanTube(rows)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_error", "Could not read tube")
			return
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *App) getTube(w http.ResponseWriter, r *http.Request) {
	thu := normalizeTHU(r.PathValue("thu"))
	t, err := scanTube(a.db.QueryRow(r.Context(), `SELECT thu,shipped_at,store_code,order_number,returned_at,created_at,updated_at FROM tubes WHERE thu=$1`, thu))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "THU not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not read THU")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *App) syncTubes(w http.ResponseWriter, r *http.Request) {
	sinceRaw := strings.TrimSpace(r.URL.Query().Get("since"))
	if sinceRaw == "" {
		writeError(w, http.StatusBadRequest, "since_required", "since is required (RFC3339)")
		return
	}
	since, err := time.Parse(time.RFC3339, sinceRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_since", "since must be RFC3339")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT thu,shipped_at,store_code,order_number,returned_at,created_at,updated_at FROM tubes WHERE updated_at > $1 ORDER BY updated_at ASC LIMIT 5000`, since.UTC())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not sync tubes")
		return
	}
	defer rows.Close()
	items := make([]Tube, 0)
	for rows.Next() {
		t, err := scanTube(rows)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_error", "Could not read sync data")
			return
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items), "server_time": time.Now().UTC()})
}

type tubeScanner interface{ Scan(dest ...any) error }

func scanTube(row tubeScanner) (Tube, error) {
	var t Tube
	var shippedAt time.Time
	err := row.Scan(&t.THU, &shippedAt, &t.StoreCode, &t.OrderNumber, &t.ReturnedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.ShippedAt = shippedAt.Format("2006-01-02")
	return t, nil
}
func normalizeTHU(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return err
	}
	return nil
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": code, "message": message})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
func envDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
