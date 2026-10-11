package httpapi

import "github.com/serverwatch/serverwatch/server/internal/version"

func statusPageVersion() string {
	return version.Version
}

// statusPageHTML is the full document served at /status. It uses no inline
// styles or scripts so it is compatible with the strict default CSP.
const statusPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<meta name="theme-color" content="#4f46e5">
<title>ServerWatch Status</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="stylesheet" href="/status.css">
</head>
<body>
<main class="page">
  <header class="page-header">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none"><path d="M2 12h4l3-7 4 14 3-7h6" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </span>
      <div>
        <h1>ServerWatch Status</h1>
        <p class="subtitle">Live service availability</p>
      </div>
    </div>
    <span class="version">v{{.Version}}</span>
  </header>

  <div class="overall overall-{{.Overall}}">
    <span class="overall-dot"></span>
    <span class="overall-text">{{.OverallText}}</span>
  </div>

  <ul class="summary">
    <li class="summary-up"><span class="count">{{.Summary.Up}}</span><span>Operational</span></li>
    <li class="summary-down"><span class="count">{{.Summary.Down}}</span><span>Outages</span></li>
    <li class="summary-pending"><span class="count">{{.Summary.Pending}}</span><span>No data</span></li>
    <li class="summary-maintenance"><span class="count">{{.Summary.Maintenance}}</span><span>Maintenance</span></li>
    <li class="summary-disabled"><span class="count">{{.Summary.Disabled}}</span><span>Disabled</span></li>
  </ul>

  {{range .Groups}}
  <section class="group">
    <h2>{{.Name}}</h2>
    <ul class="monitors">
      {{range .Monitors}}
      <li class="monitor" data-state="{{.State}}">
        <span class="state-dot state-{{.State}}"></span>
        <span class="monitor-name">{{.Name}}</span>
        <span class="monitor-meta">{{.Type}}</span>
        <span class="monitor-state">{{.StateLabel}}</span>
        <span class="monitor-time">{{.ResponseTime}} · {{.LastChecked}}</span>
      </li>
      {{end}}
    </ul>
  </section>
  {{end}}

  <footer class="page-footer">Updated {{.GeneratedAt}}</footer>
</main>
</body>
</html>
`

// statusPageCSS is served at /status.css. It shares the dashboard's visual
// language (Inter fallback stack, gradient accent, status palette, soft cards)
// but stays dependency-free and theme-aware through prefers-color-scheme.
const statusPageCSS = `:root{
--bg:#f4f6fb;--surface:#fff;--surface-2:#f7f9fc;--border:#e3e8f0;--text:#1c2536;--text-strong:#0f172a;--muted:#667085;--faint:#98a2b3;
--accent:#4f46e5;--accent-2:#8b5cf6;--accent-grad:linear-gradient(135deg,#6366f1,#8b5cf6 55%,#a855f7);
--up:#16a34a;--up-soft:rgba(22,163,74,.12);--down:#e11d48;--down-soft:rgba(225,29,72,.12);--pending:#d97706;--pending-soft:rgba(217,119,6,.14);--maintenance:#7c3aed;--maintenance-soft:rgba(124,58,237,.12);--disabled:#64748b;--disabled-soft:rgba(100,116,139,.14);
--shadow:0 12px 28px -14px rgba(15,23,42,.28);color-scheme:light}
*{box-sizing:border-box}
body{margin:0;font-family:"Inter","Inter Variable",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;color:var(--text);background-color:var(--bg);background-image:radial-gradient(52rem 38rem at 100% -12%,rgba(99,102,241,.16),transparent 60%),radial-gradient(46rem 34rem at -10% 2%,rgba(168,85,247,.13),transparent 58%);background-attachment:fixed;background-repeat:no-repeat;min-height:100vh}
.page{max-width:56rem;margin:0 auto;padding:2.5rem 1.15rem 4rem;-webkit-font-smoothing:antialiased}
.page-header{display:flex;align-items:center;justify-content:space-between;gap:1rem;margin-bottom:1.75rem}
.brand{display:flex;align-items:center;gap:.75rem;min-width:0}
.brand-mark{flex:0 0 auto;width:2.5rem;height:2.5rem;border-radius:.75rem;display:grid;place-items:center;background:var(--accent-grad);box-shadow:0 12px 26px -12px rgba(79,70,229,.6)}
.brand-mark svg{width:1.35rem;height:1.35rem;display:block}
.page-header h1{font-size:1.35rem;margin:0;letter-spacing:-.02em;color:var(--text-strong)}
.subtitle{margin:.15rem 0 0;font-size:.8rem;color:var(--muted)}
.version{flex:0 0 auto;color:var(--muted);font-size:.8rem;font-weight:600;border:1px solid var(--border);border-radius:999px;padding:.25rem .7rem;background:var(--surface)}
.overall{display:flex;align-items:center;gap:.7rem;padding:1.1rem 1.3rem;border-radius:1rem;font-weight:700;font-size:1.05rem;margin-bottom:1.5rem;border:1px solid transparent;box-shadow:var(--shadow)}
.overall-dot{width:.8rem;height:.8rem;border-radius:50%;flex:0 0 auto}
.overall-operational{background:var(--up-soft);color:var(--up);border-color:rgba(22,163,74,.28)}
.overall-operational .overall-dot{background:var(--up);box-shadow:0 0 0 5px var(--up-soft)}
.overall-degraded{background:var(--down-soft);color:var(--down);border-color:rgba(225,29,72,.28)}
.overall-degraded .overall-dot{background:var(--down);box-shadow:0 0 0 5px var(--down-soft)}
.overall-empty{background:var(--disabled-soft);color:var(--disabled);border-color:var(--border)}
.overall-empty .overall-dot{background:var(--disabled)}
.summary{list-style:none;display:flex;flex-wrap:wrap;gap:.75rem;padding:0;margin:0 0 2rem}
.summary li{flex:1 1 7.5rem;display:flex;flex-direction:column;gap:.2rem;padding:.9rem 1.1rem;background:var(--surface);border:1px solid var(--border);border-radius:.85rem;box-shadow:0 2px 8px rgba(15,23,42,.05)}
.summary .count{font-size:1.6rem;font-weight:800;line-height:1;color:var(--text-strong);font-variant-numeric:tabular-nums}
.summary li span:last-child{font-size:.75rem;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}
.summary-up .count{color:var(--up)}.summary-down .count{color:var(--down)}.summary-pending .count{color:var(--pending)}.summary-maintenance .count{color:var(--maintenance)}.summary-disabled .count{color:var(--disabled)}
.group{margin-bottom:2rem}
.group h2{font-size:.78rem;text-transform:uppercase;letter-spacing:.07em;color:var(--muted);margin:0 0 .7rem}
.monitors{list-style:none;padding:0;margin:0;border:1px solid var(--border);border-radius:.9rem;overflow:hidden;background:var(--surface);box-shadow:var(--shadow)}
.monitor{position:relative;display:grid;grid-template-columns:auto 1fr auto auto;grid-gap:.5rem 1rem;align-items:center;padding:.9rem 1.1rem;border-bottom:1px solid var(--border);transition:background .15s ease}
.monitor:last-child{border-bottom:0}
.monitor:hover{background:var(--surface-2)}
.monitor::before{content:"";position:absolute;left:0;top:12px;bottom:12px;width:3px;border-radius:0 3px 3px 0;background:transparent}
[data-state="up"]::before{background:var(--up)}
[data-state="down"]::before{background:var(--down)}
[data-state="pending"]::before{background:var(--pending)}
[data-state="maintenance"]::before{background:var(--maintenance)}
[data-state="disabled"]::before{background:var(--disabled)}
.monitor-name{font-weight:600;color:var(--text-strong)}
.monitor-meta{font-size:.72rem;color:var(--faint);text-transform:uppercase;letter-spacing:.04em;grid-column:2}
.monitor-state{font-size:.78rem;font-weight:700;text-align:right}
.monitor-time{font-size:.75rem;color:var(--muted);text-align:right;grid-column:3 / span 2}
.state-dot{width:.7rem;height:.7rem;border-radius:50%;grid-row:span 2;flex:0 0 auto}
.state-up{background:var(--up);box-shadow:0 0 0 4px var(--up-soft)}
.state-down{background:var(--down);box-shadow:0 0 0 4px var(--down-soft);animation:sw-pulse 1.8s ease-out infinite}
.state-pending{background:var(--pending);box-shadow:0 0 0 4px var(--pending-soft)}
.state-maintenance{background:var(--maintenance);box-shadow:0 0 0 4px var(--maintenance-soft)}
.state-disabled{background:var(--disabled);box-shadow:0 0 0 4px var(--disabled-soft)}
[data-state="down"] .monitor-state{color:var(--down)}
[data-state="up"] .monitor-state{color:var(--up)}
[data-state="maintenance"] .monitor-state{color:var(--maintenance)}
[data-state="disabled"] .monitor-state,[data-state="pending"] .monitor-state{color:var(--muted)}
.page-footer{color:var(--muted);font-size:.8rem;text-align:center;margin-top:2rem}
@keyframes sw-pulse{0%{box-shadow:0 0 0 0 var(--down-soft)}70%{box-shadow:0 0 0 6px transparent}100%{box-shadow:0 0 0 0 transparent}}
@media (prefers-color-scheme:dark){
:root{--bg:#080b14;--surface:#101725;--surface-2:#161f31;--border:#222d42;--text:#d6dced;--text-strong:#f4f7ff;--muted:#94a3b8;--faint:#64748b;
--up:#34d399;--up-soft:rgba(52,211,153,.16);--down:#fb7185;--down-soft:rgba(251,113,133,.16);--pending:#fbbf24;--pending-soft:rgba(251,191,36,.16);--maintenance:#a78bfa;--maintenance-soft:rgba(167,139,250,.16);--disabled:#94a3b8;--disabled-soft:rgba(148,163,184,.16);--shadow:0 22px 44px -20px rgba(0,0,0,.7)}
}
@media (prefers-reduced-motion:reduce){*{animation-duration:.001ms !important;transition-duration:.001ms !important}}
`
