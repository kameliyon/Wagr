import { useEffect, useState } from 'react'
import { useWallet } from '../hooks/useWallet'
import type { PlatformLeague, PlatformProfile, League } from '../types/league'
import './ImportLeagueModal.css'

interface Props {
  isOpen: boolean
  onClose: () => void
  onImported: () => void
  existingLeagues: League[]
}

export default function ImportLeagueModal({ isOpen, onClose, onImported, existingLeagues }: Props) {
  const { token } = useWallet()
  const [step, setStep] = useState<1 | 2 | 3>(1)
  const [username, setUsername] = useState('')
  const [profile, setProfile] = useState<PlatformProfile | null>(null)
  const [availableLeagues, setAvailableLeagues] = useState<PlatformLeague[]>([])
  const [selectedLeague, setSelectedLeague] = useState<PlatformLeague | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Reset state when modal opens
  useEffect(() => {
    if (isOpen) {
      setStep(1)
      setUsername('')
      setProfile(null)
      setAvailableLeagues([])
      setSelectedLeague(null)
      setError(null)
    }
  }, [isOpen])

  if (!isOpen) return null

  const importedIds = new Set(existingLeagues.map((l) => l.platform_league_id))

  async function handleLinkPlatform(e: React.FormEvent) {
    e.preventDefault()
    if (!username.trim()) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/leagues/link-platform', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ platform: 'sleeper', platform_username: username.trim() }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error ?? `Request failed (${res.status})`)
      }
      const data: PlatformProfile = await res.json()
      setProfile(data)

      // Fetch leagues for this platform user
      const leaguesRes = await fetch(
        `/api/fantasy/sleeper/user/${data.platform_user_id}/leagues?sport=nfl&season=2025`,
        { headers: { Authorization: `Bearer ${token}` } }
      )
      if (!leaguesRes.ok) {
        throw new Error(`Failed to load leagues (${leaguesRes.status})`)
      }
      const leagues: PlatformLeague[] = await leaguesRes.json()
      setAvailableLeagues(leagues ?? [])
      setStep(2)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  function handleSelectLeague(league: PlatformLeague) {
    if (importedIds.has(league.platform_league_id)) return
    setSelectedLeague(league)
    setStep(3)
  }

  async function handleImport() {
    if (!selectedLeague) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/leagues/import', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ platform: 'sleeper', platform_league_id: selectedLeague.platform_league_id }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error ?? `Import failed (${res.status})`)
      }
      onImported()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div className="modal-header">
          <h2>Import League</h2>
          <button className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        {/* Step indicator */}
        <div className="modal-steps">
          <div className={`modal-step ${step === 1 ? 'active' : 'done'}`}>
            <span className="step-dot">{step > 1 ? '✓' : '1'}</span>
            <span>Username</span>
          </div>
          <div className="step-divider" />
          <div className={`modal-step ${step === 2 ? 'active' : step > 2 ? 'done' : ''}`}>
            <span className="step-dot">{step > 2 ? '✓' : '2'}</span>
            <span>Pick League</span>
          </div>
          <div className="step-divider" />
          <div className={`modal-step ${step === 3 ? 'active' : ''}`}>
            <span className="step-dot">3</span>
            <span>Confirm</span>
          </div>
        </div>

        {/* Step 1 — Enter username */}
        {step === 1 && (
          <form className="modal-body" onSubmit={handleLinkPlatform}>
            <p>Enter your Sleeper username to connect your account.</p>
            <div className="modal-field">
              <label htmlFor="sleeper-username">Sleeper Username</label>
              <input
                id="sleeper-username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="your_sleeper_name"
                autoFocus
                autoComplete="off"
              />
            </div>
            {error && <p className="modal-error">{error}</p>}
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={onClose}>
                Cancel
              </button>
              <button type="submit" className="btn-primary" disabled={loading || !username.trim()}>
                {loading ? 'Connecting...' : 'Connect'}
              </button>
            </div>
          </form>
        )}

        {/* Step 2 — Pick a league */}
        {step === 2 && (
          <div className="modal-body">
            <p>
              Select a league to import from <strong>{profile?.platform_username}</strong>'s
              2024 NFL season.
            </p>
            {availableLeagues.length === 0 ? (
              <p>No leagues found for this account.</p>
            ) : (
              <div className="league-options">
                {availableLeagues.map((league) => {
                  const alreadyImported = importedIds.has(league.platform_league_id)
                  return (
                    <button
                      key={league.platform_league_id}
                      className={`league-option${alreadyImported ? ' disabled' : ''}`}
                      onClick={() => handleSelectLeague(league)}
                      disabled={alreadyImported}
                    >
                      <div className="league-option-name">{league.name}</div>
                      <div className="league-option-meta">
                        <span>{league.season} {league.sport.toUpperCase()}</span>
                        <span>{league.total_rosters} teams</span>
                        {league.scoring_type && <span>{league.scoring_type}</span>}
                        <span>{league.status}</span>
                        {alreadyImported && <span className="imported-badge">Imported</span>}
                      </div>
                    </button>
                  )
                })}
              </div>
            )}
            {error && <p className="modal-error">{error}</p>}
            <div className="modal-actions">
              <button className="btn-secondary" onClick={() => setStep(1)}>
                Back
              </button>
            </div>
          </div>
        )}

        {/* Step 3 — Confirm */}
        {step === 3 && selectedLeague && (
          <div className="modal-body">
            <p>Review the details before importing this league.</p>
            <div className="confirm-summary">
              <div className="confirm-row">
                <span className="confirm-label">League</span>
                <span className="confirm-value">{selectedLeague.name}</span>
              </div>
              <div className="confirm-row">
                <span className="confirm-label">Platform</span>
                <span className="confirm-value">Sleeper</span>
              </div>
              <div className="confirm-row">
                <span className="confirm-label">Season</span>
                <span className="confirm-value">{selectedLeague.season}</span>
              </div>
              <div className="confirm-row">
                <span className="confirm-label">Sport</span>
                <span className="confirm-value">{selectedLeague.sport.toUpperCase()}</span>
              </div>
              <div className="confirm-row">
                <span className="confirm-label">Teams</span>
                <span className="confirm-value">{selectedLeague.total_rosters}</span>
              </div>
              {selectedLeague.scoring_type && (
                <div className="confirm-row">
                  <span className="confirm-label">Scoring</span>
                  <span className="confirm-value">{selectedLeague.scoring_type}</span>
                </div>
              )}
              <div className="confirm-row">
                <span className="confirm-label">Status</span>
                <span className="confirm-value">{selectedLeague.status}</span>
              </div>
            </div>
            {error && <p className="modal-error">{error}</p>}
            <div className="modal-actions">
              <button className="btn-secondary" onClick={() => setStep(2)}>
                Back
              </button>
              <button className="btn-primary" onClick={handleImport} disabled={loading}>
                {loading ? 'Importing...' : 'Import League'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
