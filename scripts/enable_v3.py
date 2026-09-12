from pathlib import Path
p = Path('main.go')
s = p.read_text(encoding='utf-8')
needle = '\tregisterDeviceRoutes(mux, app)\n'
replacement = '\tregisterDeviceRoutes(mux, app)\n\tregisterSiteRoutes(mux, app)\n\tregisterStatusRoutes(mux, app)\n'
if 'registerSiteRoutes(mux, app)' not in s:
    if needle not in s:
        raise SystemExit('main.go patch point not found')
    s = s.replace(needle, replacement, 1)
    p.write_text(s, encoding='utf-8')
