package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type statusUpdateRequest struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
}

func registerStatusRoutes(mux *http.ServeMux, a *App) {
	mux.Handle("GET /api/v3/tubes", a.requireDesktop(a.requireDB(http.HandlerFunc(a.listTubesV3))))
	mux.Handle("GET /api/v3/tubes/{thu}", a.requireDesktop(a.requireDB(http.HandlerFunc(a.getTubeV3))))
	mux.Handle("PATCH /api/v3/tubes/{thu}/status", a.requireDesktop(a.requireDB(http.HandlerFunc(a.updateTubeStatus))))
}

type TubeV3 struct {
	THU           string     `json:"thu"`
	ShippedAt     string     `json:"shipped_at"`
	StoreCode     string     `json:"store_code"`
	OrderNumber   string     `json:"order_number"`
	ReturnedAt    *time.Time `json:"returned_at,omitempty"`
	ManualStatus  *string    `json:"manual_status,omitempty"`
	Status        string     `json:"status"`
	StatusComment *string    `json:"status_comment,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (a *App) listTubesV3(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureV3Schema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not prepare v3 schema")
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT thu,shipped_at,store_code,order_number,returned_at,manual_status,status_comment,updated_at FROM tubes ORDER BY shipped_at DESC,thu ASC LIMIT 5000`)
	if err != nil {
		writeError(w, 500, "database_error", "Could not list tubes")
		return
	}
	defer rows.Close()
	items := make([]TubeV3, 0)
	for rows.Next() {
		var t TubeV3
		var shipped time.Time
		if err := rows.Scan(&t.THU, &shipped, &t.StoreCode, &t.OrderNumber, &t.ReturnedAt, &t.ManualStatus, &t.StatusComment, &t.UpdatedAt); err != nil {
			writeError(w, 500, "database_error", "Could not read tube")
			return
		}
		t.ShippedAt = shipped.Format("2006-01-02")
		t.Status = effectiveTubeStatus(t.ManualStatus, t.ReturnedAt, shipped)
		items = append(items, t)
	}
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items)})
}

func (a *App) getTubeV3(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureV3Schema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not prepare v3 schema")
		return
	}
	thu := normalizeTHU(r.PathValue("thu"))
	var t TubeV3
	var shipped time.Time
	err := a.db.QueryRow(r.Context(), `SELECT thu,shipped_at,store_code,order_number,returned_at,manual_status,status_comment,updated_at FROM tubes WHERE thu=$1`, thu).Scan(&t.THU, &shipped, &t.StoreCode, &t.OrderNumber, &t.ReturnedAt, &t.ManualStatus, &t.StatusComment, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "THU not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "Could not read tube")
		return
	}
	t.ShippedAt = shipped.Format("2006-01-02")
	t.Status = effectiveTubeStatus(t.ManualStatus, t.ReturnedAt, shipped)
	writeJSON(w, 200, t)
}

func (a *App) updateTubeStatus(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureV3Schema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not prepare v3 schema")
		return
	}
	thu := normalizeTHU(r.PathValue("thu"))
	if thu == "" {
		writeError(w, http.StatusBadRequest, "thu_required", "THU is required")
		return
	}
	var req statusUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.Comment = strings.TrimSpace(req.Comment)
	if len([]rune(req.Comment)) > 500 {
		writeError(w, http.StatusBadRequest, "comment_too_long", "Comment must be at most 500 characters")
		return
	}
	valid := map[string]bool{"auto": true, "sent": true, "transit": true, "returned": true, "lost": true}
	if !valid[req.Status] {
		writeError(w, http.StatusBadRequest, "invalid_status", "status must be auto, sent, transit, returned or lost")
		return
	}

	tx, err := a.db.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		writeError(w, 500, "database_error", "Could not start status update")
		return
	}
	defer tx.Rollback(r.Context())

	var oldManual *string
	var oldReturned *time.Time
	err = tx.QueryRow(r.Context(), `SELECT manual_status, returned_at FROM tubes WHERE thu=$1 FOR UPDATE`, thu).Scan(&oldManual, &oldReturned)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "THU not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "Could not read THU")
		return
	}

	oldStatus := effectiveTubeStatus(oldManual, oldReturned, time.Time{})
	if req.Status == "auto" {
		_, err = tx.Exec(r.Context(), `UPDATE tubes SET manual_status=NULL,status_comment=$2,status_updated_at=NOW(),updated_at=NOW() WHERE thu=$1`, thu, nullableString(req.Comment))
	} else if req.Status == "returned" {
		_, err = tx.Exec(r.Context(), `UPDATE tubes SET manual_status='returned',status_comment=$2,status_updated_at=NOW(),returned_at=COALESCE(returned_at,NOW()),updated_at=NOW() WHERE thu=$1`, thu, nullableString(req.Comment))
	} else {
		_, err = tx.Exec(r.Context(), `UPDATE tubes SET manual_status=$2,status_comment=$3,status_updated_at=NOW(),returned_at=NULL,updated_at=NOW() WHERE thu=$1`, thu, req.Status, nullableString(req.Comment))
	}
	if err != nil {
		writeError(w, 500, "database_error", "Could not update status")
		return
	}

	var newManual *string
	var newReturned *time.Time
	var shippedAt time.Time
	if err := tx.QueryRow(r.Context(), `SELECT manual_status,returned_at,shipped_at FROM tubes WHERE thu=$1`, thu).Scan(&newManual, &newReturned, &shippedAt); err != nil {
		writeError(w, 500, "database_error", "Could not read updated status")
		return
	}
	newStatus := effectiveTubeStatus(newManual, newReturned, shippedAt)
	_, err = tx.Exec(r.Context(), `INSERT INTO tube_status_events(thu,old_status,new_status,comment,device_id,created_at) VALUES($1,$2,$3,$4,$5,NOW())`, thu, oldStatus, newStatus, nullableString(req.Comment), nullableString(requestDeviceID(r)))
	if err != nil {
		writeError(w, 500, "database_error", "Could not write status history")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not commit status update")
		return
	}

	writeJSON(w, 200, map[string]any{"status": "ok", "thu": thu, "tube_status": newStatus})
}

func effectiveTubeStatus(manual *string, returned *time.Time, shipped time.Time) string {
	if returned != nil {
		return "returned"
	}
	if manual != nil && strings.TrimSpace(*manual) != "" {
		return strings.TrimSpace(*manual)
	}
	if !shipped.IsZero() && time.Since(shipped) > 30*24*time.Hour {
		return "overdue"
	}
	return "transit"
}

func (a *App) ensureV3Schema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `
ALTER TABLE tubes ADD COLUMN IF NOT EXISTS manual_status TEXT NULL;
ALTER TABLE tubes ADD COLUMN IF NOT EXISTS status_comment TEXT NULL;
ALTER TABLE tubes ADD COLUMN IF NOT EXISTS status_updated_at TIMESTAMPTZ NULL;
CREATE TABLE IF NOT EXISTS tube_status_events (
    id BIGSERIAL PRIMARY KEY,
    thu TEXT NOT NULL,
    old_status TEXT NULL,
    new_status TEXT NOT NULL,
    comment TEXT NULL,
    device_id TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tube_status_events_thu ON tube_status_events(thu,created_at DESC);
CREATE TABLE IF NOT EXISTS public_activation_requests (
    id BIGSERIAL PRIMARY KEY,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_public_activation_requests_hash_time ON public_activation_requests(request_hash,created_at DESC);
`)
	return err
}
