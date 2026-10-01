package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

func webRequestBaseURL(r *http.Request) string {
	if v := strings.TrimSpace(envDefault("PUBLIC_BASE_URL", "")); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Host)
	if scheme == "http" && host != "" && !strings.HasPrefix(host, "localhost") && !strings.HasPrefix(host, "127.0.0.1") {
		scheme = "https"
	}
	return scheme + "://" + host
}

func (a *App) webAndroidEnrollment(w http.ResponseWriter, r *http.Request) {
	code, err := makeHumanCode("A")
	if err != nil {
		writeError(w, 500, "random_error", "Не удалось создать код подключения")
		return
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if _, err := a.db.Exec(r.Context(), "INSERT INTO activation_codes(code_hash,kind,created_by_device_id,expires_at) VALUES($1,'android',NULL,$2)", hashSecret(code), expires); err != nil {
		writeError(w, 500, "database_error", "Не удалось создать подключение Android")
		return
	}
	server := webRequestBaseURL(r)
	payloadBytes, _ := json.Marshal(map[string]string{
		"type":   "tubecontrol-enroll-v2",
		"server": server,
		"code":   code,
	})
	png, err := qrcode.Encode(string(payloadBytes), qrcode.Medium, 360)
	if err != nil {
		writeError(w, 500, "qr_error", "Не удалось создать QR-код")
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":     "ok",
		"code":       code,
		"expires_at": expires,
		"qr_png":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (a *App) webDeviceList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), "SELECT id,name,kind,created_at,last_seen_at,revoked_at FROM devices ORDER BY created_at DESC")
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось загрузить устройства")
		return
	}
	defer rows.Close()
	items := make([]DeviceRecord, 0)
	for rows.Next() {
		var d DeviceRecord
		if err := rows.Scan(&d.ID, &d.Name, &d.Kind, &d.CreatedAt, &d.LastSeenAt, &d.RevokedAt); err != nil {
			writeError(w, 500, "database_error", "Не удалось прочитать устройство")
			return
		}
		items = append(items, d)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) webDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, 400, "device_required", "Не указано устройство")
		return
	}
	tag, err := a.db.Exec(r.Context(), "UPDATE devices SET revoked_at=NOW() WHERE id=$1 AND kind='android' AND revoked_at IS NULL", id)
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось отключить устройство")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "device_not_found", "Android-устройство не найдено или уже отключено")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok"})
}
