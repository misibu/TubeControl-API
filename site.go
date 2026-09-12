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
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TubeControl — учёт и возврат тубусов</title>
<meta name="theme-color" content="#06100d">
<style>
:root{--bg:#06100d;--bg2:#0a1713;--panel:#10201b;--panel2:#132823;--mint:#75ffd0;--emerald:#19d99a;--line:#58e9bb;--text:#f6fffb;--muted:#a5bdb5;--danger:#ff6b72;--yellow:#ffd43b}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:linear-gradient(180deg,#050d0b,#091511 52%,#050d0b);color:var(--text);font-family:"Segoe UI",Arial,sans-serif}.wrap{width:min(1040px,92vw);margin:auto}.top{padding:34px 0 18px;border-bottom:1px solid rgba(117,255,208,.12)}.toprow{display:flex;align-items:flex-start;justify-content:space-between;gap:24px}.logo{font-size:48px;line-height:1;font-weight:800;letter-spacing:-1.8px}.logo span{color:var(--mint)}.tag{margin-top:12px;color:var(--mint);font-size:14px;font-weight:800;letter-spacing:.16em}.nav{display:flex;gap:10px;flex-wrap:wrap}.pill,.btn{border:1.5px solid var(--line);border-radius:999px;background:transparent;color:var(--text);padding:13px 19px;text-decoration:none;font-weight:800;cursor:pointer;transition:.18s ease}.pill:hover,.btn:hover{background:rgba(25,217,154,.10);box-shadow:0 0 0 3px rgba(25,217,154,.08)}.btn.primary{background:linear-gradient(180deg,#2ee6b0,#15cf96);color:#032018;border-color:#8dffdc;box-shadow:0 0 24px rgba(25,217,154,.16)}.hero{padding:58px 0 36px}.hero h1{font-size:54px;line-height:1.06;margin:0 0 20px;max-width:760px;letter-spacing:-1.6px}.hero h1 span{color:var(--mint)}.hero p{max-width:700px;color:var(--muted);font-size:18px;margin:0}.actions{display:flex;gap:12px;flex-wrap:wrap;margin-top:30px}.section{padding:28px 0}.section h2{font-size:29px;margin:0 0 10px}.sectionLead{color:var(--muted);margin:0 0 20px}.box{border:1.5px solid rgba(117,255,208,.46);border-radius:28px;background:linear-gradient(180deg,rgba(26,62,51,.64),rgba(10,24,19,.9));padding:28px}.downloadGrid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.downloadCard{border:1.5px solid rgba(117,255,208,.30);border-radius:24px;padding:24px;background:rgba(16,32,27,.72)}.downloadCard h3{font-size:24px;margin:0 0 8px}.downloadCard p{color:var(--muted);margin:0 0 20px}.keybox{display:grid;grid-template-columns:1fr auto;gap:24px;align-items:center}.keyValue{font:800 34px/1.2 Consolas,monospace;letter-spacing:.07em;color:var(--mint);margin-top:14px}.keyMeta{color:var(--muted);font-size:13px;margin-top:7px}.steps{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.step{border:1.5px solid rgba(117,255,208,.22);border-radius:22px;padding:20px;background:rgba(15,34,28,.70)}.num{width:38px;height:38px;border:1.5px solid var(--line);border-radius:50%;display:grid;place-items:center;color:var(--mint);font-weight:900;margin-bottom:18px}.step h3{margin:0 0 7px;font-size:17px}.step p{margin:0;color:var(--muted);font-size:14px}.footer{padding:34px 0 46px;color:#75958a;font-size:13px;text-align:center}.toast{position:fixed;right:22px;bottom:22px;display:none;max-width:360px;padding:15px 18px;border:1.5px solid var(--line);border-radius:20px;background:#10201b;box-shadow:0 18px 50px #000;color:#fff}.toast.err{border-color:var(--danger)}
@media(max-width:760px){.toprow{display:block}.nav{margin-top:22px}.logo{font-size:40px}.hero h1{font-size:40px}.downloadGrid,.steps{grid-template-columns:1fr}.keybox{grid-template-columns:1fr}.actions .btn,.actions a{width:100%;text-align:center}.nav .pill{padding:10px 14px;font-size:13px}}
</style>
</head>
<body>
<header class="top"><div class="wrap toprow"><div><div class="logo">Tube<span>Control</span></div><div class="tag">УЧЁТ · ВОЗВРАТ · ПОД КОНТРОЛЕМ</div></div><nav class="nav"><a class="pill" href="#download">СКАЧАТЬ</a><a class="pill" href="#key">КЛЮЧ</a><a class="pill" href="#how">КАК ЭТО РАБОТАЕТ</a></nav></div></header>
<main class="wrap">
<section class="hero"><h1>УЧЁТ ТУБУСОВ БЕЗ ЛИШНИХ ДЕЙСТВИЙ. <span>ОДНА СИСТЕМА ДЛЯ WINDOWS И ANDROID.</span></h1><p>Сканирование Code 128, регистрация возвратов, ручные статусы, контроль просрочки и подключение Android только по QR-коду.</p><div class="actions"><a class="btn primary" href="{{.WindowsURL}}">СКАЧАТЬ ДЛЯ WINDOWS</a><a class="btn" href="{{.AndroidURL}}">СКАЧАТЬ ДЛЯ ANDROID</a><a class="btn" href="#key">ПОЛУЧИТЬ КЛЮЧ</a></div></section>
<section class="section" id="download"><div class="downloadGrid"><div class="downloadCard"><h3>WINDOWS</h3><p>Импорт Excel, список THU, статусы, выгрузка и подключение Android.</p><a class="btn primary" href="{{.WindowsURL}}">СКАЧАТЬ .EXE</a></div><div class="downloadCard"><h3>ANDROID</h3><p>Сканирование Code 128 и регистрация возврата. Подключение только по QR из Windows.</p><a class="btn" href="{{.AndroidURL}}">СКАЧАТЬ .APK</a></div></div></section>
<section class="section" id="key"><div class="box keybox"><div><h2>ОДНОРАЗОВЫЙ КЛЮЧ WINDOWS</h2><p class="sectionLead">Ключ создаётся автоматически, действует 30 минут и используется только один раз.</p><div id="keyBox" style="display:none"><div class="keyValue" id="keyValue"></div><div class="keyMeta" id="keyMeta"></div></div></div><button class="btn primary" id="keyBtn" onclick="getKey()">ПОЛУЧИТЬ КЛЮЧ</button></div></section>
<section class="section" id="how"><h2>КАК ЭТО РАБОТАЕТ</h2><p class="sectionLead">Минимум действий на каждом устройстве.</p><div class="steps"><div class="step"><div class="num">1</div><h3>СКАЧАТЬ</h3><p>Установите TubeControl для Windows.</p></div><div class="step"><div class="num">2</div><h3>ПОЛУЧИТЬ КЛЮЧ</h3><p>Сайт выдаст одноразовый код активации.</p></div><div class="step"><div class="num">3</div><h3>АКТИВИРОВАТЬ</h3><p>Введите код прямо в Windows-приложении.</p></div><div class="step"><div class="num">4</div><h3>ПОДКЛЮЧИТЬ ANDROID</h3><p>Откройте настройки Windows и отсканируйте QR-код.</p></div></div></section>
</main>
<footer class="wrap footer">TubeControl · УЧЁТ · ВОЗВРАТ · ПОД КОНТРОЛЕМ</footer>
<div id="toast" class="toast"></div>
<script>
async function getKey(){const b=document.getElementById('keyBtn');b.disabled=true;b.textContent='СОЗДАЁМ…';try{const r=await fetch('/api/v3/public/activation-key',{method:'POST'});const x=await r.json();if(!r.ok)throw new Error(x.message||x.error||('HTTP '+r.status));document.getElementById('keyValue').textContent=x.code;document.getElementById('keyMeta').textContent='Действует 30 минут. Используется один раз.';document.getElementById('keyBox').style.display='block';b.textContent='ПОЛУЧИТЬ НОВЫЙ КЛЮЧ'}catch(e){showToast(e.message,true);b.textContent='ПОЛУЧИТЬ КЛЮЧ'}finally{b.disabled=false}}function showToast(t,err){const x=document.getElementById('toast');x.textContent=t;x.className='toast'+(err?' err':'');x.style.display='block';setTimeout(()=>x.style.display='none',5000)}
</script>
</body></html>`
