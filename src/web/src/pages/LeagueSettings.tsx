import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import type { LeagueSettings, PayoutEntry, BonusType } from '../types/league'
import './LeagueSettings.css'

const BONUS_TYPE_OPTIONS: { value: BonusType; label: string }[] = [
  { value: 'weekly_high_score', label: 'Weekly High Score' },
  { value: 'score_threshold',   label: 'Score Threshold' },
]

function defaultLabelForBonusType(bt: BonusType): string {
  return BONUS_TYPE_OPTIONS.find(o => o.value === bt)?.label ?? bt
}

function ordinalLabel(n: number): string {
  const s = n % 100
  if (s >= 11 && s <= 13) return `${n}th Place`
  switch (n % 10) {
    case 1: return `${n}st Place`
    case 2: return `${n}nd Place`
    case 3: return `${n}rd Place`
    default: return `${n}th Place`
  }
}

function normalizeEntries(entries: PayoutEntry[]): PayoutEntry[] {
  return entries.map((e, i) => ({
    type: e.type ?? 'placement',
    bonus_type: e.type === 'weekly' ? (e.bonus_type ?? 'weekly_high_score') : undefined,
    label: e.label || (e.type === 'weekly' ? defaultLabelForBonusType(e.bonus_type ?? 'weekly_high_score') : ordinalLabel(e.place ?? i + 1)),
    place: e.place,
    amount_cents: e.amount_cents,
    weeks: e.weeks,
    criteria: e.criteria,
  }))
}

function totalPayout(rows: PayoutEntry[]): number {
  return rows.reduce((s, r) => {
    if (r.type === 'weekly') return s + r.amount_cents * (r.weeks ?? 0)
    return s + r.amount_cents
  }, 0)
}

const formatDollars = (cents: number) => `$${Math.round(cents / 100)}`

export default function LeagueSettingsPage() {
  const { leagueId } = useParams<{ leagueId: string }>()
  const { isAuthenticated, token } = useWallet()
  const navigate = useNavigate()

  const [settings, setSettings] = useState<LeagueSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [entryFeeCents, setEntryFeeCents] = useState(0)
  const [payoutRows, setPayoutRows] = useState<PayoutEntry[]>([])

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!token || !leagueId) return
    const fetchSettings = async () => {
      try {
        const res = await fetch(`/api/leagues/${leagueId}/settings`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) throw new Error(`Failed to load settings (${res.status})`)
        const data: LeagueSettings = await res.json()
        setSettings(data)
        setEntryFeeCents(data.entry_fee_cents)
        setPayoutRows(normalizeEntries(data.payout_structure ?? []))
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Something went wrong')
      } finally {
        setLoading(false)
      }
    }
    fetchSettings()
  }, [token, leagueId])

  const placements = payoutRows.filter((r) => r.type === 'placement')
  const weekly = payoutRows.filter((r) => r.type === 'weekly')
  const totalPayoutCents = totalPayout(payoutRows)
  const potentialPotCents = entryFeeCents * (settings?.total_rosters ?? 0)

  const updateRow = (index: number, patch: Partial<PayoutEntry>) =>
    setPayoutRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)))

  const removeRow = (index: number) =>
    setPayoutRows((prev) => prev.filter((_, i) => i !== index))

  const handleAddPlacement = () => {
    const maxPlace = placements.reduce((m, r) => Math.max(m, r.place ?? 0), 0)
    const nextPlace = maxPlace + 1
    setPayoutRows((prev) => [
      ...prev,
      { type: 'placement', label: ordinalLabel(nextPlace), place: nextPlace, amount_cents: 0 },
    ])
  }

  const handleAddWeekly = () => {
    setPayoutRows((prev) => [
      ...prev,
      { type: 'weekly', bonus_type: 'weekly_high_score', label: 'Weekly High Score', amount_cents: 0, weeks: 14 },
    ])
  }

  const updateWeekly = (i: number, patch: Partial<PayoutEntry>) =>
    setPayoutRows((prev) => prev.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!token || !leagueId) return
    setSaving(true)
    setSaveError(null)
    setSaveSuccess(false)
    try {
      const res = await fetch(`/api/leagues/${leagueId}/settings`, {
        method: 'PUT',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ entry_fee_cents: entryFeeCents, payout_structure: payoutRows }),
      })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `Error ${res.status}`)
      }
      const updated: LeagueSettings = await res.json()
      setSettings(updated)
      setEntryFeeCents(updated.entry_fee_cents)
      setPayoutRows(normalizeEntries(updated.payout_structure ?? []))
      setSaveSuccess(true)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  if (!isAuthenticated) return null

  return (
    <div className="settings-page">
      <div className="settings-header">
        <button className="btn-back" onClick={() => navigate('/leagues')}>
          ← Back to Leagues
        </button>
        <h1>League Settings</h1>
      </div>

      {loading && <p className="settings-status">Loading settings...</p>}
      {error && <p className="settings-status settings-error">{error}</p>}

      {!loading && !error && settings && (
        <>
          {settings.is_commissioner ? (
            <form className="settings-form" onSubmit={handleSave}>
              {/* Entry Fee */}
              <section className="settings-section">
                <h2>Entry Fee</h2>
                <div className="settings-field">
                  <label htmlFor="entry-fee">Entry Fee per Team</label>
                  <div className="input-dollar">
                    <span>$</span>
                    <input
                      id="entry-fee"
                      type="number"
                      min="0"
                      step="1"
                      value={entryFeeCents / 100}
                      onChange={(e) =>
                        setEntryFeeCents((parseInt(e.target.value || '0', 10) || 0) * 100)
                      }
                    />
                  </div>
                </div>
              </section>

              {/* Season Payouts */}
              <section className="settings-section">
                <h2>Season Payouts</h2>
                <p className="settings-hint">End-of-season prizes based on final standings.</p>
                {placements.length === 0 ? (
                  <p className="settings-empty">No season payouts defined yet.</p>
                ) : (
                  <table className="payout-table">
                    <thead>
                      <tr>
                        <th>Label</th>
                        <th>Amount</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {payoutRows.map((row, i) => {
                        if (row.type !== 'placement') return null
                        return (
                          <tr key={i} className="payout-row">
                            <td>
                              <input
                                className="input-label"
                                type="text"
                                value={row.label}
                                onChange={(e) => updateRow(i, { label: e.target.value })}
                              />
                            </td>
                            <td>
                              <div className="input-dollar">
                                <span>$</span>
                                <input
                                  type="number"
                                  min="0"
                                  step="1"
                                  value={row.amount_cents / 100}
                                  onChange={(e) =>
                                    updateRow(i, {
                                      amount_cents: (parseInt(e.target.value || '0', 10) || 0) * 100,
                                    })
                                  }
                                />
                              </div>
                            </td>
                            <td>
                              <button type="button" className="btn-remove" onClick={() => removeRow(i)}>
                                Remove
                              </button>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                )}
                <button type="button" className="btn-add-place" onClick={handleAddPlacement}>
                  + Add Season Payout
                </button>
              </section>

              {/* Recurring Bonuses */}
              <section className="settings-section">
                <h2>Recurring Bonuses</h2>
                <p className="settings-hint">Prizes awarded repeatedly throughout the season (e.g., weekly high score).</p>
                {weekly.length === 0 ? (
                  <p className="settings-empty">No recurring bonuses defined yet.</p>
                ) : (
                  <table className="payout-table payout-table--weekly">
                    <thead>
                      <tr>
                        <th>Bonus Type</th>
                        <th>Per Week</th>
                        <th>Weeks</th>
                        <th>Total</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {payoutRows.map((row, i) => {
                        if (row.type !== 'weekly') return null
                        const rowTotal = row.amount_cents * (row.weeks ?? 0)
                        return (
                          <tr key={i} className="payout-row">
                            <td>
                              <select
                                value={row.bonus_type ?? 'weekly_high_score'}
                                onChange={e => {
                                  const bt = e.target.value as BonusType
                                  updateWeekly(i, {
                                    bonus_type: bt,
                                    label: defaultLabelForBonusType(bt),
                                    criteria: bt === 'score_threshold' ? { threshold: row.criteria?.threshold ?? 100 } : undefined,
                                  })
                                }}
                              >
                                {BONUS_TYPE_OPTIONS.map(o => (
                                  <option key={o.value} value={o.value}>{o.label}</option>
                                ))}
                              </select>
                              {row.bonus_type === 'score_threshold' && (
                                <input
                                  type="number"
                                  min={0}
                                  placeholder="Min pts"
                                  value={row.criteria?.threshold ?? ''}
                                  onChange={e => updateWeekly(i, { criteria: { threshold: Number(e.target.value) } })}
                                />
                              )}
                            </td>
                            <td>
                              <div className="input-dollar">
                                <span>$</span>
                                <input
                                  type="number"
                                  min="0"
                                  step="1"
                                  value={row.amount_cents / 100}
                                  onChange={(e) =>
                                    updateRow(i, {
                                      amount_cents: (parseInt(e.target.value || '0', 10) || 0) * 100,
                                    })
                                  }
                                />
                              </div>
                            </td>
                            <td>
                              <input
                                className="input-weeks"
                                type="number"
                                min="1"
                                step="1"
                                value={row.weeks ?? 0}
                                onChange={(e) =>
                                  updateRow(i, { weeks: parseInt(e.target.value || '0', 10) || 0 })
                                }
                              />
                            </td>
                            <td className="payout-row-total">{formatDollars(rowTotal)}</td>
                            <td>
                              <button type="button" className="btn-remove" onClick={() => removeRow(i)}>
                                Remove
                              </button>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                )}
                <button type="button" className="btn-add-place" onClick={handleAddWeekly}>
                  + Add Recurring Bonus
                </button>
              </section>

              {/* Totals + Save */}
              <div className="payout-totals">
                <div className="payout-total-row">
                  <span>Total Payout:</span>
                  <span>{formatDollars(totalPayoutCents)}</span>
                </div>
                <div className="payout-total-row">
                  <span>Entry Pool ({settings.total_rosters} teams):</span>
                  <span>{formatDollars(potentialPotCents)}</span>
                </div>
              </div>

              <div className="settings-actions">
                {saveError && <p className="settings-error">{saveError}</p>}
                {saveSuccess && <p className="settings-success">Settings saved!</p>}
                <button type="submit" className="btn-primary" disabled={saving}>
                  {saving ? 'Saving…' : 'Save Settings'}
                </button>
              </div>
            </form>
          ) : (
            /* Read-only view */
            <div className="settings-readonly">
              <p className="readonly-notice">Only the commissioner can edit settings.</p>

              <section className="settings-section">
                <h2>Entry Fee</h2>
                <p className="readonly-value">{formatDollars(settings.entry_fee_cents)} per team</p>
              </section>

              {(() => {
                const normalized = normalizeEntries(settings.payout_structure ?? [])
                const roPlacement = normalized.filter((r) => r.type === 'placement')
                const roWeekly = normalized.filter((r) => r.type === 'weekly')
                const roTotal = totalPayout(normalized)
                return (
                  <>
                    <section className="settings-section">
                      <h2>Season Payouts</h2>
                      {roPlacement.length === 0 ? (
                        <p className="settings-empty">No season payouts defined.</p>
                      ) : (
                        <table className="payout-table">
                          <thead><tr><th>Label</th><th>Amount</th></tr></thead>
                          <tbody>
                            {roPlacement.map((row, i) => (
                              <tr key={i} className="payout-row">
                                <td>{row.label}</td>
                                <td>{formatDollars(row.amount_cents)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      )}
                    </section>

                    <section className="settings-section">
                      <h2>Recurring Bonuses</h2>
                      {roWeekly.length === 0 ? (
                        <p className="settings-empty">No recurring bonuses defined.</p>
                      ) : (
                        <table className="payout-table payout-table--weekly">
                          <thead>
                            <tr><th>Bonus Type</th><th>Per Week</th><th>Weeks</th><th>Total</th></tr>
                          </thead>
                          <tbody>
                            {roWeekly.map((row, i) => (
                              <tr key={i} className="payout-row">
                                <td>
                                  {row.label}
                                  {row.bonus_type === 'score_threshold' && row.criteria?.threshold != null
                                    ? ` (≥ ${row.criteria.threshold} pts)` : ''}
                                </td>
                                <td>{formatDollars(row.amount_cents)}</td>
                                <td>{row.weeks ?? 0}</td>
                                <td>{formatDollars(row.amount_cents * (row.weeks ?? 0))}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      )}
                    </section>

                    <div className="payout-totals">
                      <div className="payout-total-row">
                        <span>Total Payout:</span>
                        <span>{formatDollars(roTotal)}</span>
                      </div>
                      <div className="payout-total-row">
                        <span>Entry Pool ({settings.total_rosters} teams):</span>
                        <span>{formatDollars(settings.entry_fee_cents * settings.total_rosters)}</span>
                      </div>
                    </div>
                  </>
                )
              })()}
            </div>
          )}
        </>
      )}
    </div>
  )
}
