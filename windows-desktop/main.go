package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultServer = "https://tubecontrol-api-msk-misibun.amvera.io"

type Config struct {
	Server     string `json:"server"`
	Token      string `json:"token"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

type Tube struct {
	THU         string  `json:"thu"`
	ShippedAt   string  `json:"shipped_at"`
	StoreCode   string  `json:"store_code"`
	OrderNumber string  `json:"order_number"`
	ReturnedAt  *string `json:"returned_at,omitempty"`
}

type ImportItem struct {
	THU         string `json:"thu"`
	ShippedAt   string `json:"shipped_at"`
	StoreCode   string `json:"store_code"`
	OrderNumber string `json:"order_number"`
}

type App struct {
	cfg     Config
	client  *http.Client
	cfgPath string
}

func main() {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = "."
	}
	cfgDir = filepath.Join(cfgDir, "TubeControl")
	_ = os.MkdirAll(cfgDir, 0700)
	a := &App{client: &http.Client{Timeout: 30 * time.Second}, cfgPath: filepath.Join(cfgDir, "device-v2.json")}
	a.cfg.Server = defaultServer
	a.loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.index)
	mux.HandleFunc("GET /local/status", a.status)
	mux.HandleFunc("POST /local/activate", a.activate)
	mux.HandleFunc("GET /local/tubes", a.tubes)
	mux.HandleFunc("POST /local/import", a.importExcel)
	mux.HandleFunc("POST /local/export", a.exportExcel)
	mux.HandleFunc("POST /local/enrollment", a.enrollment)
	mux.HandleFunc("GET /local/devices", a.devices)
	mux.HandleFunc("POST /local/revoke", a.revoke)
	mux.HandleFunc("POST /local/windows-key", a.windowsKey)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String() + "/"
	go func() {
		if err := http.Serve(ln, mux); err != nil {
			log.Println(err)
		}
	}()
	openBrowser(url)
	select {}
}

func (a *App) loadConfig() {
	b, err := os.ReadFile(a.cfgPath)
	if err != nil {
		return
	}
	var c Config
	if json.Unmarshal(b, &c) == nil {
		if c.Server == "" {
			c.Server = defaultServer
		}
		a.cfg = c
	}
}

func (a *App) saveConfig() error {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	return os.WriteFile(a.cfgPath, b, 0600)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, indexHTML)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) requireActivated(w http.ResponseWriter) bool {
	if a.cfg.Token == "" {
		writeJSON(w, 401, map[string]string{"error": "Требуется активация"})
		return false
	}
	return true
}

func (a *App) cloudJSON(method, path string, payload any, token string, out any) (int, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(a.cfg.Server, "/")+path, body)
	if err != nil {
		return 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func codeOr(code, fallback int) int {
	if code > 0 {
		return code
	}
	return fallback
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
