import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
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
  interval_seconds: number
  timeout_seconds: number
  config?: MonitorConfig
  group_id?: number | null
  group_name?: string
}

type Group = {
  id: number
  name: string
  created_at: string
  updated_at: string
}

type MonitorConfig = {
  record_type?: string
  expected_values?: string[]
  ports?: number[]
}

type MonitorResult = {
  id: number
  monitor_id: number
  status: string
  response_time_ms: number
  status_code?: number
  message?: string
  checked_at: string
}

type SSLDetails = {
  monitor_id: number
  status: string
  response_time_ms: number
  subject: string
  issuer: string
  not_before: string
  not_after: string
  days_remaining: number
  certificate_valid: boolean
  message: string
}

type NewMonitor = {
  name: string
  monitor_type: string
  target: string
  interval_seconds: number
  timeout_seconds: number
  config: MonitorConfig
  group_id: number | null
}
type DNSDetails = {
  record_type: string
  target: string
  records: string[]
  expected_values: string[]
  matched: boolean
  response_time_ms: number
  message: string
}
type PortCheckResult = {
  port: number
  status: string
  response_time_ms: number
  message: string
}
type PortDetails = {
  monitor_id: number
  target: string
  status: string
  ports: PortCheckResult[]
  reachable: number
  total: number
  response_time_ms: number
  message: string
}
type MonitorStats = {
  monitor_id: number
  total_checks: number
  up_checks: number
  down_checks: number
  uptime_percentage: number
  average_response_ms: number
}

type MonitorChartPoint = {
  checked_at: string
  average_response_ms: number
  up_checks: number
  down_checks: number
}

type TimeRange = '1h' | '24h' | '7d' | '30d'
type AlertStatus = {
  email_enabled: boolean
  smtp_configured: boolean
  recipient_configured: boolean
}

const emptyMonitor: NewMonitor = {
  name: '',
  monitor_type: 'https',
  target: '',
  interval_seconds: 60,
  timeout_seconds: 10,
  config: { record_type: 'A', expected_values: [] },
  group_id: null,
}

// parsePortsInput parses the comma-separated Ports field. Ports are normalized
// to unique whole numbers sorted ascending; invalid entries are reported
// instead of being dropped. The backend validates ports again.
function parsePortsInput(value: string): {
  ports: number[]
  error: string
} {
  const ports: number[] = []

  for (const entry of value.split(',')) {
    const text = entry.trim()

    if (text === '') {
      return {
        ports: [],
        error:
          'Enter one or more ports separated by commas, for example 22, 80, 443.',
      }
    }

    if (!/^\d+$/.test(text)) {
      return {
        ports: [],
        error: `"${text}" is not a valid port. Use whole numbers between 1 and 65535.`,
      }
    }

    const port = Number(text)

    if (port < 1 || port > 65535) {
      return {
        ports: [],
        error: `Port ${text} is out of range. Ports must be between 1 and 65535.`,
      }
    }

    ports.push(port)
  }

  const unique = Array.from(new Set(ports)).sort(
    (left, right) => left - right,
  )

  if (unique.length > 20) {
    return {
      ports: [],
      error: 'A port monitor supports at most 20 ports.',
    }
  }

  return { ports: unique, error: '' }
}

function App() {
  const [monitors, setMonitors] = useState<MonitorStatus[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [groupFilter, setGroupFilter] = useState<
    'all' | 'ungrouped' | number
  >('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [alertStatus, setAlertStatus] = useState<AlertStatus | null>(null)

  const [showGroupManager, setShowGroupManager] = useState(false)
  const [newGroupName, setNewGroupName] = useState('')
  const [groupFormError, setGroupFormError] = useState('')
  const [groupSaving, setGroupSaving] = useState(false)
  const [editingGroup, setEditingGroup] = useState<Group | null>(null)
  const [editGroupName, setEditGroupName] = useState('')
  const [editGroupError, setEditGroupError] = useState('')
  const [editGroupSaving, setEditGroupSaving] = useState(false)
  const [deletingGroup, setDeletingGroup] = useState<Group | null>(null)
  const [deleteGroupSaving, setDeleteGroupSaving] = useState(false)
  const [deleteGroupError, setDeleteGroupError] = useState('')

  const [showAddMonitor, setShowAddMonitor] = useState(false)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')
  const [newMonitor, setNewMonitor] =
    useState<NewMonitor>(emptyMonitor)

  const [selectedMonitor, setSelectedMonitor] =
    useState<MonitorStatus | null>(null)

  const [results, setResults] = useState<MonitorResult[]>([])
  const [resultsLoading, setResultsLoading] = useState(false)
  const [sslDetails, setSSLDetails] = useState<SSLDetails | null>(null)
  const [sslLoading, setSSLLoading] = useState(false)
  const [dnsDetails, setDNSDetails] = useState<DNSDetails | null>(null)
  const [dnsLoading, setDNSLoading] = useState(false)
  const [portDetails, setPortDetails] = useState<PortDetails | null>(null)
  const [portLoading, setPortLoading] = useState(false)
  const [newPortsInput, setNewPortsInput] = useState('')
  const [editPortsInput, setEditPortsInput] = useState('')

  const [chartPoints, setChartPoints] =
    useState<MonitorChartPoint[]>([])
  const [chartLoading, setChartLoading] = useState(false)

  const [editingMonitor, setEditingMonitor] =
    useState<MonitorStatus | null>(null)

  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState('')

  const [deletingMonitor, setDeletingMonitor] = useState(false)
  const [stats, setStats] = useState<MonitorStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)
  const [selectedRange, setSelectedRange] =
    useState<TimeRange>('24h')
  const responseChart = buildResponseChart(chartPoints)

  async function loadMonitorAnalytics(
    monitorID: number,
    range: TimeRange,
  ) {
    setStatsLoading(true)
    setChartLoading(true)
    setStats(null)
    setChartPoints([])

    const [statsResult, chartResult] = await Promise.allSettled([
      fetch(
        `/api/v1/monitors/${monitorID}/stats?range=${range}`,
      ).then(async (response) => {
        if (!response.ok) {
          throw new Error(`Stats HTTP ${response.status}`)
        }

        return (await response.json()) as MonitorStats
      }),
      fetch(
        `/api/v1/monitors/${monitorID}/chart?range=${range}`,
      ).then(async (response) => {
        if (!response.ok) {
          throw new Error(`Chart HTTP ${response.status}`)
        }

        return (await response.json()) as MonitorChartPoint[]
      }),
    ])

    if (statsResult.status === 'fulfilled') {
      setStats(statsResult.value)
    }

    if (chartResult.status === 'fulfilled') {
      setChartPoints(chartResult.value)
    }

    setStatsLoading(false)
    setChartLoading(false)
  }

  async function loadRecentResults(monitorID: number) {
    setResultsLoading(true)

    try {
      const response = await fetch(
        `/api/v1/monitors/${monitorID}/results?range=30d&limit=50`,
      )

      if (!response.ok) {
        throw new Error(`History HTTP ${response.status}`)
      }

      setResults((await response.json()) as MonitorResult[])
    } catch {
      setResults([])
    } finally {
      setResultsLoading(false)
    }
  }

  async function loadSSLDetails(monitorID: number) {
    setSSLLoading(true)
    setSSLDetails(null)

    try {
      const response = await fetch(
        `/api/v1/monitors/${monitorID}/ssl`,
      )
      if (!response.ok) {
        throw new Error(`SSL HTTP ${response.status}`)
      }
      setSSLDetails((await response.json()) as SSLDetails)
    } catch {
      setSSLDetails(null)
    } finally {
      setSSLLoading(false)
    }
  }
  async function loadDNSDetails(monitorID: number) {
    setDNSLoading(true)
    setDNSDetails(null)
    try {
      const response = await fetch(`/api/v1/monitors/${monitorID}/dns`)
      if (!response.ok) throw new Error(`DNS HTTP ${response.status}`)
      setDNSDetails((await response.json()) as DNSDetails)
    } catch {
      setDNSDetails(null)
    } finally { setDNSLoading(false) }
  }
  async function loadPortDetails(monitorID: number) {
    setPortLoading(true)
    setPortDetails(null)
    try {
      const response = await fetch(`/api/v1/monitors/${monitorID}/ports`)
      if (!response.ok) throw new Error(`Ports HTTP ${response.status}`)
      setPortDetails((await response.json()) as PortDetails)
    } catch {
      setPortDetails(null)
    } finally { setPortLoading(false) }
  }
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

  async function loadGroups() {
    try {
      const response = await fetch('/api/v1/groups')
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      setGroups((await response.json()) as Group[])
    } catch {
      // Keep the last known groups during a transient fetch failure.
    }
  }

  async function loadAlertStatus() {
    try {
      const response = await fetch('/api/v1/alerts/status')
      if (!response.ok) return
      setAlertStatus((await response.json()) as AlertStatus)
    } catch {
      setAlertStatus(null)
    }
  }

  useEffect(() => {
    loadMonitors()
    loadGroups()
    loadAlertStatus()

    const timer = window.setInterval(() => {
      loadMonitors()
      loadGroups()
    }, 10000)

    return () => window.clearInterval(timer)
  }, [])

  const summary = useMemo(() => {
    return {
      total: monitors.length,
      up: monitors.filter(
        (item) => item.enabled && item.status === 'up',
      ).length,
      down: monitors.filter(
        (item) => item.enabled && item.status === 'down',
      ).length,
      pending: monitors.filter(
        (item) => item.enabled && item.status === 'pending',
      ).length,
    }
  }, [monitors])

  const filteredMonitors = useMemo(() => {
    if (groupFilter === 'all') {
      return monitors
    }
    if (groupFilter === 'ungrouped') {
      return monitors.filter((monitor) => !monitor.group_id)
    }
    return monitors.filter(
      (monitor) => monitor.group_id === groupFilter,
    )
  }, [monitors, groupFilter])

  function formatLastChecked(value?: string) {
    if (!value) {
      return 'Not checked yet'
    }

    return new Date(value).toLocaleString()
  }

  function closeAddMonitor() {
    if (saving) {
      return
    }

    setShowAddMonitor(false)
    setFormError('')
    setNewMonitor(emptyMonitor)
    setNewPortsInput('')
  }

async function openMonitorDetails(
  monitor: MonitorStatus,
) {
  setSelectedMonitor(monitor)
  setSelectedRange('24h')
  setResults([])
  setSSLDetails(null)
  setDNSDetails(null)
  setPortDetails(null)
  setStats(null)
  setChartPoints([])

  const requests = [
    loadMonitorAnalytics(monitor.id, '24h'),
    loadRecentResults(monitor.id),
  ]
  if (monitor.monitor_type === 'ssl') {
    requests.push(loadSSLDetails(monitor.id))
  }
  if (monitor.monitor_type === 'dns') {
    requests.push(loadDNSDetails(monitor.id))
  }
  if (monitor.monitor_type === 'port') {
    requests.push(loadPortDetails(monitor.id))
  }
  await Promise.all(requests)
}
async function changeRange(range: TimeRange) {
  if (!selectedMonitor) {
    return
  }

  setSelectedRange(range)

  await loadMonitorAnalytics(selectedMonitor.id, range)
}

  async function toggleMonitor(monitor: MonitorStatus) {
    try {
      const response = await fetch(
        `/api/v1/monitors/${monitor.id}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: monitor.name,
            monitor_type: monitor.monitor_type,
            target: monitor.target,
            interval_seconds: monitor.interval_seconds,
            timeout_seconds: monitor.timeout_seconds,
            enabled: !monitor.enabled,
            config: monitor.config,
          }),
        },
      )

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      setSelectedMonitor(null)
      await loadMonitors()
    } catch (err) {
      window.alert(
        err instanceof Error
          ? `Failed to update monitor: ${err.message}`
          : 'Failed to update monitor',
      )
    }
  }

  async function deleteMonitor(monitor: MonitorStatus) {
    const confirmed = window.confirm(
      `Delete monitor "${monitor.name}"?`,
    )

    if (!confirmed) {
      return
    }

    setDeletingMonitor(true)

    try {
      const response = await fetch(
        `/api/v1/monitors/${monitor.id}`,
        {
          method: 'DELETE',
        },
      )

      if (!response.ok && response.status !== 204) {
        throw new Error(`HTTP ${response.status}`)
      }

      setSelectedMonitor(null)
      await loadMonitors()
    } catch (err) {
      window.alert(
        err instanceof Error
          ? `Failed to delete monitor: ${err.message}`
          : 'Failed to delete monitor',
      )
    } finally {
      setDeletingMonitor(false)
    }
  }

  function closeGroupManager() {
    if (groupSaving || editGroupSaving) {
      return
    }
    setShowGroupManager(false)
    setNewGroupName('')
    setGroupFormError('')
    setEditingGroup(null)
    setEditGroupName('')
    setEditGroupError('')
  }

  async function handleCreateGroup(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault()
    setGroupSaving(true)
    setGroupFormError('')

    try {
      const response = await fetch('/api/v1/groups', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: newGroupName }),
      })
      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || `HTTP ${response.status}`)
      }

      setNewGroupName('')
      await loadGroups()
    } catch (err) {
      setGroupFormError(
        err instanceof Error
          ? err.message
          : 'Failed to create group',
      )
    } finally {
      setGroupSaving(false)
    }
  }

  async function handleRenameGroup(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault()

    if (!editingGroup) {
      return
    }

    setEditGroupSaving(true)
    setEditGroupError('')

    try {
      const response = await fetch(
        `/api/v1/groups/${editingGroup.id}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ name: editGroupName }),
        },
      )
      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || `HTTP ${response.status}`)
      }

      setEditingGroup(null)
      setEditGroupName('')
      await Promise.all([loadGroups(), loadMonitors()])
    } catch (err) {
      setEditGroupError(
        err instanceof Error
          ? err.message
          : 'Failed to rename group',
      )
    } finally {
      setEditGroupSaving(false)
    }
  }

  async function handleDeleteGroup() {
    if (!deletingGroup) {
      return
    }

    setDeleteGroupSaving(true)
    setDeleteGroupError('')

    try {
      const response = await fetch(
        `/api/v1/groups/${deletingGroup.id}`,
        { method: 'DELETE' },
      )

      if (!response.ok && response.status !== 204) {
        throw new Error(`HTTP ${response.status}`)
      }

      if (groupFilter === deletingGroup.id) {
        setGroupFilter('all')
      }
      setDeletingGroup(null)
      setDeleteGroupError('')
      await Promise.all([loadGroups(), loadMonitors()])
    } catch (err) {
      setDeleteGroupError(
        err instanceof Error
          ? err.message
          : 'Failed to delete group',
      )
    } finally {
      setDeleteGroupSaving(false)
    }
  }

  async function handleAddMonitor(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault()

    setSaving(true)
    setFormError('')

    let payload = newMonitor

    if (newMonitor.monitor_type === 'port') {
      const parsed = parsePortsInput(newPortsInput)

      if (parsed.error) {
        setFormError(parsed.error)
        setSaving(false)
        return
      }

      payload = {
        ...newMonitor,
        config: { ...newMonitor.config, ports: parsed.ports },
      }
    }

    try {
      const response = await fetch('/api/v1/monitors', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      const data = await response.json()

      if (!response.ok) {
        throw new Error(
          data.error || `HTTP ${response.status}`,
        )
      }

      setShowAddMonitor(false)
      setNewMonitor(emptyMonitor)
      setNewPortsInput('')

      await loadMonitors()
    } catch (err) {
      setFormError(
        err instanceof Error
          ? err.message
          : 'Failed to create monitor',
      )
    } finally {
      setSaving(false)
    }
  }

  async function handleEditMonitor(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault()

    if (!editingMonitor) {
      return
    }

    setEditSaving(true)
    setEditError('')

    let config = editingMonitor.config

    if (editingMonitor.monitor_type === 'port') {
      const parsed = parsePortsInput(editPortsInput)

      if (parsed.error) {
        setEditError(parsed.error)
        setEditSaving(false)
        return
      }

      config = { ...config, ports: parsed.ports }
    }

    try {
      const response = await fetch(
        `/api/v1/monitors/${editingMonitor.id}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: editingMonitor.name,
            monitor_type: editingMonitor.monitor_type,
            target: editingMonitor.target,
            interval_seconds:
              editingMonitor.interval_seconds,
            timeout_seconds:
              editingMonitor.timeout_seconds,
            enabled: editingMonitor.enabled,
            config,
            group_id: editingMonitor.group_id ?? null,
          }),
        },
      )

      const data = await response.json()

      if (!response.ok) {
        throw new Error(
          data.error || `HTTP ${response.status}`,
        )
      }

      setEditingMonitor(null)
      setSelectedMonitor(null)

      await loadMonitors()
    } catch (err) {
      setEditError(
        err instanceof Error
          ? err.message
          : 'Failed to update monitor',
      )
    } finally {
      setEditSaving(false)
    }
  }
function buildResponseChart(data: MonitorChartPoint[]) {
  if (data.length === 0) {
    return {
      points: '',
      max: 0,
      min: 0,
      average: 0,
    }
  }

  const ordered = data.filter((item) =>
    Number.isFinite(item.average_response_ms),
  )

  if (ordered.length === 0) {
    return {
      points: '',
      max: 0,
      min: 0,
      average: 0,
    }
  }

  const values = ordered.map(
    (item) => item.average_response_ms,
  )

  const max = Math.max(...values)
  const min = Math.min(...values)
  const average =
    values.reduce((total, value) => total + value, 0) /
    values.length

  const width = 700
  const height = 180
  const paddingX = 12
  const paddingY = 15

  const usableWidth = width - paddingX * 2
  const usableHeight = height - paddingY * 2

  const range = Math.max(max - min, 1)

  const points = ordered
    .map((item, index) => {
      const x =
        ordered.length === 1
          ? width / 2
          : paddingX +
            (index / (ordered.length - 1)) * usableWidth

      const y =
        paddingY +
        ((max - item.average_response_ms) / range) *
          usableHeight

      return `${x},${y}`
    })
    .join(' ')

  return {
    points,
    max,
    min,
    average,
  }
}
  return (
    <div className="app">
      <header className="topbar">
        <div>
          <h1>ServerWatch</h1>
          <p>Community Edition</p>
        </div>

        <div className="topbar-actions">
          <button
            className="primary-button"
            type="button"
            onClick={() => setShowAddMonitor(true)}
          >
            + Add Monitor
          </button>

          <div className="live-indicator">
            <span />
            Live
          </div>
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

        <section className="alert-section">
          <div>
            <h2>Email Alerts</h2>
            <p>SMTP is configured on the ServerWatch server using environment variables.</p>
          </div>
          <div className="alert-status-grid">
            <span>Email Alerts <strong>{alertStatus?.email_enabled ? 'Enabled' : 'Disabled'}</strong></span>
            <span>SMTP <strong>{alertStatus?.smtp_configured ? 'Configured' : 'Not Configured'}</strong></span>
            <span>Recipient <strong>{alertStatus?.recipient_configured ? 'Configured' : 'Not Configured'}</strong></span>
          </div>
        </section>

        <section className="monitor-section">
          <div className="section-header">
            <div>
              <h2>Monitors</h2>
              <p>Current infrastructure status</p>
            </div>

            <div className="section-header-actions">
              <label className="group-filter">
                <span>Group</span>
                <select
                  value={String(groupFilter)}
                  onChange={(event) => {
                    const value = event.target.value
                    setGroupFilter(
                      value === 'all' || value === 'ungrouped'
                        ? value
                        : Number(value),
                    )
                  }}
                >
                  <option value="all">All Groups</option>
                  <option value="ungrouped">Ungrouped</option>
                  {groups.map((group) => (
                    <option key={group.id} value={group.id}>
                      {group.name}
                    </option>
                  ))}
                </select>
              </label>

              <button
                type="button"
                onClick={() => setShowGroupManager(true)}
              >
                Manage Groups
              </button>

              <button
                type="button"
                onClick={loadMonitors}
              >
                Refresh
              </button>
            </div>
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

          {!loading &&
            !error &&
            monitors.length === 0 && (
              <div className="message">
                No monitors configured.
              </div>
            )}

          {!loading &&
            monitors.length > 0 &&
            filteredMonitors.length === 0 && (
              <div className="message">
                No monitors match this group filter.
              </div>
            )}

          {!loading && filteredMonitors.length > 0 && (
            <div className="monitor-list">
              {filteredMonitors.map((monitor) => (
                <article
                  className="monitor-card"
                  key={monitor.id}
                  onClick={() =>
                    openMonitorDetails(monitor)
                  }
                >
                  <div className="monitor-main">
                    <div
                      className={`status-dot ${
                        monitor.enabled
                          ? monitor.status
                          : 'paused'
                      }`}
                    />

                    <div>
                      <div className="monitor-title-row">
                        <h3>{monitor.name}</h3>
                        <span
                          className={
                            monitor.group_name
                              ? 'group-badge'
                              : 'group-badge ungrouped'
                          }
                        >
                          {monitor.group_name || 'Ungrouped'}
                        </span>
                      </div>
                      <p>
                        {monitor.monitor_type.toUpperCase()} ·{' '}
                        {monitor.target}
                        {monitor.monitor_type === 'port' &&
                        monitor.config?.ports?.length
                          ? ` · ${monitor.config.ports.length} ports`
                          : ''}
                      </p>
                    </div>
                  </div>

                  <div className="monitor-meta">
                    <div>
                      <span>Status</span>

                      <strong
                        className={`status-text ${
                          monitor.enabled
                            ? monitor.status
                            : 'paused'
                        }`}
                      >
                        {monitor.enabled
                          ? monitor.status.toUpperCase()
                          : 'PAUSED'}
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
                      <span>Code</span>
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

      {showAddMonitor && (
        <div className="modal-backdrop">
          <div className="modal-card">
            <div className="modal-header">
              <div>
                <h2>Add Monitor</h2>
                <p>
                  Create a new ServerWatch monitor.
                </p>
              </div>

              <button
                className="close-button"
                type="button"
                onClick={closeAddMonitor}
              >
                ×
              </button>
            </div>

            <form
              className="monitor-form"
              onSubmit={handleAddMonitor}
            >
              <label>
                Monitor name

                <input
                  type="text"
                  value={newMonitor.name}
                  onChange={(event) =>
                    setNewMonitor({
                      ...newMonitor,
                      name: event.target.value,
                    })
                  }
                  placeholder="Production Website"
                  required
                />
              </label>

              <label>
                Monitor type

                <select
                  value={newMonitor.monitor_type}
                  onChange={(event) =>
                    setNewMonitor({
                      ...newMonitor,
                      monitor_type:
                        event.target.value,
                    })
                  }
                >
                  <option value="https">
                    HTTPS
                  </option>
                  <option value="http">
                    HTTP
                  </option>
                  <option value="tcp">
                    TCP
                  </option>
                  <option value="ssl">
                    SSL Certificate
                  </option>
                  <option value="ping">
                    Ping
                  </option>
                  <option value="dns">
                    DNS
                  </option>
                  <option value="port">
                    Port
                  </option>
                </select>
              </label>

              <label>
                Target

                <input
                  type="text"
                  value={newMonitor.target}
                  onChange={(event) =>
                    setNewMonitor({
                      ...newMonitor,
                      target: event.target.value,
                    })
                  }
                  placeholder={newMonitor.monitor_type === 'dns' ? 'example.com' : newMonitor.monitor_type === 'port' ? 'example.com' : newMonitor.monitor_type === 'ping' ? 'example.com or 192.168.1.1' : 'https://example.com'}
                  required
                />
                {newMonitor.monitor_type === 'dns' && <small>DNS name only — no protocol or port. Status updates after the next scheduled check.</small>}
                {newMonitor.monitor_type === 'port' && <small>Host only — no protocol or port. Status updates after the next scheduled check.</small>}
              </label>

              <label>
                Group

                <select
                  value={newMonitor.group_id ?? ''}
                  onChange={(event) =>
                    setNewMonitor({
                      ...newMonitor,
                      group_id:
                        event.target.value === ''
                          ? null
                          : Number(event.target.value),
                    })
                  }
                >
                  <option value="">Ungrouped</option>
                  {groups.map((group) => (
                    <option key={group.id} value={group.id}>
                      {group.name}
                    </option>
                  ))}
                </select>
              </label>

              {newMonitor.monitor_type === 'port' && <>
                <label>Ports
                  <input type="text" value={newPortsInput} onChange={(event) => setNewPortsInput(event.target.value)} placeholder="22, 80, 443" />
                  <small>Enter up to 20 TCP ports separated by commas. Individual ports only — ranges are not supported.</small>
                </label>
              </>}

              {newMonitor.monitor_type === 'dns' && <>
                <label>Record Type<select value={newMonitor.config.record_type || 'A'} onChange={(event) => setNewMonitor({...newMonitor, config: {...newMonitor.config, record_type: event.target.value}})}>
                  {['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT'].map((type) => <option key={type} value={type}>{type}</option>)}
                </select></label>
                <label>Expected Value <input type="text" value={(newMonitor.config.expected_values || []).join(', ')} onChange={(event) => setNewMonitor({...newMonitor, config: {...newMonitor.config, expected_values: event.target.value.split(',').map((value) => value.trim()).filter(Boolean)}})} placeholder="Optional — e.g. 203.0.113.10" /></label>
              </>}

              <div className="form-row">
                <label>
                  Check every

                  <select
                    value={
                      newMonitor.interval_seconds
                    }
                    onChange={(event) =>
                      setNewMonitor({
                        ...newMonitor,
                        interval_seconds: Number(
                          event.target.value,
                        ),
                      })
                    }
                  >
                    <option value={30}>
                      30 seconds
                    </option>
                    <option value={60}>
                      1 minute
                    </option>
                    <option value={300}>
                      5 minutes
                    </option>
                    <option value={600}>
                      10 minutes
                    </option>
                  </select>
                </label>

                <label>
                  Timeout

                  <select
                    value={
                      newMonitor.timeout_seconds
                    }
                    onChange={(event) =>
                      setNewMonitor({
                        ...newMonitor,
                        timeout_seconds: Number(
                          event.target.value,
                        ),
                      })
                    }
                  >
                    <option value={5}>
                      5 seconds
                    </option>
                    <option value={10}>
                      10 seconds
                    </option>
                    <option value={15}>
                      15 seconds
                    </option>
                    <option value={30}>
                      30 seconds
                    </option>
                  </select>
                </label>
              </div>

              {formError && (
                <div className="form-error">
                  {formError}
                </div>
              )}

              <div className="form-actions">
                <button
                  type="button"
                  onClick={closeAddMonitor}
                  disabled={saving}
                >
                  Cancel
                </button>

                <button
                  className="primary-button"
                  type="submit"
                  disabled={saving}
                >
                  {saving
                    ? 'Creating...'
                    : 'Create Monitor'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {selectedMonitor && (
        <div className="modal-backdrop">
          <div className="modal-card monitor-detail-modal">
            <div className="modal-header">
              <div>
                <h2>{selectedMonitor.name}</h2>
                <p>
                  {selectedMonitor.target} ·{' '}
                  {selectedMonitor.group_name || 'Ungrouped'}
                </p>
              </div>

              <button
                className="close-button"
                type="button"
                onClick={() =>
                  setSelectedMonitor(null)
                }
              >
                ×
              </button>
            </div>

            <div className="detail-summary">
              <div>
                <span>Status</span>

                <strong
                  className={`status-text ${
                    selectedMonitor.enabled
                      ? selectedMonitor.status
                      : 'paused'
                  }`}
                >
                  {selectedMonitor.enabled
                    ? selectedMonitor.status.toUpperCase()
                    : 'PAUSED'}
                </strong>
              </div>

              <div>
                <span>Response</span>

                <strong>
                  {selectedMonitor.response_time_ms !==
                  undefined
                    ? `${selectedMonitor.response_time_ms} ms`
                    : '—'}
                </strong>
              </div>

              <div>
                <span>{selectedMonitor.monitor_type === 'ssl' ? 'Certificate' : selectedMonitor.monitor_type === 'port' ? 'Ports' : 'HTTP Code'}</span>

                <strong>
                  {selectedMonitor.monitor_type === 'port'
                    ? `${selectedMonitor.config?.ports?.length ?? 0} configured`
                    : selectedMonitor.status_code ?? '—'}
                </strong>
              </div>

              <div>
                <span>Last Checked</span>

                <strong>
                  {formatLastChecked(
                    selectedMonitor.last_checked_at,
                  )}
                </strong>
              </div>
            </div>

            {selectedMonitor.monitor_type === 'ssl' && (
              <section className="ssl-details">
                <h3>SSL Certificate</h3>
                {sslLoading && <div className="message">Checking certificate...</div>}
                {!sslLoading && sslDetails && (
                  <div className="ssl-detail-grid">
                    <div><span>Status</span><strong className={`status-text ${sslDetails.status}`}>{sslDetails.status.toUpperCase()}</strong></div>
                    <div><span>Certificate Subject</span><strong>{sslDetails.subject || '—'}</strong></div>
                    <div><span>Issuer</span><strong>{sslDetails.issuer || '—'}</strong></div>
                    <div><span>Valid From</span><strong>{sslDetails.not_before ? new Date(sslDetails.not_before).toLocaleString() : '—'}</strong></div>
                    <div><span>Expires</span><strong>{sslDetails.not_after ? new Date(sslDetails.not_after).toLocaleString() : '—'}</strong></div>
                    <div><span>Days Remaining</span><strong className={sslDetails.days_remaining <= 30 ? 'ssl-expiry-warning' : ''}>{sslDetails.days_remaining <= 0 ? 'Expired' : `${sslDetails.days_remaining} days`}</strong></div>
                    <div><span>Certificate Valid</span><strong className={sslDetails.certificate_valid ? 'status-text up' : 'status-text down'}>{sslDetails.certificate_valid ? 'YES' : 'NO'}</strong></div>
                    <div><span>Response Time</span><strong>{sslDetails.response_time_ms} ms</strong></div>
                  </div>
                )}
                {!sslLoading && !sslDetails && <div className="message error">Certificate details unavailable.</div>}
              </section>
            )}

            {selectedMonitor.monitor_type === 'dns' && (
              <section className="ssl-details">
                <h3>DNS Records</h3>
                <div className="ssl-detail-grid">
                  <div><span>Record Type</span><strong>{selectedMonitor.config?.record_type || dnsDetails?.record_type || '—'}</strong></div>
                  <div><span>Expected Value(s)</span><strong>{selectedMonitor.config?.expected_values?.join(', ') || 'Any successful record'}</strong></div>
                  <div><span>Latest Records</span><strong>{dnsLoading ? 'Loading...' : dnsDetails?.records?.join(', ') || '—'}</strong></div>
                  <div><span>Lookup Result</span><strong>{dnsDetails?.message || '—'}</strong></div>
                </div>
              </section>
            )}

            {selectedMonitor.monitor_type === 'port' && (
              <section className="ssl-details">
                <h3>Configured Ports</h3>
                <div className="ssl-detail-grid">
                  <div><span>Configured</span><strong>{selectedMonitor.config?.ports?.join(', ') || '—'}</strong></div>
                  <div><span>Reachable</span><strong>{portLoading ? 'Checking...' : portDetails ? `${portDetails.reachable}/${portDetails.total}` : '—'}</strong></div>
                  <div><span>Latest Check</span><strong>{portDetails?.message || '—'}</strong></div>
                  <div><span>Check Duration</span><strong>{portDetails ? `${portDetails.response_time_ms} ms` : '—'}</strong></div>
                  {(portDetails?.ports || []).map((item) => (
                    <div key={item.port}>
                      <span>Port {item.port}</span>
                      <strong className={`status-text ${item.status}`}>{item.status.toUpperCase()} · {item.response_time_ms} ms · {item.message}</strong>
                    </div>
                  ))}
                </div>
                <small>Per-port results come from a fresh check run when this panel opens; stored history stays on the scheduled checks.</small>
              </section>
            )}

<div className="monitor-insights">
  <div className="insight-card">
    <span>Uptime</span>

    <strong>
      {statsLoading
        ? '...'
        : stats
          ? `${stats.uptime_percentage.toFixed(2)}%`
          : '—'}
    </strong>
  </div>

  <div className="insight-card">
    <span>Average Response</span>

    <strong>
      {statsLoading
        ? '...'
        : stats
          ? `${stats.average_response_ms.toFixed(2)} ms`
          : '—'}
    </strong>
  </div>

  <div className="insight-card">
    <span>Total Checks</span>

    <strong>
      {statsLoading
        ? '...'
        : stats
          ? stats.total_checks
          : '—'}
    </strong>
  </div>

  <div className="insight-card">
    <span>Successful Checks</span>

    <strong>
      {statsLoading
        ? '...'
        : stats
          ? stats.up_checks
          : '—'}
    </strong>
  </div>
</div>
            <div className="detail-actions">
              <button
                type="button"
                onClick={() =>
                  toggleMonitor(selectedMonitor)
                }
              >
                {selectedMonitor.enabled
                  ? 'Pause Monitor'
                  : 'Resume Monitor'}
              </button>

              <button
                type="button"
                onClick={() => {
                  setEditingMonitor(
                    selectedMonitor,
                  )
                  setEditPortsInput(
                    (selectedMonitor.config?.ports || []).join(
                      ', ',
                    ),
                  )
                  setEditError('')
                }}
              >
                Edit
              </button>

              <button
                type="button"
                className="danger-button"
                disabled={deletingMonitor}
                onClick={() =>
                  deleteMonitor(selectedMonitor)
                }
              >
                {deletingMonitor
                  ? 'Deleting...'
                  : 'Delete'}
              </button>
            </div>

<div className="range-selector">
  <span>Time Range</span>

  <div className="range-buttons">
    {(
      [
        ['1h', '1H'],
        ['24h', '24H'],
        ['7d', '7D'],
        ['30d', '30D'],
      ] as [TimeRange, string][]
    ).map(([value, label]) => (
      <button
        key={value}
        type="button"
        className={
          selectedRange === value
            ? 'range-button active'
            : 'range-button'
        }
        disabled={
          statsLoading || chartLoading
        }
        onClick={() => changeRange(value)}
      >
        {label}
      </button>
    ))}
  </div>
</div>
<div className="response-chart-section">
  <div className="chart-header">
    <div>
      <h3>Response Time Trend</h3>
      <p>{chartPoints.length} aggregated data points</p>
    </div>

    {chartPoints.length > 0 && (
      <div className="chart-stats">
        <span>
          Min
          <strong>
            {responseChart.min.toFixed(0)} ms
          </strong>
        </span>

        <span>
          Avg
          <strong>
            {responseChart.average.toFixed(0)} ms
          </strong>
        </span>

        <span>
          Max
          <strong>
            {responseChart.max.toFixed(0)} ms
          </strong>
        </span>
      </div>
    )}
  </div>

  {chartLoading && (
    <div className="message">
      Loading response trend...
    </div>
  )}

  {!chartLoading && chartPoints.length === 0 && (
    <div className="message">
      No response-time data available.
    </div>
  )}

  {!chartLoading && chartPoints.length > 0 && (
    <div className="response-chart-wrapper">
      <svg
        className="response-chart"
        viewBox="0 0 700 180"
        preserveAspectRatio="none"
        role="img"
        aria-label="Response time trend"
      >
        <line
          className="chart-grid-line"
          x1="0"
          x2="700"
          y1="15"
          y2="15"
        />

        <line
          className="chart-grid-line"
          x1="0"
          x2="700"
          y1="90"
          y2="90"
        />

        <line
          className="chart-grid-line"
          x1="0"
          x2="700"
          y1="165"
          y2="165"
        />

        <polyline
          className="response-chart-line"
          points={responseChart.points}
          fill="none"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
    </div>
  )}

  {!chartLoading && chartPoints.length > 0 && (
    <div className="chart-axis">
      <span>
        {new Date(
          chartPoints[0].checked_at,
        ).toLocaleTimeString()}
      </span>

	<span>Earlier → Latest</span>
      <span>
        {new Date(
          chartPoints[chartPoints.length - 1].checked_at,
        ).toLocaleTimeString()}
      </span>
    </div>
  )}
</div>
            <div className="history-section">
              <h3>Recent Checks</h3>

              {resultsLoading && (
                <div className="message">
                  Loading history...
                </div>
              )}

              {!resultsLoading &&
                results.length === 0 && (
                  <div className="message">
                    No monitoring history
                    available.
                  </div>
                )}

              {!resultsLoading &&
                results.length > 0 && (
                  <div className="history-table">
                    <div className="history-row history-header">
                      <span>Status</span>
                      <span>Response</span>
                      <span>Code</span>
                      <span>Checked At</span>
                    </div>

                    {results.map((result) => (
                      <div
                        className="history-row"
                        key={result.id}
                      >
                        <span
                          className={`status-text ${result.status}`}
                        >
                          {result.status.toUpperCase()}
                        </span>

                        <span>
                          {result.response_time_ms}{' '}
                          ms
                        </span>

                        <span>
                          {result.status_code ?? '—'}
                        </span>

                        <span>
                          {new Date(
                            result.checked_at,
                          ).toLocaleString()}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
            </div>
          </div>
        </div>
      )}

      {showGroupManager && (
        <div className="modal-backdrop">
          <div className="modal-card group-manager-modal">
            <div className="modal-header">
              <div>
                <h2>Monitor Groups</h2>
                <p>
                  Groups organize monitors without changing how they are
                  checked or alerted.
                </p>
              </div>

              <button
                className="close-button"
                type="button"
                onClick={closeGroupManager}
              >
                ×
              </button>
            </div>

            <form
              className="group-create-form"
              onSubmit={handleCreateGroup}
            >
              <label>
                New group name
                <input
                  type="text"
                  value={newGroupName}
                  onChange={(event) =>
                    setNewGroupName(event.target.value)
                  }
                  placeholder="Production"
                  required
                />
              </label>

              <button
                className="primary-button"
                type="submit"
                disabled={groupSaving}
              >
                {groupSaving ? 'Adding...' : 'Add Group'}
              </button>
            </form>

            {groupFormError && (
              <div className="form-error">{groupFormError}</div>
            )}

            <p className="group-manager-note">
              Group names are unique regardless of letter case and may be up
              to 100 characters.
            </p>

            <div className="group-list">
              {groups.length === 0 && (
                <div className="message">
                  No groups yet. Monitors remain valid and ungrouped.
                </div>
              )}

              {groups.map((group) => (
                <div className="group-row" key={group.id}>
                  {editingGroup?.id === group.id ? (
                    <form
                      className="group-rename-form"
                      onSubmit={handleRenameGroup}
                    >
                      <input
                        type="text"
                        value={editGroupName}
                        onChange={(event) =>
                          setEditGroupName(event.target.value)
                        }
                        autoFocus
                        required
                      />

                      {editGroupError && (
                        <div className="form-error">
                          {editGroupError}
                        </div>
                      )}

                      <div className="group-row-actions">
                        <button
                          type="button"
                          disabled={editGroupSaving}
                          onClick={() => {
                            setEditingGroup(null)
                            setEditGroupName('')
                            setEditGroupError('')
                          }}
                        >
                          Cancel
                        </button>
                        <button
                          className="primary-button"
                          type="submit"
                          disabled={editGroupSaving}
                        >
                          {editGroupSaving ? 'Saving...' : 'Save'}
                        </button>
                      </div>
                    </form>
                  ) : (
                    <>
                      <strong>{group.name}</strong>
                      <div className="group-row-actions">
                        <button
                          type="button"
                          onClick={() => {
                            setEditingGroup(group)
                            setEditGroupName(group.name)
                            setEditGroupError('')
                          }}
                        >
                          Rename
                        </button>
                        <button
                          className="danger-button"
                          type="button"
                          onClick={() => {
                            setDeletingGroup(group)
                            setDeleteGroupError('')
                          }}
                        >
                          Delete
                        </button>
                      </div>
                    </>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {deletingGroup && (
        <div className="modal-backdrop">
          <div className="modal-card confirm-card">
            <div className="modal-header">
              <div>
                <h2>Delete "{deletingGroup.name}"?</h2>
                <p>
                  Deleting this group does not delete its monitors. The
                  monitors will become ungrouped.
                </p>
              </div>
            </div>

            {deleteGroupError && (
              <div className="form-error">{deleteGroupError}</div>
            )}

            <div className="form-actions">
              <button
                type="button"
                disabled={deleteGroupSaving}
                onClick={() => {
                  setDeletingGroup(null)
                  setDeleteGroupError('')
                }}
              >
                Cancel
              </button>
              <button
                className="danger-button"
                type="button"
                disabled={deleteGroupSaving}
                onClick={handleDeleteGroup}
              >
                {deleteGroupSaving ? 'Deleting...' : 'Delete Group'}
              </button>
            </div>
          </div>
        </div>
      )}

      {editingMonitor && (
        <div className="modal-backdrop">
          <div className="modal-card">
            <div className="modal-header">
              <div>
                <h2>Edit Monitor</h2>
                <p>
                  Update monitor configuration.
                </p>
              </div>

              <button
                className="close-button"
                type="button"
                onClick={() => {
                  if (!editSaving) {
                    setEditingMonitor(null)
                    setEditError('')
                  }
                }}
              >
                ×
              </button>
            </div>

            <form
              className="monitor-form"
              onSubmit={handleEditMonitor}
            >
              <label>
                Monitor name

                <input
                  type="text"
                  value={editingMonitor.name}
                  onChange={(event) =>
                    setEditingMonitor({
                      ...editingMonitor,
                      name: event.target.value,
                    })
                  }
                  required
                />
              </label>

              <label>
                Monitor type

                <select
                  value={
                    editingMonitor.monitor_type
                  }
                  onChange={(event) =>
                    setEditingMonitor({
                      ...editingMonitor,
                      monitor_type:
                        event.target.value,
                    })
                  }
                >
                  <option value="https">
                    HTTPS
                  </option>
                  <option value="http">
                    HTTP
                  </option>
                  <option value="tcp">
                    TCP
                  </option>
                  <option value="ssl">
                    SSL Certificate
                  </option>
                  <option value="ping">
                    Ping
                  </option>
                  <option value="dns">
                    DNS
                  </option>
                  <option value="port">
                    Port
                  </option>
                </select>
              </label>

              <label>
                Target

                <input
                  type="text"
                  value={editingMonitor.target}
                  onChange={(event) =>
                    setEditingMonitor({
                      ...editingMonitor,
                      target: event.target.value,
                    })
                  }
                  placeholder={editingMonitor.monitor_type === 'dns' ? 'example.com' : editingMonitor.monitor_type === 'port' ? 'example.com' : editingMonitor.monitor_type === 'ping' ? 'example.com or 192.168.1.1' : undefined}
                  required
                />
                {editingMonitor.monitor_type === 'dns' && <small>DNS name only — no protocol or port. Changes affect status on the next scheduled check.</small>}
                {editingMonitor.monitor_type === 'port' && <small>Host only — no protocol or port. Changes affect status on the next scheduled check.</small>}
              </label>

              <label>
                Group

                <select
                  value={editingMonitor.group_id ?? ''}
                  onChange={(event) =>
                    setEditingMonitor({
                      ...editingMonitor,
                      group_id:
                        event.target.value === ''
                          ? null
                          : Number(event.target.value),
                    })
                  }
                >
                  <option value="">Ungrouped</option>
                  {groups.map((group) => (
                    <option key={group.id} value={group.id}>
                      {group.name}
                    </option>
                  ))}
                </select>
              </label>

              {editingMonitor.monitor_type === 'port' && <>
                <label>Ports
                  <input type="text" value={editPortsInput} onChange={(event) => setEditPortsInput(event.target.value)} placeholder="22, 80, 443" />
                  <small>Enter up to 20 TCP ports separated by commas. Individual ports only — ranges are not supported. Changes affect status on the next scheduled check.</small>
                </label>
              </>}

              {editingMonitor.monitor_type === 'dns' && <>
                <label>Record Type<select value={editingMonitor.config?.record_type || 'A'} onChange={(event) => setEditingMonitor({...editingMonitor, config: {...editingMonitor.config, record_type: event.target.value}})}>
                  {['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT'].map((type) => <option key={type} value={type}>{type}</option>)}
                </select></label>
                <label>Expected Value <input type="text" value={(editingMonitor.config?.expected_values || []).join(', ')} onChange={(event) => setEditingMonitor({...editingMonitor, config: {...editingMonitor.config, expected_values: event.target.value.split(',').map((value) => value.trim()).filter(Boolean)}})} placeholder="Optional — e.g. 203.0.113.10" /></label>
              </>}

              <div className="form-row">
                <label>
                  Check every

                  <select
                    value={
                      editingMonitor.interval_seconds
                    }
                    onChange={(event) =>
                      setEditingMonitor({
                        ...editingMonitor,
                        interval_seconds: Number(
                          event.target.value,
                        ),
                      })
                    }
                  >
                    <option value={30}>
                      30 seconds
                    </option>
                    <option value={60}>
                      1 minute
                    </option>
                    <option value={300}>
                      5 minutes
                    </option>
                    <option value={600}>
                      10 minutes
                    </option>
                  </select>
                </label>

                <label>
                  Timeout

                  <select
                    value={
                      editingMonitor.timeout_seconds
                    }
                    onChange={(event) =>
                      setEditingMonitor({
                        ...editingMonitor,
                        timeout_seconds: Number(
                          event.target.value,
                        ),
                      })
                    }
                  >
                    <option value={5}>
                      5 seconds
                    </option>
                    <option value={10}>
                      10 seconds
                    </option>
                    <option value={15}>
                      15 seconds
                    </option>
                    <option value={30}>
                      30 seconds
                    </option>
                  </select>
                </label>
              </div>

              {editError && (
                <div className="form-error">
                  {editError}
                </div>
              )}

              <div className="form-actions">
                <button
                  type="button"
                  disabled={editSaving}
                  onClick={() => {
                    setEditingMonitor(null)
                    setEditError('')
                  }}
                >
                  Cancel
                </button>

                <button
                  className="primary-button"
                  type="submit"
                  disabled={editSaving}
                >
                  {editSaving
                    ? 'Saving...'
                    : 'Save Changes'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
