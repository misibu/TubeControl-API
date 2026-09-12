package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func (a *App) importExcel(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Не удалось прочитать файл"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Файл не выбран"})
		return
	}
	defer file.Close()
	f, err := excelize.OpenReader(file)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Не удалось открыть Excel"})
		return
	}
	defer f.Close()
	items, err := parseWorkbook(f)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	processed := 0
	for start := 0; start < len(items); start += 1000 {
		end := start + 1000
		if end > len(items) {
			end = len(items)
		}
		payload := map[string]any{"items": items[start:end]}
		var out map[string]any
		code, e := a.cloudJSON("POST", "/api/v1/tubes/import", payload, a.cfg.Token, &out)
		if e != nil || code != 200 {
			writeJSON(w, codeOr(code, 502), map[string]any{"error": "Ошибка загрузки в облако", "details": errString(e)})
			return
		}
		processed += end - start
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "processed": processed})
}

func parseWorkbook(f *excelize.File) ([]ImportItem, error) {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("в книге нет листов")
	}
	sheet := sheets[0]
	shippedRaw, _ := f.GetCellValue(sheet, "H6")
	shipped, err := parseDate(shippedRaw)
	if err != nil {
		return nil, fmt.Errorf("не удалось определить дату отгрузки в H6: %s", shippedRaw)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	aliases := map[string]string{
		"№ео": "thu", "номерthu": "thu", "thu": "thu",
		"кодтк": "store", "код": "store", "№конечногопунктавыгрузкирцткпоставщик": "store",
		"№заказанапоставкуsap": "order", "№заказанапоставкуизsap": "order", "номерзаказанапоставкуизsap": "order",
	}
	header := -1
	cols := map[string]int{}
	max := 50
	if len(rows) < max {
		max = len(rows)
	}
	for i := 0; i < max; i++ {
		found := map[string]int{}
		for j, v := range rows[i] {
			if key, ok := aliases[norm(v)]; ok {
				found[key] = j
			}
		}
		_, a := found["thu"]
		_, b := found["store"]
		_, c := found["order"]
		if a && b && c {
			header = i
			cols = found
			break
		}
	}
	if header < 0 {
		return nil, fmt.Errorf("не найдены колонки № ЕО, Код ТК и № заказа на поставку SAP")
	}
	items := make([]ImportItem, 0)
	for i := header + 1; i < len(rows); i++ {
		get := func(k string) string {
			idx := cols[k]
			if idx < len(rows[i]) {
				return strings.TrimSpace(rows[i][idx])
			}
			return ""
		}
		thu := strings.ToUpper(get("thu"))
		if thu == "" {
			continue
		}
		items = append(items, ImportItem{THU: thu, ShippedAt: shipped, StoreCode: get("store"), OrderNumber: get("order")})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("в файле не найдено строк с THU")
	}
	return items, nil
}

func parseDate(v string) (string, error) {
	v = strings.TrimSpace(v)
	formats := []string{"2006-01-02", "02.01.2006", "02.01.06", "02/01/2006", "2006-01-02 15:04:05"}
	for _, f := range formats {
		if t, e := time.Parse(f, v); e == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("bad date")
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer(" ", "", "\n", "", "\r", "", ".", "", ",", "", ":", "", "-", "", "_", "", "(", "", ")", "")
	return r.Replace(s)
}

func (a *App) exportExcel(w http.ResponseWriter, r *http.Request) {
	if !a.requireActivated(w) {
		return
	}
	var req struct {
		THUs []string `json:"thus"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	wanted := map[string]bool{}
	for _, v := range req.THUs {
		wanted[v] = true
	}
	var out struct {
		Items []Tube `json:"items"`
	}
	code, err := a.cloudJSON("GET", "/api/v1/tubes?returned=false&limit=5000", nil, a.cfg.Token, &out)
	if err != nil || code != 200 {
		writeJSON(w, 502, map[string]string{"error": "Не удалось выгрузить данные"})
		return
	}
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	headers := []string{"THU", "Дата отгрузки", "Код магазина", "Номер заказа", "Статус"}
	for i, h := range headers {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(s, c, h)
	}
	row := 2
	for _, t := range out.Items {
		if len(wanted) > 0 && !wanted[t.THU] {
			continue
		}
		vals := []any{t.THU, t.ShippedAt, t.StoreCode, t.OrderNumber, "Не возвращён"}
		for i, v := range vals {
			c, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(s, c, v)
		}
		row++
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Ошибка Excel"})
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=TubeControl_non_returned.xlsx")
	_, _ = w.Write(buf.Bytes())
}
