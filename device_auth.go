package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const deviceContextKey contextKey = "tubecontrol-device"

type DeviceIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type DeviceRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type activationRequest struct {
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}
type enrollmentRequest struct {
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}

type tokenResponse struct {
	Status      string         `json:"status"`
	Device      DeviceIdentity `json:"device"`
	DeviceToken string         `json:"device_token"`
}

func migrateDevices(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('desktop','android')),
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_devices_kind ON devices(kind);
CREATE INDEX IF NOT EXISTS idx_devices_active ON devices(kind, revoked_at);
CREATE TABLE IF NOT EXISTS activation_codes (
    code_hash TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('desktop','android')),
    created_by_device_id TEXT NULL REFERENCES devices(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_activation_codes_valid ON activation_codes(kind, expires_at, used_at);
`)
	return err
}

func registerDeviceRoutes(mux *http.ServeMux, a *App) {
	mux.Handle("GET /api/v2/bootstrap/status", a.requireDB(http.HandlerFunc(a.bootstrapStatus)))
	mux.Handle("POST /api/v2/activate", a.requireDB(http.HandlerFunc(a.activateDesktop)))
	mux.Handle("POST /api/v2/enroll", a.requireDB(http.HandlerFunc(a.enrollAndroid)))
	mux.Handle("GET /api/v2/me", a.requireDB(a.requireDeviceKind("", http.HandlerFunc(a.me))))
	mux.Handle("POST /api/v2/enrollments", a.requireDB(a.requireDeviceKind("desktop", http.HandlerFunc(a.createAndroidEnrollment))))
	mux.Handle("POST /api/v2/activation-codes", a.requireDB(a.requireDeviceKind("desktop", http.HandlerFunc(a.createDesktopActivationCode))))
	mux.Handle("GET /api/v2/devices", a.requireDB(a.requireDeviceKind("desktop", http.HandlerFunc(a.listDevices))))
	mux.Handle("POST /api/v2/devices/{id}/revoke", a.requireDB(a.requireDeviceKind("desktop", http.HandlerFunc(a.revokeDevice))))
}

func (a *App) ensureBootstrapActivation(ctx context.Context) error {
	var count int
	if err := a.db.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE kind='desktop' AND revoked_at IS NULL`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if _, err := a.db.Exec(ctx, `DELETE FROM activation_codes WHERE kind='desktop' AND created_by_device_id IS NULL AND used_at IS NULL`); err != nil {
		return err
	}
	code, err := makeHumanCode("TC")
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(48 * time.Hour)
	if _, err := a.db.Exec(ctx, `INSERT INTO activation_codes(code_hash,kind,expires_at) VALUES($1,'desktop',$2)`, hashSecret(code), expires); err != nil {
		return err
	}
	log.Printf("============================================================")
	log.Printf("TubeControl FIRST WINDOWS ACTIVATION KEY: %s", code)
	log.Printf("Valid until: %s", expires.Format(time.RFC3339))
	log.Printf("Enter this key once in TubeControl Windows. Do not publish it.")
	log.Printf("============================================================")
	return nil
}

func (a *App) bootstrapStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM devices WHERE kind='desktop' AND revoked_at IS NULL`).Scan(&count); err != nil {
		writeError(w, 500, "database_error", "Could not read activation state")
		return
	}
	writeJSON(w, 200, map[string]any{"needs_activation": count == 0})
}

func (a *App) activateDesktop(w http.ResponseWriter, r *http.Request) {
	var req activationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.DeviceName = cleanDeviceName(req.DeviceName, "Windows PC")
	token, device, err := a.consumeCodeAndCreateDevice(r.Context(), req.Code, "desktop", req.DeviceName)
	if errors.Is(err, errInvalidCode) {
		writeError(w, 401, "invalid_activation_code", "Activation key is invalid, expired, or already used")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "Could not activate device")
		return
	}
	writeJSON(w, 200, tokenResponse{Status: "activated", Device: device, DeviceToken: token})
}

func (a *App) enrollAndroid(w http.ResponseWriter, r *http.Request) {
	var req enrollmentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.DeviceName = cleanDeviceName(req.DeviceName, "Android scanner")
	token, device, err := a.consumeCodeAndCreateDevice(r.Context(), req.Code, "android", req.DeviceName)
	if errors.Is(err, errInvalidCode) {
		writeError(w, 401, "invalid_enrollment_code", "QR code is invalid, expired, or already used")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "Could not enroll device")
		return
	}
	writeJSON(w, 200, tokenResponse{Status: "enrolled", Device: device, DeviceToken: token})
}

var errInvalidCode = errors.New("invalid activation code")

func (a *App) consumeCodeAndCreateDevice(ctx context.Context, code, kind, name string) (string, DeviceIdentity, error) {
	if code == "" {
		return "", DeviceIdentity{}, errInvalidCode
	}
	tx, err := a.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", DeviceIdentity{}, err
	}
	defer tx.Rollback(ctx)
	var storedKind string
	err = tx.QueryRow(ctx, `SELECT kind FROM activation_codes WHERE code_hash=$1 AND used_at IS NULL AND expires_at>NOW() FOR UPDATE`, hashSecret(code)).Scan(&storedKind)
	if errors.Is(err, pgx.ErrNoRows) || storedKind != kind {
		return "", DeviceIdentity{}, errInvalidCode
	}
	if err != nil {
		return "", DeviceIdentity{}, err
	}
	id, err := randomID()
	if err != nil {
		return "", DeviceIdentity{}, err
	}
	token, err := randomDeviceToken(kind)
	if err != nil {
		return "", DeviceIdentity{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO devices(id,name,kind,token_hash,last_seen_at) VALUES($1,$2,$3,$4,NOW())`, id, name, kind, hashSecret(token)); err != nil {
		return "", DeviceIdentity{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE activation_codes SET used_at=NOW() WHERE code_hash=$1`, hashSecret(code)); err != nil {
		return "", DeviceIdentity{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", DeviceIdentity{}, err
	}
	return token, DeviceIdentity{ID: id, Name: name, Kind: kind}, nil
}

func (a *App) createAndroidEnrollment(w http.ResponseWriter, r *http.Request) {
	id := requestDeviceID(r)
	code, err := makeHumanCode("A")
	if err != nil {
		writeError(w, 500, "random_error", "Could not create enrollment")
		return
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if _, err := a.db.Exec(r.Context(), `INSERT INTO activation_codes(code_hash,kind,created_by_device_id,expires_at) VALUES($1,'android',$2,$3)`, hashSecret(code), id, expires); err != nil {
		writeError(w, 500, "database_error", "Could not create enrollment")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "code": code, "expires_at": expires})
}

func (a *App) createDesktopActivationCode(w http.ResponseWriter, r *http.Request) {
	id := requestDeviceID(r)
	code, err := makeHumanCode("TC")
	if err != nil {
		writeError(w, 500, "random_error", "Could not create activation key")
		return
	}
	expires := time.Now().UTC().Add(48 * time.Hour)
	if _, err := a.db.Exec(r.Context(), `INSERT INTO activation_codes(code_hash,kind,created_by_device_id,expires_at) VALUES($1,'desktop',$2,$3)`, hashSecret(code), id, expires); err != nil {
		writeError(w, 500, "database_error", "Could not create activation key")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "code": code, "expires_at": expires})
}

func (a *App) listDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id,name,kind,created_at,last_seen_at,revoked_at FROM devices ORDER BY created_at ASC`)
	if err != nil {
		writeError(w, 500, "database_error", "Could not list devices")
		return
	}
	defer rows.Close()
	items := make([]DeviceRecord, 0)
	for rows.Next() {
		var d DeviceRecord
		if err := rows.Scan(&d.ID, &d.Name, &d.Kind, &d.CreatedAt, &d.LastSeenAt, &d.RevokedAt); err != nil {
			writeError(w, 500, "database_error", "Could not read device")
			return
		}
		items = append(items, d)
	}
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items), "current_device_id": requestDeviceID(r)})
}

func (a *App) revokeDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, 400, "device_required", "Device id is required")
		return
	}
	if id == requestDeviceID(r) {
		writeError(w, 400, "cannot_revoke_self", "Current Windows device cannot revoke itself")
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE devices SET revoked_at=NOW() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil {
		writeError(w, 500, "database_error", "Could not revoke device")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "Device not found or already disabled")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "revoked", "id": id})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	identity, _ := r.Context().Value(deviceContextKey).(DeviceIdentity)
	writeJSON(w, 200, map[string]any{"status": "ok", "device": identity})
}

func (a *App) requireDesktop(next http.Handler) http.Handler {
	return a.requireLegacyOrDevice("desktop", a.desktopToken, next)
}
func (a *App) requireScanner(next http.Handler) http.Handler {
	return a.requireLegacyOrDevice("android", a.scannerToken, next)
}

func (a *App) requireLegacyOrDevice(kind, legacyToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualLegacy := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if secureEqual(actualLegacy, legacyToken) {
			next.ServeHTTP(w, r)
			return
		}
		identity, err := a.authenticateDevice(r.Context(), bearerToken(r), kind)
		if err != nil {
			writeError(w, 401, "unauthorized", "Invalid or disabled device")
			return
		}
		ctx := context.WithValue(r.Context(), deviceContextKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) requireDeviceKind(kind string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := a.authenticateDevice(r.Context(), bearerToken(r), kind)
		if err != nil {
			writeError(w, 401, "unauthorized", "Device activation is required")
			return
		}
		ctx := context.WithValue(r.Context(), deviceContextKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) authenticateDevice(ctx context.Context, token, expectedKind string) (DeviceIdentity, error) {
	if token == "" {
		return DeviceIdentity{}, errors.New("missing token")
	}
	var d DeviceIdentity
	err := a.db.QueryRow(ctx, `SELECT id,name,kind FROM devices WHERE token_hash=$1 AND revoked_at IS NULL`, hashSecret(token)).Scan(&d.ID, &d.Name, &d.Kind)
	if err != nil {
		return DeviceIdentity{}, err
	}
	if expectedKind != "" && d.Kind != expectedKind {
		return DeviceIdentity{}, errors.New("wrong device kind")
	}
	_, _ = a.db.Exec(ctx, `UPDATE devices SET last_seen_at=NOW() WHERE id=$1`, d.ID)
	return d, nil
}

func requestDeviceID(r *http.Request) string {
	if d, ok := r.Context().Value(deviceContextKey).(DeviceIdentity); ok {
		return d.ID
	}
	return ""
}
func bearerToken(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(v) > 7 && strings.EqualFold(v[:7], "Bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Device-Token"))
}
func secureEqual(a, b string) bool {
	return len(a) == len(b) && len(a) > 0 && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func hashSecret(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func cleanDeviceName(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		v = fallback
	}
	if len([]rune(v)) > 80 {
		v = string([]rune(v)[:80])
	}
	return v
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "dev_" + hex.EncodeToString(b), nil
}
func randomDeviceToken(kind string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	prefix := "tcd_"
	if kind == "android" {
		prefix = "tca_"
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}
func makeHumanCode(prefix string) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, string(b[:4]), string(b[4:])), nil
}

var _ = json.Valid
