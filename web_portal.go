package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

const webSessionCookie = "tubecontrol_web_session"
const webUserContextKey contextKey = "tubecontrol-web-user"

type WebUser struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Role            string `json:"role"`
	StoreCodeFilter string `json:"store_code_filter,omitempty"`
	Active          bool   `json:"active"`
}

type webLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

type webResetAdminRequest struct {
	RecoveryCode string `json:"recovery_code"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

type webCreateUserRequest struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Password        string `json:"password"`
	StoreCodeFilter string `json:"store_code_filter"`
}

type webUserStateRequest struct {
	Active bool `json:"active"`
}

func registerWebRoutes(mux *http.ServeMux, a *App) {
	mux.HandleFunc("GET /app", a.webPortal)
	mux.Handle("GET /api/web/bootstrap", a.requireDB(http.HandlerFunc(a.webBootstrap)))
	mux.Handle("POST /api/web/setup", a.requireDB(http.HandlerFunc(a.webSetup)))
	mux.Handle("POST /api/web/login", a.requireDB(http.HandlerFunc(a.webLogin)))
	mux.Handle("POST /api/web/admin/reset", a.requireDB(http.HandlerFunc(a.webResetAdmin)))
	mux.Handle("POST /api/web/logout", a.requireDB(http.HandlerFunc(a.webLogout)))

	mux.Handle("GET /api/web/tubes", a.requireDB(a.requireWebAuth(http.HandlerFunc(a.webListTubes))))
	mux.Handle("GET /api/web/export", a.requireDB(a.requireWebAuth(http.HandlerFunc(a.webExport))))
	mux.Handle("POST /api/web/import", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webImportExcel))))
	mux.Handle("POST /api/web/tubes", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.importTubes))))
	mux.Handle("POST /api/web/returns", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.registerReturn))))
	mux.Handle("PATCH /api/web/tubes/{thu}/status", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.updateTubeStatus))))

	mux.Handle("POST /api/web/android/enrollment", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webAndroidEnrollment))))
	mux.Handle("GET /api/web/devices", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webDeviceList))))
	mux.Handle("POST /api/web/devices/{id}/revoke", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webDeviceRevoke))))

	mux.Handle("GET /api/web/users", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webListUsers))))
	mux.Handle("POST /api/web/users", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webCreateClient))))
	mux.Handle("PATCH /api/web/users/{id}", a.requireDB(a.requireWebAdmin(http.HandlerFunc(a.webSetUserState))))
}

func (a *App) ensureWebSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS web_users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin','client')),
    store_code_filter TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_web_users_role_active ON web_users(role, active);
CREATE TABLE IF NOT EXISTS web_sessions (
    token_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES web_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_web_sessions_user ON web_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_web_sessions_expiry ON web_sessions(expires_at);
`)
	return err
}

func (a *App) webPortal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, webPortalHTML)
}

func (a *App) webBootstrap(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureWebSchema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Не удалось подготовить веб-кабинет")
		return
	}
	var count int
	if err := a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM web_users`).Scan(&count); err != nil {
		writeError(w, 500, "database_error", "Не удалось проверить пользователей")
		return
	}
	user, _ := a.webCurrentUser(r)
	writeJSON(w, 200, map[string]any{
		"needs_setup":   count == 0,
		"authenticated": user != nil,
		"user":          user,
	})
}

func normalizeWebUsername(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func validateWebCredentials(username, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return errors.New("Логин должен содержать от 3 до 64 символов")
	}
	if len(password) < 8 {
		return errors.New("Пароль должен содержать минимум 8 символов")
	}
	return nil
}

func (a *App) webSetup(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureWebSchema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Не удалось подготовить веб-кабинет")
		return
	}
	var count int
	if err := a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM web_users`).Scan(&count); err != nil {
		writeError(w, 500, "database_error", "Не удалось проверить пользователей")
		return
	}
	if count != 0 {
		writeError(w, http.StatusForbidden, "already_configured", "Первичная настройка уже выполнена")
		return
	}

	var req webLoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Username = normalizeWebUsername(req.Username)
	if err := validateWebCredentials(req.Username, req.Password); err != nil {
		writeError(w, 400, "invalid_credentials", err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "password_error", "Не удалось сохранить пароль")
		return
	}
	var id int64
	err = a.db.QueryRow(r.Context(), `
INSERT INTO web_users(username,display_name,password_hash,role)
VALUES($1,'Администратор',$2,'admin')
RETURNING id
`, req.Username, string(hash)).Scan(&id)
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось создать администратора")
		return
	}
	user := WebUser{ID: id, Username: req.Username, DisplayName: "Администратор", Role: "admin", Active: true}
	if err := a.webStartSession(w, r, user.ID); err != nil {
		writeError(w, 500, "session_error", "Администратор создан, но не удалось открыть сессию")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "user": user})
}

func (a *App) webResetAdmin(w http.ResponseWriter, r *http.Request) {
	var req webResetAdminRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	resetCode := strings.TrimSpace(os.Getenv("WEB_ADMIN_RESET_CODE"))
	if resetCode == "" {
		writeError(w, http.StatusServiceUnavailable, "reset_not_configured", "Сброс доступа не настроен в Amvera")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.RecoveryCode), []byte(resetCode)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid_recovery_code", "Неверный код восстановления")
		return
	}

	req.Username = normalizeWebUsername(req.Username)
	if err := validateWebCredentials(req.Username, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_credentials", err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "password_error", "Не удалось сохранить пароль")
		return
	}

	var id int64
	if err := a.db.QueryRow(r.Context(), `SELECT COALESCE(MIN(id),0) FROM web_users WHERE role='admin'`).Scan(&id); err != nil {
		writeError(w, 500, "database_error", "Не удалось проверить администратора")
		return
	}
	if id == 0 {
		err = a.db.QueryRow(r.Context(), `
INSERT INTO web_users(username,display_name,password_hash,role,active)
VALUES($1,'Администратор',$2,'admin',TRUE)
RETURNING id
`, req.Username, string(hash)).Scan(&id)
	} else {
		_, err = a.db.Exec(r.Context(), `
UPDATE web_users
SET username=$2, display_name='Администратор', password_hash=$3, active=TRUE, updated_at=NOW()
WHERE id=$1
`, id, req.Username, string(hash))
	}
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось обновить данные администратора")
		return
	}

	_, _ = a.db.Exec(r.Context(), `DELETE FROM web_sessions WHERE user_id=$1`, id)
	user := WebUser{ID: id, Username: req.Username, DisplayName: "Администратор", Role: "admin", Active: true}
	if err := a.webStartSession(w, r, id); err != nil {
		writeError(w, 500, "session_error", "Пароль изменён, но не удалось открыть сессию")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "user": user})
}

func (a *App) webLogin(w http.ResponseWriter, r *http.Request) {
	var req webLoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Username = normalizeWebUsername(req.Username)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "" && req.Role != "admin" && req.Role != "client" {
		writeError(w, http.StatusBadRequest, "invalid_role", "Некорректный тип входа")
		return
	}

	var user WebUser
	var passwordHash string
	err := a.db.QueryRow(r.Context(), `
SELECT id,username,display_name,password_hash,role,store_code_filter,active
FROM web_users WHERE username=$1
`, req.Username).Scan(&user.ID, &user.Username, &user.DisplayName, &passwordHash, &user.Role, &user.StoreCodeFilter, &user.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "unknown_user", "Пользователь с таким логином не найден")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database_error", "Не удалось проверить данные входа")
		return
	}
	if !user.Active {
		writeError(w, http.StatusUnauthorized, "user_disabled", "Учётная запись отключена")
		return
	}
	if req.Role != "" && user.Role != req.Role {
		writeError(w, http.StatusUnauthorized, "wrong_role", "Для этого логина выбран неверный тип входа")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "wrong_password", "Неверный пароль")
		return
	}
	if err := a.webStartSession(w, r, user.ID); err != nil {
		writeError(w, 500, "session_error", "Не удалось открыть сессию")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "user": user})
}

func (a *App) webLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(webSessionCookie); err == nil && c.Value != "" {
		_, _ = a.db.Exec(r.Context(), `DELETE FROM web_sessions WHERE token_hash=$1`, hashSecret(c.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name: webSessionCookie, Value: "", Path: "/",
		MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, Secure: webCookieSecure(r), SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, 200, map[string]any{"status": "ok"})
}

func webCookieSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func randomWebToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "tcw_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *App) webStartSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := randomWebToken()
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(webSessionTTL)
	_, err = a.db.Exec(r.Context(), `
INSERT INTO web_sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)
`, hashSecret(token), userID, expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: webSessionCookie, Value: token, Path: "/",
		Expires: expires, MaxAge: int(webSessionTTL / time.Second),
		HttpOnly: true, Secure: webCookieSecure(r), SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *App) webCurrentUser(r *http.Request) (*WebUser, error) {
	c, err := r.Cookie(webSessionCookie)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return nil, errors.New("no session")
	}
	var user WebUser
	err = a.db.QueryRow(r.Context(), `
SELECT u.id,u.username,u.display_name,u.role,u.store_code_filter,u.active
FROM web_sessions s
JOIN web_users u ON u.id=s.user_id
WHERE s.token_hash=$1 AND s.expires_at>NOW() AND u.active=TRUE
`, hashSecret(c.Value)).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.StoreCodeFilter, &user.Active)
	if err != nil {
		return nil, err
	}
	_, _ = a.db.Exec(r.Context(), `UPDATE web_sessions SET last_seen_at=NOW() WHERE token_hash=$1`, hashSecret(c.Value))
	return &user, nil
}

func (a *App) requireWebAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.webCurrentUser(r)
		if err != nil || user == nil {
			writeError(w, http.StatusUnauthorized, "login_required", "Требуется вход в кабинет")
			return
		}
		ctx := context.WithValue(r.Context(), webUserContextKey, *user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) requireWebAdmin(next http.Handler) http.Handler {
	return a.requireWebAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(webUserContextKey).(WebUser)
		if user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin_required", "Эта команда доступна только администратору")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func requestWebUser(r *http.Request) WebUser {
	user, _ := r.Context().Value(webUserContextKey).(WebUser)
	return user
}

func onlyDigits(v string) string {
	var b strings.Builder
	for _, r := range v {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (a *App) queryWebTubes(ctx context.Context, user WebUser, q, status string) ([]TubeV3, error) {
	if err := a.ensureV3Schema(ctx); err != nil {
		return nil, err
	}
	args := make([]any, 0)
	where := []string{"1=1"}

	if user.Role == "client" && strings.TrimSpace(user.StoreCodeFilter) != "" {
		args = append(args, strings.TrimSpace(user.StoreCodeFilter))
		where = append(where, fmt.Sprintf("LOWER(store_code)=LOWER($%d)", len(args)))
	}

	q = strings.TrimSpace(q)
	if q != "" {
		args = append(args, "%"+q+"%")
		p := len(args)
		search := fmt.Sprintf("(thu ILIKE $%d OR order_number ILIKE $%d", p, p)
		d := onlyDigits(q)
		if len(d) == 4 {
			args = append(args, d)
			search += fmt.Sprintf(" OR RIGHT(REGEXP_REPLACE(order_number,'[^0-9]','','g'),4)=$%d", len(args))
		}
		search += ")"
		where = append(where, search)
	}

	query := `SELECT thu,shipped_at,store_code,order_number,returned_at,manual_status,status_comment,updated_at
FROM tubes WHERE ` + strings.Join(where, " AND ") + ` ORDER BY shipped_at DESC,thu ASC LIMIT 5000`
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TubeV3, 0)
	for rows.Next() {
		var t TubeV3
		var shipped time.Time
		if err := rows.Scan(&t.THU, &shipped, &t.StoreCode, &t.OrderNumber, &t.ReturnedAt, &t.ManualStatus, &t.StatusComment, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.ShippedAt = shipped.Format("2006-01-02")
		t.Status = effectiveTubeStatus(t.ManualStatus, t.ReturnedAt, shipped)
		if status != "" && status != "all" && t.Status != status {
			continue
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (a *App) webListTubes(w http.ResponseWriter, r *http.Request) {
	user := requestWebUser(r)
	items, err := a.queryWebTubes(r.Context(), user, r.URL.Query().Get("q"), strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))))
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось загрузить отправления")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items)})
}

func (a *App) webImportExcel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeError(w, 400, "invalid_upload", "Не удалось прочитать файл")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "file_required", "Выберите Excel-файл")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, 400, "file_read_error", "Не удалось прочитать Excel-файл")
		return
	}

	wb, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		writeError(w, 400, "invalid_excel", "Файл не похож на Excel .xlsx/.xlsm")
		return
	}
	defer wb.Close()

	sheets := wb.GetSheetList()
	if len(sheets) == 0 {
		writeError(w, 400, "empty_excel", "В файле нет листов")
		return
	}
	sheet := sheets[0]

	shippedAt, err := parseWebExcelDateCell(wb, sheet, "H6")
	if err != nil {
		writeError(w, 400, "invalid_date", "Не удалось прочитать дату отгрузки из H6")
		return
	}

	rows, err := wb.GetRows(sheet)
	if err != nil {
		writeError(w, 400, "excel_rows_error", "Не удалось прочитать строки Excel")
		return
	}
	headerRow, thuCol := 0, 0
	maxRows := len(rows)
	if maxRows > 50 {
		maxRows = 50
	}
	for rix := 1; rix <= maxRows; rix++ {
		maxCols := len(rows[rix-1])
		if maxCols > 120 {
			maxCols = 120
		}
		for c := 1; c <= maxCols; c++ {
			cell, _ := excelize.CoordinatesToCellName(c, rix)
			v, _ := wb.GetCellValue(sheet, cell)
			if normWebHeader(v) == normWebHeader("№ ЕО") {
				headerRow, thuCol = rix, c
				break
			}
		}
		if headerRow > 0 {
			break
		}
	}
	if headerRow == 0 {
		writeError(w, 400, "thu_header_missing", "Не найден заголовок «№ ЕО»")
		return
	}

	storeCol, orderCol := 0, 0
	for c := 1; c <= 120; c++ {
		cell, _ := excelize.CoordinatesToCellName(c, headerRow)
		v, _ := wb.GetCellValue(sheet, cell)
		h := normWebHeader(v)
		if h == "КОД ТК" || h == "КОД" || h == "КОД МАГАЗИНА" {
			storeCol = c
		}
		if h == normWebHeader("№ заказа на поставку SAP") {
			orderCol = c
		}
	}
	if storeCol == 0 || orderCol == 0 {
		writeError(w, 400, "columns_missing", "Не найдены столбцы «Код ТК» или «№ заказа на поставку SAP»")
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось начать импорт")
		return
	}
	defer tx.Rollback(r.Context())

	processed := 0
	for rix := headerRow + 1; rix <= len(rows); rix++ {
		thuCell, _ := excelize.CoordinatesToCellName(thuCol, rix)
		storeCell, _ := excelize.CoordinatesToCellName(storeCol, rix)
		orderCell, _ := excelize.CoordinatesToCellName(orderCol, rix)
		thu, _ := wb.GetCellValue(sheet, thuCell)
		thu = normalizeTHU(thu)
		if thu == "" {
			continue
		}
		store, _ := wb.GetCellValue(sheet, storeCell)
		order, _ := wb.GetCellValue(sheet, orderCell)
		_, err = tx.Exec(r.Context(), `
INSERT INTO tubes(thu,shipped_at,store_code,order_number)
VALUES($1,$2,$3,$4)
ON CONFLICT (thu) DO UPDATE SET
 shipped_at=EXCLUDED.shipped_at,
 store_code=EXCLUDED.store_code,
 order_number=EXCLUDED.order_number,
 updated_at=NOW()
`, thu, shippedAt, strings.TrimSpace(store), strings.TrimSpace(order))
		if err != nil {
			writeError(w, 500, "database_error", fmt.Sprintf("Ошибка импорта строки %d", rix))
			return
		}
		processed++
	}
	if processed == 0 {
		writeError(w, 400, "no_tubes", "В файле не найдено THU для импорта")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Не удалось завершить импорт")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "processed": processed})
}

func normWebHeader(v string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.ReplaceAll(v, "\u00A0", " ")), " "))
}

func parseWebExcelDateCell(wb *excelize.File, sheet, cell string) (time.Time, error) {
	values := make([]string, 0, 3)
	if v, err := wb.GetCellValue(sheet, cell); err == nil {
		values = append(values, v)
	}
	if v, err := wb.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true}); err == nil {
		values = append(values, v)
	}
	if formula, err := wb.GetCellFormula(sheet, cell); err == nil && strings.TrimSpace(formula) != "" {
		if v, err := wb.CalcCellValue(sheet, cell); err == nil {
			values = append(values, v)
		}
	}

	seen := make(map[string]bool)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		if t, err := parseWebExcelDate(v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func parseWebExcelDate(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, " г.", ""), "г.", ""))
	layouts := []string{
		"02.01.2006", "2.1.2006", "02.01.06", "2.1.06",
		"02.01.2006 15:04", "2.1.2006 15:04", "02.01.2006 15:04:05", "2.1.2006 15:04:05",
		"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05",
		"02/01/2006", "2/1/2006", "02/01/2006 15:04", "2/1/2006 15:04",
		time.RFC3339, time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	if n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
		if t, err := excelize.ExcelDateToTime(n, false); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func (a *App) webExport(w http.ResponseWriter, r *http.Request) {
	user := requestWebUser(r)
	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if category != "" && category != "current" {
		status = "all"
	}
	items, err := a.queryWebTubes(r.Context(), user, r.URL.Query().Get("q"), status)
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось подготовить выгрузку")
		return
	}

	filtered := make([]TubeV3, 0, len(items))
	sheet := "Выгрузка"
	suffix := "vygruzka"
	for _, t := range items {
		include := true
		switch category {
		case "", "not_returned":
			include = t.Status != "returned"
			sheet = "Не возвращены"
			suffix = "ne_vozvrasheny"
		case "current":
			sheet = "Текущая выборка"
			suffix = "tekushaya_vyborka"
		case "all":
			sheet = "Все доступные"
			suffix = "vse_dostupnye"
		case "overdue":
			include = t.Status == "overdue"
			sheet = "Просроченные"
			suffix = "prosrochennye"
		case "returned":
			include = t.Status == "returned"
			sheet = "Возвращённые"
			suffix = "vozvrashennye"
		case "transit":
			include = t.Status == "transit"
			sheet = "В пути"
			suffix = "v_puti"
		case "sent":
			include = t.Status == "sent"
			sheet = "Отправленные"
			suffix = "otpravlennye"
		case "lost":
			include = t.Status == "lost"
			sheet = "Потерянные"
			suffix = "poteryannye"
		default:
			writeError(w, 400, "invalid_category", "Неизвестная категория выгрузки")
			return
		}
		if include {
			filtered = append(filtered, t)
		}
	}

	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", sheet)
	headers := []string{"THU", "Дата отправки", "Магазин", "№ заказа", "Статус"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for i, t := range filtered {
		row := i + 2
		values := []any{t.THU, t.ShippedAt, t.StoreCode, t.OrderNumber, webStatusRu(t.Status)}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	_ = f.SetCellStyle(sheet, "A1", "E1", style)
	_ = f.SetColWidth(sheet, "A", "A", 24)
	_ = f.SetColWidth(sheet, "B", "B", 16)
	_ = f.SetColWidth(sheet, "C", "C", 18)
	_ = f.SetColWidth(sheet, "D", "D", 28)
	_ = f.SetColWidth(sheet, "E", "E", 18)
	buf, err := f.WriteToBuffer()
	if err != nil {
		writeError(w, 500, "export_error", "Не удалось сформировать Excel")
		return
	}
	filename := "TubeControl_" + suffix + "_" + time.Now().Format("20060102_1504") + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(200)
	_, _ = w.Write(buf.Bytes())
}

func webStatusRu(v string) string {
	switch v {
	case "sent":
		return "Отправлен"
	case "transit":
		return "В пути"
	case "returned":
		return "Возвращён"
	case "lost":
		return "Потерян"
	case "overdue":
		return "Просрочен"
	default:
		return v
	}
}

func (a *App) webListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `
SELECT id,username,display_name,role,store_code_filter,active
FROM web_users ORDER BY role ASC,display_name ASC,username ASC
`)
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось загрузить пользователей")
		return
	}
	defer rows.Close()
	items := make([]WebUser, 0)
	for rows.Next() {
		var u WebUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.StoreCodeFilter, &u.Active); err != nil {
			writeError(w, 500, "database_error", "Не удалось прочитать пользователя")
			return
		}
		items = append(items, u)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) webCreateClient(w http.ResponseWriter, r *http.Request) {
	var req webCreateUserRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Username = normalizeWebUsername(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.StoreCodeFilter = strings.TrimSpace(req.StoreCodeFilter)
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	if err := validateWebCredentials(req.Username, req.Password); err != nil {
		writeError(w, 400, "invalid_user", err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "password_error", "Не удалось сохранить пароль")
		return
	}
	var id int64
	err = a.db.QueryRow(r.Context(), `
INSERT INTO web_users(username,display_name,password_hash,role,store_code_filter)
VALUES($1,$2,$3,'client',$4) RETURNING id
`, req.Username, req.DisplayName, string(hash), req.StoreCodeFilter).Scan(&id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, 409, "username_exists", "Такой логин уже существует")
			return
		}
		writeError(w, 500, "database_error", "Не удалось создать клиента")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "id": id})
}

func (a *App) webSetUserState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid_user", "Некорректный пользователь")
		return
	}
	current := requestWebUser(r)
	if id == current.ID {
		writeError(w, 400, "cannot_disable_self", "Нельзя отключить текущего администратора")
		return
	}
	var req webUserStateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE web_users SET active=$2,updated_at=NOW() WHERE id=$1 AND role='client'`, id, req.Active)
	if err != nil {
		writeError(w, 500, "database_error", "Не удалось изменить доступ")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "user_not_found", "Клиент не найден")
		return
	}
	if !req.Active {
		_, _ = a.db.Exec(r.Context(), `DELETE FROM web_sessions WHERE user_id=$1`, id)
	}
	writeJSON(w, 200, map[string]any{"status": "ok"})
}

const webPortalHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TubeControl — веб-кабинет</title>
<meta name="theme-color" content="#06100d">
<meta name="description" content="TubeControl — веб-кабинет учета и возврата тубусов">
<style>
/* TubeControl web portal */
:root{--bg:#06100d;--bg2:#0a1713;--panel:#10201b;--panel2:#132823;--mint:#75ffd0;--emerald:#19d99a;--line:#58e9bb;--text:#f6fffb;--muted:#9eb7ae;--danger:#ff6b72;--yellow:#ffd43b;--blue:#67b7ff;--orange:#ff9a55}
*{box-sizing:border-box}html,body{margin:0;min-height:100%;font-family:"Segoe UI",Arial,sans-serif;background:radial-gradient(circle at 20% 0,#0d2a20 0,#06100d 34%,#040a08 100%);color:var(--text)}button,input,select,textarea{font:inherit}.hidden{display:none!important}
.wrap{width:100%;max-width:1480px;margin:auto;padding-left:24px;padding-right:24px}.top{height:92px;border-bottom:1px solid rgba(117,255,208,.14);display:flex;align-items:center}.toprow{display:flex;align-items:center;justify-content:space-between;gap:24px;width:100%}.logo{font-size:38px;font-weight:800;letter-spacing:-1.4px}.logo span{color:var(--mint)}.tag{margin-top:5px;color:var(--mint);font-size:11px;font-weight:800;letter-spacing:.13em}
.userbox{display:flex;align-items:center;gap:12px;color:var(--muted)}.role{border:1px solid rgba(117,255,208,.4);border-radius:999px;padding:7px 11px;color:var(--mint);font-size:12px;font-weight:800}
.btn{height:42px;border:1px solid var(--line);border-radius:21px;background:transparent;color:var(--text);padding:0 18px;font-weight:750;cursor:pointer;transition:.15s}.btn:hover{background:rgba(25,217,154,.10)}.btn.primary{background:linear-gradient(180deg,#2ee6b0,#15cf96);color:#032018;border-color:#90ffdf}.btn.danger{border-color:rgba(255,107,114,.6);color:#ff9da2}.btn:disabled{opacity:.4;cursor:not-allowed}
.auth{min-height:100vh;display:grid;place-items:center;padding:30px}.authcard{width:min(720px,94vw);border:1px solid rgba(117,255,208,.45);border-radius:30px;background:linear-gradient(180deg,rgba(17,42,34,.94),rgba(7,18,14,.96));padding:34px;box-shadow:0 30px 90px rgba(0,0,0,.35)}.authcard h1{margin:20px 0 8px;font-size:28px}.authcard p{color:var(--muted);margin:0 0 24px}.roleintro{text-align:center}.roleintro .tag{margin-bottom:28px}.rolegrid{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:24px}.rolecard{min-height:150px;border:1px solid rgba(117,255,208,.30);border-radius:24px;background:rgba(7,19,15,.72);color:var(--text);padding:28px 24px;text-align:left;cursor:pointer;transition:.18s ease}.rolecard:hover{transform:translateY(-2px);border-color:var(--mint);background:rgba(19,51,40,.78);box-shadow:0 18px 46px rgba(0,0,0,.22)}.rolecard strong{display:block;font-size:21px;margin-bottom:10px}.rolecard span{display:block;color:var(--muted);font-size:13px;line-height:1.5}.authform{width:min(480px,100%);margin:0 auto}.backrole{display:inline-flex;align-items:center;gap:7px;margin-top:4px;border:0;background:transparent;color:var(--mint);padding:0;cursor:pointer;font-weight:750}.rememberrow{display:flex;align-items:center;gap:10px;margin:16px 0 2px;color:var(--muted);font-size:14px;cursor:pointer;user-select:none}.rememberrow input{width:18px;height:18px;margin:0;accent-color:var(--emerald);cursor:pointer}.field{display:flex;flex-direction:column;gap:7px;margin:13px 0}.field label{font-size:13px;color:var(--muted);font-weight:700}.input,.select,.textarea{width:100%;border:1px solid rgba(117,255,208,.35);border-radius:18px;background:#07130f;color:var(--text);outline:none;padding:0 15px}.input,.select{height:44px}.textarea{padding-top:12px;min-height:92px;resize:vertical}.input:focus,.select:focus,.textarea:focus{border-color:var(--mint);box-shadow:0 0 0 3px rgba(117,255,208,.08)}
.main{padding:28px 0 46px}.toolbar{display:flex;gap:10px;flex-wrap:wrap;margin-bottom:16px}.searchrow{display:grid;grid-template-columns:minmax(320px,680px) 210px 1fr;gap:12px;margin:12px 0 18px}.searchbox{position:relative}.searchbox:before{content:"⌕";position:absolute;left:16px;top:9px;color:var(--mint);font-size:22px}.searchbox .input{padding-left:46px;border-color:var(--line)}
.card{border:1px solid rgba(117,255,208,.28);border-radius:24px;background:linear-gradient(180deg,rgba(16,32,27,.92),rgba(8,18,14,.92));overflow:hidden}.tablewrap{overflow:auto;max-height:calc(100vh - 300px)}table{width:100%;border-collapse:collapse;min-width:980px}thead{position:sticky;top:0;z-index:2;background:#0b1d17}th{font-size:12px;color:#a9c1b8;text-align:left;padding:15px 14px;border-bottom:1px solid rgba(117,255,208,.18)}td{padding:14px;border-bottom:1px solid rgba(117,255,208,.10);font-size:14px}tbody tr{cursor:pointer;transition:.12s}tbody tr:hover{background:rgba(117,255,208,.045)}tbody tr.selected{background:rgba(25,217,154,.12);box-shadow:inset 3px 0 0 var(--mint)}.mark{width:18px;height:18px;border:1px solid #5f897a;border-radius:5px;display:grid;place-items:center;color:#031a11}.selected .mark{background:var(--mint);border-color:var(--mint)}.status{display:inline-flex;align-items:center;gap:8px;border:1px solid currentColor;border-radius:999px;padding:6px 11px;font-weight:750;font-size:12px}.status:before{content:"";width:7px;height:7px;border-radius:50%;background:currentColor}.s-sent{color:var(--blue)}.s-transit{color:var(--yellow)}.s-returned{color:var(--mint)}.s-lost{color:var(--danger)}.s-overdue{color:var(--orange)}
.footerline{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:14px 18px;color:var(--muted);font-size:13px}.copyhint{color:var(--mint)}
.modalback{position:fixed;inset:0;background:rgba(0,0,0,.66);display:grid;place-items:center;padding:18px;z-index:20}.modal{width:min(560px,96vw);max-height:90vh;overflow:auto;border:1px solid rgba(117,255,208,.5);border-radius:28px;background:#0a1813;padding:28px;box-shadow:0 28px 90px rgba(0,0,0,.55)}.modal h2{margin:0 0 18px}.modalactions{display:flex;gap:10px;justify-content:flex-end;margin-top:20px}.clients{margin-top:18px;display:grid;gap:8px}.clientrow{display:grid;grid-template-columns:1fr auto;gap:12px;align-items:center;border:1px solid rgba(117,255,208,.18);border-radius:17px;padding:12px}.clientmeta{color:var(--muted);font-size:12px;margin-top:4px}.toast{position:fixed;right:22px;bottom:22px;z-index:50;max-width:420px;padding:15px 18px;border:1px solid var(--line);border-radius:18px;background:#10201b;box-shadow:0 18px 50px #000}.toast.err{border-color:var(--danger)}.empty{padding:46px;text-align:center;color:var(--muted)}
@media(max-width:850px){.wrap{padding-left:18px;padding-right:18px}.top{height:auto;padding:18px 0}.toprow{align-items:flex-start}.userbox{flex-wrap:wrap;justify-content:flex-end}.searchrow{grid-template-columns:1fr}.toolbar .btn{flex:1;min-width:140px}.main{padding-top:18px}.tablewrap{max-height:none}.logo{font-size:32px}}@media(max-width:620px){.wrap{padding-left:16px;padding-right:16px}.auth{padding:16px}.authcard{padding:24px 20px}.rolegrid{grid-template-columns:1fr}.rolecard{min-height:125px}.roleintro .logo{font-size:36px}}
</style>
</head>
<body>
<section id="auth" class="auth">
  <div class="authcard">
    <div id="roleBox" class="roleintro">
      <div class="logo">Tube<span>Control</span></div><div class="tag">УЧЕТ И КОНТРОЛЬ</div>
      <h1>Выберите режим входа</h1>
      <p>Укажите, как вы хотите войти в веб-версию TubeControl.</p>
      <div class="rolegrid">
        <button class="rolecard" onclick="chooseRole('admin')">
          <strong>Администратор</strong>
          <span>Полный доступ: импорт, возвраты, статусы, клиенты и подключение устройств.</span>
        </button>
        <button class="rolecard" onclick="chooseRole('client')">
          <strong>Клиент</strong>
          <span>Просмотр доступных отправлений, поиск и копирование информации.</span>
        </button>
      </div>
    </div>
    <div id="setupBox" class="authform hidden">
      <div class="logo">Tube<span>Control</span></div><div class="tag">АДМИНИСТРАТОР</div>
      <button class="backrole" onclick="showRoleChoice()">← Назад к выбору</button>
      <h1>Создание администратора</h1>
      <p>Первый вход. Создайте логин и пароль администратора веб-кабинета.</p>
      <div class="field"><label>Логин</label><input id="setupLogin" class="input" autocomplete="username"></div>
      <div class="field"><label>Пароль</label><input id="setupPassword" type="password" class="input" autocomplete="new-password"></div>
      <button class="btn primary" style="width:100%;margin-top:10px" onclick="setupAdmin()">СОЗДАТЬ АДМИНИСТРАТОРА</button>
    </div>
    <div id="loginBox" class="authform hidden">
      <div class="logo">Tube<span>Control</span></div><div id="loginRoleTag" class="tag"></div>
      <button class="backrole" onclick="showRoleChoice()">← Назад к выбору</button>
      <h1 id="loginTitle">Вход в кабинет</h1>
      <p id="loginText"></p>
      <div class="field"><label>Логин</label><input id="login" class="input" autocomplete="username"></div>
      <div class="field"><label>Пароль</label><input id="password" type="password" class="input" autocomplete="current-password" onkeydown="if(event.key==='Enter')loginUser()"></div>
      <label class="rememberrow"><input id="rememberMe" type="checkbox" onchange="rememberChanged()"> <span>Запомнить меня</span></label>
      <button class="btn primary" style="width:100%;margin-top:10px" onclick="loginUser()">ВОЙТИ</button>
      <button id="resetAccessBtn" class="backrole hidden" style="margin-top:16px" onclick="showAdminReset()">Сбросить доступ администратора</button>
    </div>
    <div id="resetBox" class="authform hidden">
      <div class="logo">Tube<span>Control</span></div><div class="tag">ВОССТАНОВЛЕНИЕ ДОСТУПА</div>
      <button class="backrole" onclick="chooseRole('admin')">← Назад ко входу</button>
      <h1>Новый логин и пароль</h1>
      <p>Введите код восстановления из переменной WEB_ADMIN_RESET_CODE в Amvera. Данные отправлений не удаляются.</p>
      <div class="field"><label>Код восстановления</label><input id="resetCode" type="password" class="input" autocomplete="off"></div>
      <div class="field"><label>Новый логин</label><input id="resetLogin" class="input" autocomplete="username"></div>
      <div class="field"><label>Новый пароль</label><input id="resetPassword" type="password" class="input" autocomplete="new-password" onkeydown="if(event.key==='Enter')resetAdminAccess()"></div>
      <button class="btn primary" style="width:100%;margin-top:10px" onclick="resetAdminAccess()">СОХРАНИТЬ И ВОЙТИ</button>
    </div>
  </div>
</section>

<section id="app" class="hidden">
<header class="top"><div class="wrap toprow"><div><div class="logo">Tube<span>Control</span></div><div class="tag">УЧЕТ И КОНТРОЛЬ</div></div><div class="userbox"><span id="displayName"></span><span id="role" class="role"></span><button class="btn" onclick="logoutUser()">Выйти</button></div></div></header>
<main class="wrap main">
  <div id="adminToolbar" class="toolbar hidden">
    <button class="btn" onclick="pickImport()">Импорт Excel</button>
    <button class="btn" onclick="openModal('addModal')">Добавить THU</button>
    <button class="btn" onclick="openModal('returnModal')">Возврат</button>
    <button id="editBtn" class="btn" onclick="openEdit()" disabled>Редактировать</button>
    <button class="btn" onclick="exportExcel()">Выгрузить</button>
    <button class="btn" onclick="openAndroidPair()">Подключить Android</button><button class="btn" onclick="openDevices()">Устройства</button><button class="btn" onclick="openClients()">Клиенты</button>
    <input id="importFile" type="file" accept=".xlsx,.xlsm" class="hidden" onchange="importExcel(this)">
  </div>
  <div class="searchrow">
    <div class="searchbox"><input id="search" class="input" placeholder="Поиск: THU, № заказа или последние 4 цифры заказа" oninput="debouncedLoad()"></div>
    <select id="statusFilter" class="select" onchange="loadTubes()"><option value="all">Все статусы</option><option value="sent">Отправлен</option><option value="transit">В пути</option><option value="returned">Возвращён</option><option value="lost">Потерян</option><option value="overdue">Просрочен</option></select>
    <div style="display:flex;gap:10px;justify-content:flex-end;flex-wrap:wrap"><button id="copyBtn" class="btn" onclick="copySelected()" disabled>Копировать данные</button><button class="btn" onclick="openModal('exportModal')">Выгрузка</button><button class="btn" onclick="loadTubes()">↻</button></div>
  </div>
  <div class="card">
    <div class="tablewrap">
      <table><thead><tr><th style="width:46px"></th><th>THU</th><th>№ заказа</th><th>Магазин</th><th>Дата отправки</th><th>Статус</th></tr></thead><tbody id="tbody"></tbody></table>
      <div id="empty" class="empty hidden">Ничего не найдено</div>
    </div>
    <div class="footerline"><span id="count">0 записей</span><span id="copyHint" class="copyhint">Выберите строку для копирования</span></div>
  </div>
</main>
</section>

<div id="addModal" class="modalback hidden"><div class="modal"><h2>Добавить THU</h2><div class="field"><label>THU</label><input id="addThu" class="input"></div><div class="field"><label>Код магазина</label><input id="addStore" class="input"></div><div class="field"><label>№ заказа</label><input id="addOrder" class="input"></div><div class="field"><label>Дата отправки</label><input id="addDate" type="date" class="input"></div><div class="modalactions"><button class="btn" onclick="closeModal('addModal')">Отмена</button><button class="btn primary" onclick="addTube()">Добавить</button></div></div></div>
<div id="returnModal" class="modalback hidden"><div class="modal"><h2>Зарегистрировать возврат</h2><div class="field"><label>THU</label><input id="returnThu" class="input"></div><div class="modalactions"><button class="btn" onclick="closeModal('returnModal')">Отмена</button><button class="btn primary" onclick="returnTube()">Возврат</button></div></div></div>
<div id="editModal" class="modalback hidden"><div class="modal"><h2 id="editTitle">Изменить статус</h2><div class="field"><label>Статус</label><select id="editStatus" class="select"><option value="auto">Автоматически</option><option value="sent">Отправлен</option><option value="transit">В пути</option><option value="returned">Возвращён</option><option value="lost">Потерян</option></select></div><div class="field"><label>Комментарий</label><textarea id="editComment" class="textarea" maxlength="500"></textarea></div><div class="modalactions"><button class="btn" onclick="closeModal('editModal')">Отмена</button><button class="btn primary" onclick="saveStatus()">Сохранить</button></div></div></div>
<div id="clientsModal" class="modalback hidden"><div class="modal"><h2>Клиенты</h2><div class="field"><label>Название клиента</label><input id="clientName" class="input"></div><div class="field"><label>Логин</label><input id="clientLogin" class="input"></div><div class="field"><label>Пароль (минимум 8 символов)</label><input id="clientPassword" type="password" class="input"></div><div class="field"><label>Ограничить кодом магазина (необязательно)</label><input id="clientStore" class="input" placeholder="Например: 0042"></div><button class="btn primary" onclick="createClient()">Создать клиента</button><div id="clientsList" class="clients"></div><div class="modalactions"><button class="btn" onclick="closeModal('clientsModal')">Закрыть</button></div></div></div>
<div id="androidModal" class="modalback hidden"><div class="modal"><h2>Подключить Android</h2><p style="color:var(--muted);margin-top:-6px">Откройте TubeControl на телефоне и отсканируйте этот QR-код.</p><div style="display:grid;place-items:center;margin:18px 0"><img id="androidQr" alt="QR подключения" style="width:320px;max-width:90%;border-radius:20px;background:white;padding:12px"></div><div id="androidCode" style="text-align:center;color:var(--mint);font-weight:800"></div><div class="modalactions"><button class="btn" onclick="closeModal('androidModal')">Закрыть</button><button class="btn primary" onclick="makeAndroidQr()">Новый QR</button></div></div></div>
<div id="devicesModal" class="modalback hidden"><div class="modal"><h2>Android устройства</h2><div id="devicesList" class="clients"></div><div class="modalactions"><button class="btn" onclick="closeModal('devicesModal')">Закрыть</button></div></div></div>
<div id="exportModal" class="modalback hidden"><div class="modal"><h2>Выгрузка Excel</h2><p style="color:var(--muted);margin-top:-6px">Выберите категорию. Для клиента выгружаются только доступные ему отправления.</p><div class="clients"><button class="btn" onclick="exportCategory('current')">Текущая выборка</button><button class="btn" onclick="exportCategory('all')">Все доступные</button><button class="btn" onclick="exportCategory('not_returned')">Не возвращены</button><button class="btn" onclick="exportCategory('overdue')">Просроченные</button><button class="btn" onclick="exportCategory('returned')">Возвращённые</button><button class="btn" onclick="exportCategory('transit')">В пути</button><button class="btn" onclick="exportCategory('sent')">Отправленные</button><button class="btn" onclick="exportCategory('lost')">Потерянные</button></div><div class="modalactions"><button class="btn" onclick="closeModal('exportModal')">Закрыть</button></div></div></div>
<div id="toast" class="toast hidden"></div>

<script>
let me=null, items=[], selected=null, timer=null, selectedRole='', needsSetup=false;
const $=id=>document.getElementById(id);
function esc(v){return String(v??'').replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
async function api(url,opt){const r=await fetch(url,opt);let x={};const ct=r.headers.get('content-type')||'';if(ct.includes('application/json'))x=await r.json();if(!r.ok)throw new Error(x.message||x.error||('HTTP '+r.status));return x}
function toast(t,err){const x=$('toast');x.textContent=t;x.className='toast'+(err?' err':'');setTimeout(()=>x.className='toast hidden',4200)}
const rememberKey='tubecontrol_web_remember';function readRemembered(){try{const x=JSON.parse(localStorage.getItem(rememberKey)||'null');if(x&&(x.role==='admin'||x.role==='client'))return x}catch(e){}return null}function saveRemembered(){if(!$('rememberMe').checked){localStorage.removeItem(rememberKey);return}localStorage.setItem(rememberKey,JSON.stringify({role:selectedRole,username:$('login').value.trim()}))}function rememberChanged(){if(!$('rememberMe').checked)localStorage.removeItem(rememberKey)}function showRoleChoice(){selectedRole='';$('roleBox').classList.remove('hidden');$('setupBox').classList.add('hidden');$('loginBox').classList.add('hidden');$('resetBox').classList.add('hidden');$('password').value=''}function chooseRole(role){selectedRole=role;if(role==='client'&&needsSetup){toast('Сначала необходимо создать администратора',true);return}$('roleBox').classList.add('hidden');$('resetBox').classList.add('hidden');if(role==='admin'&&needsSetup){$('setupBox').classList.remove('hidden');$('loginBox').classList.add('hidden');setTimeout(()=>$('setupLogin').focus(),0);return}$('setupBox').classList.add('hidden');$('loginBox').classList.remove('hidden');$('resetAccessBtn').classList.toggle('hidden',role!=='admin');$('loginRoleTag').textContent=role==='admin'?'АДМИНИСТРАТОР':'КЛИЕНТ';$('loginTitle').textContent=role==='admin'?'Вход администратора':'Вход клиента';$('loginText').textContent=role==='admin'?'Полный доступ к управлению TubeControl.':'Просмотр доступных отправлений, поиск и копирование данных.';setTimeout(()=>$('login').focus(),0)}function showAdminReset(){selectedRole='admin';$('roleBox').classList.add('hidden');$('setupBox').classList.add('hidden');$('loginBox').classList.add('hidden');$('resetBox').classList.remove('hidden');$('resetLogin').value=$('login').value.trim();setTimeout(()=>$('resetCode').focus(),0)}async function boot(){try{const x=await api('/api/web/bootstrap');needsSetup=!!x.needs_setup;if(x.authenticated){enterApp(x.user);return}const saved=readRemembered();if(saved&&!needsSetup){$('rememberMe').checked=true;$('login').value=saved.username||'';chooseRole(saved.role);return}showRoleChoice()}catch(e){toast(e.message,true)}}
async function setupAdmin(){try{const x=await api('/api/web/setup',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:$('setupLogin').value,password:$('setupPassword').value})});enterApp(x.user)}catch(e){toast(e.message,true)}}
async function resetAdminAccess(){try{const x=await api('/api/web/admin/reset',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({recovery_code:$('resetCode').value,username:$('resetLogin').value,password:$('resetPassword').value})});selectedRole='admin';$('login').value=$('resetLogin').value.trim();saveRemembered();$('password').value='';$('resetCode').value='';$('resetPassword').value='';toast('Доступ администратора восстановлен');enterApp(x.user)}catch(e){toast(e.message,true)}}
async function loginUser(){if(!selectedRole){showRoleChoice();return}try{const x=await api('/api/web/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:$('login').value,password:$('password').value,role:selectedRole})});saveRemembered();enterApp(x.user)}catch(e){toast(e.message,true)}}
async function logoutUser(){try{await api('/api/web/logout',{method:'POST'})}finally{location.reload()}}
function enterApp(u){me=u;$('auth').classList.add('hidden');$('app').classList.remove('hidden');$('displayName').textContent=u.display_name||u.username;$('role').textContent=u.role==='admin'?'АДМИНИСТРАТОР':'КЛИЕНТ';if(u.role==='admin')$('adminToolbar').classList.remove('hidden');loadTubes()}
function debouncedLoad(){clearTimeout(timer);timer=setTimeout(loadTubes,220)}
async function loadTubes(){try{const q=encodeURIComponent($('search').value.trim());const s=encodeURIComponent($('statusFilter').value);const x=await api('/api/web/tubes?q='+q+'&status='+s);items=x.items||[];if(selected&&!items.some(i=>i.thu===selected.thu))selected=null;render()}catch(e){if(e.message.includes('вход'))location.reload();else toast(e.message,true)}}
function statusRu(s){return {sent:'Отправлен',transit:'В пути',returned:'Возвращён',lost:'Потерян',overdue:'Просрочен'}[s]||s}
function render(){const b=$('tbody');b.innerHTML='';for(const t of items){const tr=document.createElement('tr');if(selected&&selected.thu===t.thu)tr.classList.add('selected');tr.onclick=function(){selected=(selected&&selected.thu===t.thu)?null:t;render()};tr.innerHTML='<td><span class="mark">'+((selected&&selected.thu===t.thu)?'✓':'')+'</span></td><td><strong>'+esc(t.thu)+'</strong></td><td>'+esc(t.order_number||'—')+'</td><td>'+esc(t.store_code||'—')+'</td><td>'+esc(t.shipped_at||'—')+'</td><td><span class="status s-'+esc(t.status)+'">'+esc(statusRu(t.status))+'</span></td>';b.appendChild(tr)}$('empty').classList.toggle('hidden',items.length!==0);$('count').textContent='Показано: '+items.length;$('copyBtn').disabled=!selected;if($('editBtn'))$('editBtn').disabled=!selected;$('copyHint').textContent=selected?'Выбрано: '+selected.thu:'Выберите строку для копирования'}
async function copySelected(){if(!selected)return;const text='THU: '+selected.thu+'\n№ заказа: '+(selected.order_number||'—')+'\nМагазин: '+(selected.store_code||'—')+'\nДата отправки: '+(selected.shipped_at||'—')+'\nСтатус: '+statusRu(selected.status);try{await navigator.clipboard.writeText(text);toast('Данные скопированы')}catch(e){toast('Не удалось скопировать',true)}}
function openModal(id){$(id).classList.remove('hidden')}function closeModal(id){$(id).classList.add('hidden')}
function today(){return new Date().toISOString().slice(0,10)}$('addDate').value=today();
async function addTube(){try{const item={thu:$('addThu').value.trim(),store_code:$('addStore').value.trim(),order_number:$('addOrder').value.trim(),shipped_at:$('addDate').value};await api('/api/web/tubes',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({items:[item]})});closeModal('addModal');toast('THU добавлен');loadTubes()}catch(e){toast(e.message,true)}}
async function returnTube(){try{await api('/api/web/returns',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({thu:$('returnThu').value.trim()})});closeModal('returnModal');toast('Возврат зарегистрирован');loadTubes()}catch(e){toast(e.message,true)}}
function openEdit(){if(!selected)return;$('editTitle').textContent='Изменить статус · '+selected.thu;$('editStatus').value=selected.manual_status||'auto';$('editComment').value=selected.status_comment||'';openModal('editModal')}
async function saveStatus(){if(!selected)return;try{await api('/api/web/tubes/'+encodeURIComponent(selected.thu)+'/status',{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({status:$('editStatus').value,comment:$('editComment').value})});closeModal('editModal');toast('Статус сохранён');loadTubes()}catch(e){toast(e.message,true)}}
function pickImport(){$('importFile').value='';$('importFile').click()}
async function importExcel(el){if(!el.files||!el.files[0])return;const fd=new FormData();fd.append('file',el.files[0]);try{const x=await api('/api/web/import',{method:'POST',body:fd});toast('Импортировано: '+x.processed);loadTubes()}catch(e){toast(e.message,true)}}
function exportExcel(){exportCategory('not_returned')}function exportCategory(category){const q=encodeURIComponent($('search').value.trim());const s=encodeURIComponent($('statusFilter').value);closeModal('exportModal');location.href='/api/web/export?q='+q+'&status='+s+'&category='+encodeURIComponent(category)}
async function openAndroidPair(){openModal('androidModal');await makeAndroidQr()}
async function makeAndroidQr(){try{const x=await api('/api/web/android/enrollment',{method:'POST'});$('androidQr').src=x.qr_png;$('androidCode').textContent='Код действует 10 минут · '+x.code}catch(e){toast(e.message,true)}}
async function openDevices(){openModal('devicesModal');await loadDevices()}
async function loadDevices(){try{const x=await api('/api/web/devices');const box=$('devicesList');box.innerHTML='';const dev=(x.items||[]).filter(d=>d.kind==='android');if(!dev.length){box.innerHTML='<div class="empty">Android-устройства ещё не подключены</div>';return}for(const d of dev){const row=document.createElement('div');row.className='clientrow';const active=!d.revoked_at;row.innerHTML='<div><strong>'+esc(d.name||'Android')+'</strong><div class="clientmeta">'+(active?'Подключено':'Отключено')+(d.last_seen_at?' · Последняя активность: '+esc(new Date(d.last_seen_at).toLocaleString('ru-RU')):'')+'</div></div>'+(active?'<button class="btn danger">Отключить</button>':'');if(active)row.querySelector('button').onclick=function(){revokeDevice(d.id)};box.appendChild(row)}}catch(e){toast(e.message,true)}}
async function revokeDevice(id){try{await api('/api/web/devices/'+encodeURIComponent(id)+'/revoke',{method:'POST'});toast('Устройство отключено');loadDevices()}catch(e){toast(e.message,true)}}
async function openClients(){openModal('clientsModal');await loadClients()}
async function loadClients(){try{const x=await api('/api/web/users');const box=$('clientsList');box.innerHTML='';for(const u of x.items||[]){if(u.role!=='client')continue;const row=document.createElement('div');row.className='clientrow';row.innerHTML='<div><strong>'+esc(u.display_name||u.username)+'</strong><div class="clientmeta">Логин: '+esc(u.username)+(u.store_code_filter?' · Магазин: '+esc(u.store_code_filter):' · Все отправления')+'</div></div><button class="btn '+(u.active?'danger':'')+'">'+(u.active?'Отключить':'Включить')+'</button>';row.querySelector('button').onclick=function(){setClientState(u.id,!u.active)};box.appendChild(row)}}catch(e){toast(e.message,true)}}
async function createClient(){try{await api('/api/web/users',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({display_name:$('clientName').value,username:$('clientLogin').value,password:$('clientPassword').value,store_code_filter:$('clientStore').value})});$('clientPassword').value='';toast('Клиент создан');loadClients()}catch(e){toast(e.message,true)}}
async function setClientState(id,active){try{await api('/api/web/users/'+id,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({active:active})});loadClients()}catch(e){toast(e.message,true)}}
document.addEventListener('keydown',function(e){if(e.key==='Escape'){for(const id of ['addModal','returnModal','editModal','clientsModal','androidModal','devicesModal','exportModal'])closeModal(id)}});boot();
</script>
</body></html>`
