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
<title>ServerWatch Status</title>
<link rel="stylesheet" href="/status.css">
</head>
<body>
<main class="page">
  <header class="page-header">
    <h1>ServerWatch Status</h1>
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

// statusPageCSS is served at /status.css.
const statusPageCSS = `:root{color-scheme:light dark}
*{box-sizing:border-box}
body{margin:0;font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:#f6f7f9;color:#1b1f24}
.page{max-width:56rem;margin:0 auto;padding:2rem 1rem 4rem}
.page-header{display:flex;align-items:baseline;justify-content:space-between;gap:1rem;margin-bottom:1.5rem}
.page-header h1{font-size:1.4rem;margin:0}
.version{color:#6b7280;font-size:.85rem}
.overall{display:flex;align-items:center;gap:.6rem;padding:1rem 1.25rem;border-radius:.75rem;font-weight:600;margin-bottom:1.5rem}
.overall-dot{width:.75rem;height:.75rem;border-radius:50%;flex:0 0 auto}
.overall-operational{background:#e7f6ec;color:#12603a}
.overall-operational .overall-dot{background:#16a34a}
.overall-degraded{background:#fdeaea;color:#8a1c1c}
.overall-degraded .overall-dot{background:#dc2626}
.overall-empty{background:#eef1f5;color:#3f4652}
.overall-empty .overall-dot{background:#94a3b8}
.summary{list-style:none;display:flex;flex-wrap:wrap;gap:.75rem;padding:0;margin:0 0 2rem}
.summary li{flex:1 1 8rem;display:flex;flex-direction:column;gap:.15rem;padding:.75rem 1rem;background:#fff;border:1px solid #e5e7eb;border-radius:.6rem}
.summary .count{font-size:1.4rem;font-weight:700}
.summary li span:last-child{font-size:.8rem;color:#6b7280}
.group{margin-bottom:2rem}
.group h2{font-size:.8rem;text-transform:uppercase;letter-spacing:.06em;color:#6b7280;margin:0 0 .6rem}
.monitors{list-style:none;padding:0;margin:0;border:1px solid #e5e7eb;border-radius:.6rem;overflow:hidden;background:#fff}
.monitor{display:grid;grid-template-columns:auto 1fr auto auto;grid-gap:.5rem 1rem;align-items:center;padding:.8rem 1rem;border-bottom:1px solid #f0f1f3}
.monitor:last-child{border-bottom:0}
.monitor-name{font-weight:600}
.monitor-meta{font-size:.75rem;color:#6b7280;grid-column:2}
.monitor-state{font-size:.8rem;font-weight:600;text-align:right}
.monitor-time{font-size:.75rem;color:#6b7280;text-align:right;grid-column:3 / span 2}
.state-dot{width:.7rem;height:.7rem;border-radius:50%;grid-row:span 2}
.state-up{background:#16a34a}
.state-down{background:#dc2626}
.state-pending{background:#d97706}
.state-maintenance{background:#2563eb}
.state-disabled{background:#94a3b8}
[data-state="down"] .monitor-state{color:#b91c1c}
[data-state="up"] .monitor-state{color:#15803d}
[data-state="maintenance"] .monitor-state{color:#1d4ed8}
[data-state="disabled"] .monitor-state,[data-state="pending"] .monitor-state{color:#6b7280}
.page-footer{color:#6b7280;font-size:.8rem;text-align:center;margin-top:2rem}
@media (prefers-color-scheme:dark){
body{background:#0f1216;color:#e6e8eb}
.summary li,.monitors{background:#171b21;border-color:#2a2f37}
.monitor{border-bottom-color:#222831}
.group h2,.version,.summary li span:last-child,.monitor-meta,.monitor-time,.page-footer{color:#9aa4b2}
.overall-operational{background:#12281c;color:#7ee2a8}
.overall-degraded{background:#2a1414;color:#f3a3a3}
.overall-empty{background:#1b2027;color:#b6bec9}
}
`
