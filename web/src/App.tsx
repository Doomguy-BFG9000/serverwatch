import { useEffect, useMemo, useState } from 'react'
import './App.css'

type MonitorStatus = {
  id: number
  name: string
  monitor_type: string
  target: string
  enabled: boolean
  status: string
  response_time_ms?: number
  status_code?: number
  message?: string
  last_checked_at?: string
}

function App() {
  const [monitors, setMonitors] = useState<MonitorStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  async function loadMonitors() {
    try {
      const response = await fetch('/api/v1/monitors/status')

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const data = await response.json()
      setMonitors(data)
      setError('')
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : 'Failed to load monitors',
      )
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadMonitors()

    const timer = window.setInterval(loadMonitors, 10000)

    return () => window.clearInterval(timer)
  }, [])

  const summary = useMemo(() => {
    return {
      total: monitors.length,
      up: monitors.filter((item) => item.status === 'up').length,
      down: monitors.filter((item) => item.status === 'down').length,
      pending: monitors.filter(
        (item) => item.status === 'pending',
      ).length,
    }
  }, [monitors])

  function formatLastChecked(value?: string) {
    if (!value) {
      return 'Not checked yet'
    }

    return new Date(value).toLocaleString()
  }

  return (
    <div className="app">
      <header className="topbar">
        <div>
          <h1>ServerWatch</h1>
          <p>Community Edition</p>
        </div>

        <div className="live-indicator">
          <span />
          Live
        </div>
      </header>

      <main className="content">
        <section className="summary-grid">
          <div className="summary-card">
            <span>Total Monitors</span>
            <strong>{summary.total}</strong>
          </div>

          <div className="summary-card">
            <span>Up</span>
            <strong>{summary.up}</strong>
          </div>

          <div className="summary-card">
            <span>Down</span>
            <strong>{summary.down}</strong>
          </div>

          <div className="summary-card">
            <span>Pending</span>
            <strong>{summary.pending}</strong>
          </div>
        </section>

        <section className="monitor-section">
          <div className="section-header">
            <div>
              <h2>Monitors</h2>
              <p>Current infrastructure status</p>
            </div>

            <button onClick={loadMonitors}>
              Refresh
            </button>
          </div>

          {loading && (
            <div className="message">
              Loading monitors...
            </div>
          )}

          {error && (
            <div className="message error">
              Failed to load monitors: {error}
            </div>
          )}

          {!loading && !error && monitors.length === 0 && (
            <div className="message">
              No monitors configured.
            </div>
          )}

          {!loading && monitors.length > 0 && (
            <div className="monitor-list">
              {monitors.map((monitor) => (
                <article
                  className="monitor-card"
                  key={monitor.id}
                >
                  <div className="monitor-main">
                    <div
                      className={`status-dot ${monitor.status}`}
                    />

                    <div>
                      <h3>{monitor.name}</h3>
                      <p>{monitor.target}</p>
                    </div>
                  </div>

                  <div className="monitor-meta">
                    <div>
                      <span>Status</span>
                      <strong
                        className={`status-text ${monitor.status}`}
                      >
                        {monitor.status.toUpperCase()}
                      </strong>
                    </div>

                    <div>
                      <span>Response</span>
                      <strong>
                        {monitor.response_time_ms !== undefined
                          ? `${monitor.response_time_ms} ms`
                          : '—'}
                      </strong>
                    </div>

                    <div>
                      <span>HTTP</span>
                      <strong>
                        {monitor.status_code ?? '—'}
                      </strong>
                    </div>

                    <div>
                      <span>Last checked</span>
                      <strong>
                        {formatLastChecked(
                          monitor.last_checked_at,
                        )}
                      </strong>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
      </main>
    </div>
  )
}

export default App
