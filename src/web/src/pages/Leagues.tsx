import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import type { League } from '../types/league'
import ImportLeagueModal from '../components/ImportLeagueModal'
import './Leagues.css'

export default function Leagues() {
  const { isAuthenticated, token } = useWallet()
  const navigate = useNavigate()
  const [leagues, setLeagues] = useState<League[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showModal, setShowModal] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [confirmId, setConfirmId] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/')
    }
  }, [isAuthenticated, navigate])

  const handleDelete = async (leagueId: string) => {
    if (confirmId !== leagueId) {
      setConfirmId(leagueId)
      return
    }
    setDeletingId(leagueId)
    setConfirmId(null)
    try {
      const res = await fetch(`/api/leagues/${leagueId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Failed to remove league (${res.status})`)
      setLeagues((prev) => prev.filter((l) => l.id !== leagueId))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove league')
    } finally {
      setDeletingId(null)
    }
  }

  const fetchLeagues = async () => {
    if (!token) return
    try {
      const res = await fetch('/api/leagues/', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        throw new Error(`Failed to load leagues (${res.status})`)
      }
      const data: League[] = await res.json()
      setLeagues(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (token) {
      fetchLeagues()
    }
  }, [token])

  if (!isAuthenticated) return null

  return (
    <div className="leagues-page">
      <div className="leagues-header">
        <h1>My Leagues</h1>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          Import League
        </button>
      </div>

      {loading && (
        <p className="leagues-status">Loading leagues...</p>
      )}

      {error && (
        <p className="leagues-status leagues-error">{error}</p>
      )}

      {!loading && !error && leagues.length === 0 && (
        <div className="empty-state">
          <p>No leagues imported yet.</p>
          <button
            className="btn-primary"
            style={{ marginTop: '1rem' }}
            onClick={() => setShowModal(true)}
          >
            Import your first league
          </button>
        </div>
      )}

      {!loading && !error && leagues.length > 0 && (
        <div className="leagues-list">
          {leagues.map((league) => (
            <div key={league.id} className={`league-card${league.cancelled_at ? ' league-card--cancelled' : ''}`}>
              {league.cancelled_at && <span className="league-cancelled-label">Cancelled</span>}
              <div className="league-card-header">
                <h2 className="league-name">{league.name}</h2>
                <span className="platform-badge">{league.platform}</span>
                {league.is_commissioner && (
                  <span className="commissioner-badge">Commissioner</span>
                )}
              </div>
              <div className="league-stats">
                <div className="league-stat">
                  <span className="stat-label">Season</span>
                  <span className="stat-value">{league.season}</span>
                </div>
                <div className="league-stat">
                  <span className="stat-label">Sport</span>
                  <span className="stat-value">{league.sport.toUpperCase()}</span>
                </div>
                <div className="league-stat">
                  <span className="stat-label">Teams</span>
                  <span className="stat-value">{league.total_rosters}</span>
                </div>
                {league.scoring_type && (
                  <div className="league-stat">
                    <span className="stat-label">Scoring</span>
                    <span className="stat-value">{league.scoring_type}</span>
                  </div>
                )}
                <div className="league-stat">
                  <span className="stat-label">Status</span>
                  <span className={`stat-value status-${league.status}`}>{league.status}</span>
                </div>
              </div>
              <div className="league-card-footer">
                <Link to={`/leagues/${league.id}`} className="btn-view">
                  View
                </Link>
                <Link to={`/leagues/${league.id}/settings`} className="btn-settings">
                  Settings
                </Link>
                <button
                  className={`btn-remove-league ${confirmId === league.id ? 'btn-remove-league--confirm' : ''}`}
                  disabled={deletingId === league.id}
                  onClick={() => handleDelete(league.id)}
                >
                  {deletingId === league.id
                    ? 'Removing…'
                    : confirmId === league.id
                    ? 'Confirm Remove'
                    : 'Remove'}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <ImportLeagueModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        onImported={() => {
          setLoading(true)
          fetchLeagues()
        }}
        existingLeagues={leagues}
      />
    </div>
  )
}
