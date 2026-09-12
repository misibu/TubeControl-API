package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func (a *App) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"activated":   a.cfg.Token != "",
		"server":      a.cfg.Server,
		"device_name": a.cfg.DeviceName,
		"device_id":   a.cfg.DeviceID,
	})
}

func (a *App) activate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code       string `json:"code"`
		DeviceName string `json:"device_name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "Неверный запрос"})
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	if req.DeviceName == "" {
		host, _ := os.Hostname()
		req.DeviceName = host
	}
	payload := map[string]string{"code": req.Code, "device_name": req.DeviceName}
	var out struct {
		Status      string `json:"status"`
		DeviceToken string `json:"device_token"`
		Device      struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"device"`
	}
	code, err := a.cloudJSON("POST", "/api/v2/activate", payload, "", &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]any{"error": "Ключ активации не принят", "details": errString(err)})
		return
	}
	a.cfg.Token = out.DeviceToken
	a.cfg.DeviceID = out.Device.ID
	a.cfg.DeviceName = out.Device.Name
	if a.cfg.Server == "" {
		a.cfg.Server = defaultServer
	}
	_ = a.saveConfig()
	writeJSON(w, 200, map[string]any{"status": "ok", "device": out.Device})
}

func (a *App) tubes(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var out struct {
		Items []Tube `json:"items"`
		Count int    `json:"count"`
	}
	code, err := a.cloudJSON("GET", "/api/v1/tubes?limit=5000", nil, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]any{"error": "Не удалось получить данные", "details": errString(err)})
		return
	}
	writeJSON(w, 200, out)
}

func (a *App) enrollment(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var out struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expires_at"`
	}
	code, err := a.cloudJSON("POST", "/api/v2/enrollments", map[string]any{}, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]string{"error": "Не удалось создать QR"})
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"type":   "tubecontrol-enroll-v2",
		"server": a.cfg.Server,
		"code":   out.Code,
	})
	png, err := qrcode.Encode(string(payload), qrcode.Medium, 420)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Ошибка QR"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"qr":         "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		"expires_at": out.ExpiresAt,
		"code":       out.Code,
	})
}

func (a *App) windowsKey(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var out map[string]any
	code, err := a.cloudJSON("POST", "/api/v2/activation-codes", map[string]any{}, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]string{"error": "Не удалось создать ключ"})
		return
	}
	writeJSON(w, 200, out)
}

func (a *App) devices(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var out map[string]any
	code, err := a.cloudJSON("GET", "/api/v2/devices", nil, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]string{"error": "Не удалось получить устройства"})
		return
	}
	writeJSON(w, 200, out)
}

func (a *App) revoke(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var out map[string]any
	code, err := a.cloudJSON("POST", "/api/v2/devices/"+strings.TrimSpace(req.ID)+"/revoke", map[string]any{}, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, codeOr(code, 502), map[string]string{"error": "Не удалось отключить устройство"})
		return
	}
	writeJSON(w, 200, out)
}
