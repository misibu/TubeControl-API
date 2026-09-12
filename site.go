package main

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const windowsDownloadFallback = "https://github.com/misibu/TubeControl-API/releases/download/v3-latest/TubeControl-Windows-v3.exe"
const androidDownloadFallback = "https://github.com/misibu/TubeControl-API/releases/download/v3-latest/TubeControl-Scanner-v3.apk"

func registerSiteRoutes(mux *http.ServeMux, a *App) {
	mux.HandleFunc("GET /", a.siteHome)
	mux.Handle("POST /api/v3/public/activation-key", a.requireDB(http.HandlerFunc(a.publicActivationKey)))
	mux.HandleFunc("GET /download/windows", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, envDefault("WINDOWS_DOWNLOAD_URL", windowsDownloadFallback), http.StatusFound)
	})
	mux.HandleFunc("GET /download/android", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, envDefault("ANDROID_DOWNLOAD_URL", androidDownloadFallback), http.StatusFound)
	})
}

func (a *App) siteHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := template.Must(template.New("site").Parse(siteHTML))
	_ = tmpl.Execute(w, map[string]string{
		"WindowsURL": "/download/windows",
		"AndroidURL": "/download/android",
	})
}

func (a *App) publicActivationKey(w http.ResponseWriter, r *http.Request) {
	if err := a.ensureV3Schema(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not prepare v3 schema")
		return
	}
	client := clientIP(r)
	fingerprint := hashSecret(client)

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_error", "Could not create activation key")
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(), `DELETE FROM public_activation_requests WHERE created_at < NOW() - INTERVAL '24 hours'`); err != nil {
		writeError(w, 500, "database_error", "Could not clean activation requests")
		return
	}
	var recent int
	if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM public_activation_requests WHERE request_hash=$1 AND created_at > NOW() - INTERVAL '1 hour'`, fingerprint).Scan(&recent); err != nil {
		writeError(w, 500, "database_error", "Could not check activation limit")
		return
	}
	if recent >= 5 {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много ключей. Попробуйте позже.")
		return
	}

	code, err := makeHumanCode("TC")
	if err != nil {
		writeError(w, 500, "random_error", "Could not create activation key")
		return
	}
	expires := time.Now().UTC().Add(30 * time.Minute)
	if _, err := tx.Exec(r.Context(), `INSERT INTO activation_codes(code_hash,kind,expires_at) VALUES($1,'desktop',$2)`, hashSecret(code), expires); err != nil {
		writeError(w, 500, "database_error", "Could not save activation key")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO public_activation_requests(request_hash,created_at) VALUES($1,NOW())`, fingerprint); err != nil {
		writeError(w, 500, "database_error", "Could not save activation request")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "database_error", "Could not commit activation key")
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":             "ok",
		"code":               code,
		"expires_at":         expires,
		"expires_in_minutes": 30,
	})
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

var _ = fmt.Sprintf
var _ = os.Getenv

const siteHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>TubeControl — учёт и возврат тубусов</title>
<meta name="theme-color" content="#050908">
<style>
:root{--bg:#050908;--bg2:#07120f;--panel:#0b1512;--line:#173a31;--emerald:#10b981;--mint:#6cffcb;--text:#f6fffb;--muted:#9eb1aa;--danger:#ff5c68;--shadow:0 0 36px rgba(16,185,129,.15)}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:radial-gradient(circle at 78% 8%,rgba(16,185,129,.16),transparent 28%),linear-gradient(180deg,#030706 0%,#07110e 50%,#030706 100%);color:var(--text);font:16px/1.5 Inter,Segoe UI,Arial,sans-serif}.wrap{width:min(1180px,92vw);margin:auto}.nav{height:78px;display:flex;align-items:center;gap:28px;border-bottom:1px solid rgba(108,255,203,.09);position:sticky;top:0;background:rgba(3,7,6,.84);backdrop-filter:blur(16px);z-index:10}.brand{font-size:30px;font-weight:800;letter-spacing:-1px;margin-right:auto}.brand b{color:var(--emerald)}.nav a{color:#cfe0da;text-decoration:none;font-size:14px}.nav a:hover{color:var(--mint)}.hero{display:grid;grid-template-columns:1.05fr .95fr;gap:50px;align-items:center;padding:72px 0 52px}.eyebrow{color:var(--mint);font-size:12px;letter-spacing:.28em;text-transform:uppercase}.hero h1{font-size:58px;line-height:1.03;margin:16px 0 18px;letter-spacing:-2.2px}.hero h1 em{font-style:normal;color:var(--emerald);text-shadow:0 0 30px rgba(16,185,129,.35)}.hero p{color:var(--muted);max-width:650px;font-size:18px}.actions{display:flex;gap:12px;flex-wrap:wrap;margin-top:28px}.btn{appearance:none;border:1px solid rgba(108,255,203,.28);background:linear-gradient(180deg,rgba(16,185,129,.2),rgba(16,185,129,.08));color:white;border-radius:14px;padding:14px 20px;font-weight:700;text-decoration:none;cursor:pointer;box-shadow:var(--shadow)}.btn.primary{background:linear-gradient(135deg,#0ee09d,#10b981);color:#02100b;border-color:#47ffc7}.btn.ghost{background:rgba(255,255,255,.03);box-shadow:none}.deviceStage{position:relative;min-height:430px}.laptop{position:absolute;left:0;top:20px;width:82%;border:1px solid rgba(108,255,203,.22);border-radius:24px;background:linear-gradient(160deg,#0d1a16,#06100d);padding:18px;box-shadow:0 32px 90px #000,0 0 60px rgba(16,185,129,.13);transform:perspective(900px) rotateY(-7deg)}.screenHead{display:flex;justify-content:space-between;color:#dffaf1;font-size:13px;margin-bottom:14px}.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}.stat{padding:14px;border:1px solid rgba(108,255,203,.12);border-radius:14px;background:rgba(0,0,0,.22)}.stat strong{font-size:25px;color:var(--mint)}.tableMock{margin-top:12px;border-radius:14px;overflow:hidden;border:1px solid rgba(108,255,203,.1)}.row{display:grid;grid-template-columns:1fr 1fr .9fr;gap:10px;padding:10px 12px;font-size:12px;color:#bcd0c8;border-bottom:1px solid rgba(255,255,255,.05)}.phone{position:absolute;right:4px;bottom:8px;width:31%;min-width:150px;border:1px solid rgba(108,255,203,.35);border-radius:30px;background:#050b09;padding:15px 12px 18px;box-shadow:0 24px 70px #000,0 0 36px rgba(16,185,129,.22)}.tubeIcon{height:92px;border-radius:20px;margin:16px 0;background:radial-gradient(circle at 68% 40%,rgba(108,255,203,.45),transparent 20%),linear-gradient(135deg,#0c1915,#10231d);display:grid;place-items:center;color:var(--mint);font-size:42px}.phone .small{font-size:11px;color:#9bb3aa}.section{padding:48px 0}.sectionTitle{font-size:34px;margin:0 0 8px}.sectionLead{color:var(--muted);margin:0 0 24px}.keyCard{display:grid;grid-template-columns:1fr auto;gap:26px;align-items:center;padding:28px;border:1px solid rgba(108,255,203,.2);background:linear-gradient(135deg,rgba(16,185,129,.09),rgba(4,10,8,.75));border-radius:24px;box-shadow:var(--shadow)}.keyValue{font:800 30px/1.2 ui-monospace,SFMono-Regular,Consolas,monospace;color:var(--mint);letter-spacing:.08em}.keyMeta{font-size:13px;color:var(--muted);margin-top:8px}.steps,.features{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}.card{border:1px solid rgba(108,255,203,.13);background:rgba(7,17,14,.76);border-radius:18px;padding:20px;min-height:170px}.num{width:38px;height:38px;display:grid;place-items:center;border-radius:10px;background:rgba(16,185,129,.17);color:var(--mint);font-weight:800;margin-bottom:28px}.card h3{margin:0 0 8px}.card p{color:var(--muted);font-size:14px;margin:0}.featureIcon{font-size:28px;color:var(--mint);margin-bottom:22px}.footer{padding:36px 0 54px;border-top:1px solid rgba(108,255,203,.08);display:flex;justify-content:space-between;color:#799087;font-size:13px}.toast{position:fixed;right:24px;bottom:24px;max-width:360px;padding:16px 18px;border:1px solid rgba(108,255,203,.24);border-radius:14px;background:#09130f;color:#eafff8;box-shadow:0 14px 50px #000;display:none}.toast.err{border-color:rgba(255,92,104,.42)}
@media(max-width:900px){.hero{grid-template-columns:1fr}.deviceStage{min-height:380px}.hero h1{font-size:44px}.steps,.features{grid-template-columns:1fr 1fr}.nav a{display:none}.keyCard{grid-template-columns:1fr}}@media(max-width:560px){.steps,.features{grid-template-columns:1fr}.hero h1{font-size:38px}.deviceStage{min-height:300px}.keyValue{font-size:24px}}
</style>
</head><body>
<header class="nav"><div class="wrap" style="display:flex;align-items:center;gap:28px;width:min(1180px,92vw)"><div class="brand">Tube<b>Control</b></div><a href="#download">Скачать</a><a href="#key">Ключ активации</a><a href="#how">Как это работает</a><a href="#features">Возможности</a></div></header>
<main class="wrap">
<section class="hero" id="download"><div><div class="eyebrow">Учёт · возврат · под контролем</div><h1>Контролируйте тубусы.<br><em>Возвращайте. Сохраняйте.</em></h1><p>TubeControl помогает отслеживать отправленные тубусы, регистрировать возвраты, сокращать потери и подключать Android‑сканеры без ручного ввода секретных токенов.</p><div class="actions"><a class="btn primary" href="{{.WindowsURL}}">⬇ Скачать для Windows</a><a class="btn" href="{{.AndroidURL}}">⬇ Скачать для Android</a><a class="btn ghost" href="#key">Получить одноразовый ключ</a></div></div>
<div class="deviceStage"><div class="laptop"><div class="screenHead"><b>TubeControl</b><span>Тёмная тема ●</span></div><div class="stats"><div class="stat"><small>Всего</small><br><strong>1 248</strong></div><div class="stat"><small>В пути</small><br><strong>892</strong></div><div class="stat"><small>Возвращено</small><br><strong>356</strong></div></div><div class="tableMock"><div class="row"><b>THU‑004872</b><span>Магазин 0142</span><span>В пути</span></div><div class="row"><b>THU‑004871</b><span>Магазин 0207</span><span style="color:#6cffcb">Возвращён</span></div><div class="row"><b>THU‑004864</b><span>Магазин 0109</span><span style="color:#ff6972">Потерян</span></div></div></div><div class="phone"><div style="font-weight:800">Tube<span style="color:#10b981">Control</span></div><div class="tubeIcon">◉✓</div><b>Готов к работе</b><div class="small">Подключение через QR из Windows‑приложения</div></div></div></section>
<section class="section" id="key"><div class="keyCard"><div><div class="eyebrow">Самостоятельная активация</div><h2 class="sectionTitle">Получите уникальный одноразовый ключ</h2><p class="sectionLead">Администратор не нужен. Ключ действует 30 минут и становится недействительным сразу после успешной активации одного Windows‑компьютера.</p><div id="keyBox" style="display:none"><div class="keyValue" id="keyValue"></div><div class="keyMeta" id="keyMeta"></div></div></div><button class="btn primary" id="keyBtn" onclick="getKey()">Получить ключ</button></div></section>
<section class="section" id="how"><div class="eyebrow">Как это работает</div><h2 class="sectionTitle">Четыре шага до готовой системы</h2><div class="steps"><div class="card"><div class="num">1</div><h3>Скачайте приложение</h3><p>Установите TubeControl на Windows и Android.</p></div><div class="card"><div class="num">2</div><h3>Получите ключ</h3><p>Сайт мгновенно выдаст уникальный одноразовый код.</p></div><div class="card"><div class="num">3</div><h3>Активируйте Windows</h3><p>Введите код прямо внутри приложения. Браузер не используется.</p></div><div class="card"><div class="num">4</div><h3>Подключите Android</h3><p>В Windows нажмите «Подключить устройство» и отсканируйте QR.</p></div></div></section>
<section class="section" id="features"><div class="eyebrow">Возможности</div><h2 class="sectionTitle">Создано для ежедневной работы</h2><div class="features"><div class="card"><div class="featureIcon">◫</div><h3>Прозрачный учёт</h3><p>THU, магазин, заказ, дата отправки и состояние в одном окне.</p></div><div class="card"><div class="featureIcon">↻</div><h3>Снижение потерь</h3><p>Возвраты регистрируются Android‑сканером сразу в общей базе.</p></div><div class="card"><div class="featureIcon">⚡</div><h3>Быстрое внедрение</h3><p>Активация без администратора и без ручного ввода секретных токенов.</p></div><div class="card"><div class="featureIcon">⌗</div><h3>QR‑подключение</h3><p>Каждый телефон получает собственный ключ доступа и может быть отключён отдельно.</p></div></div></section>
</main><footer class="wrap footer"><span>TubeControl · меньше отходов, больше контроля.</span><span>v3</span></footer><div id="toast" class="toast"></div>
<script>
async function getKey(){const b=document.getElementById('keyBtn');b.disabled=true;b.textContent='Создаём…';try{const r=await fetch('/api/v3/public/activation-key',{method:'POST'});const x=await r.json();if(!r.ok)throw new Error(x.message||x.error||('HTTP '+r.status));document.getElementById('keyValue').textContent=x.code;document.getElementById('keyMeta').textContent='Действует 30 минут. Используется один раз.';document.getElementById('keyBox').style.display='block';b.textContent='Получить новый ключ'}catch(e){showToast(e.message,true);b.textContent='Получить ключ'}finally{b.disabled=false}}function showToast(t,err){const x=document.getElementById('toast');x.textContent=t;x.className='toast'+(err?' err':'');x.style.display='block';setTimeout(()=>x.style.display='none',5000)}
</script></body></html>`
