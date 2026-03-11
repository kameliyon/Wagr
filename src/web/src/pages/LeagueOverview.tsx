import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import type { LeagueDetail, LeagueSettings, LeagueMember, PaymentToken, PayStubResponse } from '../types/league'
import './LeagueOverview.css'

const formatDollars = (cents: number) =>
  `$${(cents / 100).toFixed(2).replace(/\.00$/, '')}`

function initials(name: string): string {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('')
}

function Avatar({ member }: { member: LeagueMember }) {
  const [imgError, setImgError] = useState(false)
  if (member.avatar_url && !imgError) {
    return (
      <img
        className="team-avatar"
        src={member.avatar_url}
        alt={member.display_name}
        onError={() => setImgError(true)}
      />
    )
  }
  return <div className="team-avatar team-avatar--initials">{initials(member.display_name)}</div>
}

function TokenBadge({ token }: { token: PaymentToken | null }) {
  if (!token) return <span className="token-badge token-badge--none">—</span>
  return (
    <span className={`token-badge token-badge--${token}`}>
      {token.toUpperCase()}
    </span>
  )
}

export default function LeagueOverview() {
  const { leagueId } = useParams<{ leagueId: string }>()
  const { isAuthenticated, token, user } = useWallet()
  const navigate = useNavigate()

  const [detail, setDetail] = useState<LeagueDetail | null>(null)
  const [settings, setSettings] = useState<LeagueSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notMember, setNotMember] = useState(false)
  const [copied, setCopied] = useState(false)

  // Payment panel state
  const [payMessage, setPayMessage] = useState<string | null>(null)
  const [payError, setPayError] = useState<string | null>(null)
  const [payLoading, setPayLoading] = useState(false)
  const [tokenLoading, setTokenLoading] = useState(false)

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!token || !leagueId) return
    const fetchAll = async () => {
      try {
        const [detailRes, settingsRes] = await Promise.all([
          fetch(`/api/leagues/${leagueId}`, { headers: { Authorization: `Bearer ${token}` } }),
          fetch(`/api/leagues/${leagueId}/settings`, { headers: { Authorization: `Bearer ${token}` } }),
        ])
        if (detailRes.status === 403) {
          setNotMember(true)
          setLoading(false)
          return
        }
        if (!detailRes.ok) throw new Error(`Failed to load league (${detailRes.status})`)
        if (!settingsRes.ok) throw new Error(`Failed to load settings (${settingsRes.status})`)

        const [detailData, settingsData] = await Promise.all([
          detailRes.json() as Promise<LeagueDetail>,
          settingsRes.json() as Promise<LeagueSettings>,
        ])
        setDetail(detailData)
        setSettings(settingsData)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Something went wrong')
      } finally {
        setLoading(false)
      }
    }
    fetchAll()
  }, [token, leagueId])

  const handleCopyLink = async () => {
    const url = `${window.location.origin}/leagues/${leagueId}`
    await navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Select payment token — persists to DB immediately
  const handleSelectToken = async (selectedToken: PaymentToken) => {
    if (!token || !leagueId) return
    setTokenLoading(true)
    setPayError(null)
    try {
      const res = await fetch(`/api/leagues/${leagueId}/payment-token`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: selectedToken }),
      })
      if (!res.ok) throw new Error(`Failed to set token (${res.status})`)
      // Update local state so badge refreshes without full reload
      setDetail((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          members: prev.members.map((m) =>
            m.user_id === user?.id ? { ...m, payment_token: selectedToken } : m
          ),
        }
      })
    } catch (err) {
      setPayError(err instanceof Error ? err.message : 'Failed to set token')
    } finally {
      setTokenLoading(false)
    }
  }

  const handlePay = async () => {
    if (!token || !leagueId) return
    setPayLoading(true)
    setPayMessage(null)
    setPayError(null)
    try {
      const res = await fetch(`/api/leagues/${leagueId}/pay`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || `Error ${res.status}`)
      }
      const stub = await res.json() as PayStubResponse
      console.log('[WAGR Payment Stub]', stub)
      setPayMessage(stub.message)
    } catch (err) {
      setPayError(err instanceof Error ? err.message : 'Payment failed')
    } finally {
      setPayLoading(false)
    }
  }

  if (!isAuthenticated) return null

  if (loading) {
    return (
      <div className="overview-page">
        <p className="overview-status">Loading...</p>
      </div>
    )
  }

  if (notMember) {
    const platform = detail?.league.platform ?? 'your fantasy platform'
    return (
      <div className="overview-page">
        <div className="not-member-card">
          <h2>You're not a member of this league</h2>
          <p>Link your {platform} account to join and track your payment status.</p>
          <button className="btn-primary" onClick={() => navigate('/leagues')}>
            Link Account
          </button>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="overview-page">
        <p className="overview-status overview-error">{error}</p>
      </div>
    )
  }

  if (!detail || !settings) return null

  const { league, members } = detail
  const sorted = [...members].sort((a, b) => b.total_points - a.total_points)
  const prizePool = settings.entry_fee_cents * settings.total_rosters
  const isCommissioner = settings.is_commissioner

  // Find the current user's member record
  const myMember = members.find((m) => m.user_id === user?.id) ?? null

  return (
    <div className="overview-page">
      {/* Header */}
      <div className="overview-header">
        <div className="overview-header-left">
          <button className="btn-back" onClick={() => navigate('/leagues')}>
            ← Back to Leagues
          </button>
          <h1>{league.name}</h1>
          <div className="overview-badges">
            <span className="platform-badge">{league.platform}</span>
            <span className="platform-badge platform-badge--sport">{league.sport.toUpperCase()}</span>
            <span className="platform-badge platform-badge--season">{league.season}</span>
          </div>
        </div>
        <div className="overview-header-actions">
          <button className="copy-btn" onClick={handleCopyLink}>
            {copied ? '✓ Copied!' : 'Copy Invite Link'}
          </button>
          {isCommissioner && (
            <Link to={`/leagues/${leagueId}/settings`} className="btn-settings">
              Settings
            </Link>
          )}
        </div>
      </div>

      {/* Summary bar */}
      <div className="overview-stats">
        <div className="stat-chip">
          <span className="stat-chip-label">Teams</span>
          <span className="stat-chip-value">{settings.total_rosters}</span>
        </div>
        <div className="stat-chip">
          <span className="stat-chip-label">Entry Fee</span>
          <span className="stat-chip-value">{formatDollars(settings.entry_fee_cents)}</span>
        </div>
        <div className="stat-chip">
          <span className="stat-chip-label">Prize Pool</span>
          <span className="stat-chip-value">{formatDollars(prizePool)}</span>
        </div>
      </div>

      {/* Teams table */}
      <div className="settings-section">
        <h2>Teams</h2>
        {sorted.length === 0 ? (
          <p className="settings-empty">No members found.</p>
        ) : (
          <table className="payout-table teams-table">
            <thead>
              <tr>
                <th>Team</th>
                <th>Record</th>
                <th>Points</th>
                <th>Wagr</th>
                <th>Token</th>
                <th>Payment</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((member) => {
                const isConnected = member.user_id != null && member.wallet_address !== ''
                return (
                  <tr key={member.id} className="payout-row">
                    <td>
                      <div className="team-cell">
                        <Avatar member={member} />
                        <div className="team-names">
                          <span className="team-display-name">
                            {member.team_name || member.display_name}
                          </span>
                          {member.team_name && (
                            <span className="team-platform-name">{member.display_name}</span>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="team-record">
                      {member.wins}–{member.losses}
                      {member.ties > 0 ? `–${member.ties}` : ''}
                    </td>
                    <td className="team-points">{member.total_points.toFixed(2)}</td>
                    <td>
                      <span className={`badge ${isConnected ? 'badge--connected' : 'badge--not-connected'}`}>
                        {isConnected ? 'Connected' : 'Not Connected'}
                      </span>
                    </td>
                    <td>
                      <TokenBadge token={member.payment_token} />
                    </td>
                    <td>
                      <span className={`badge badge--${member.payment_status}`}>
                        {member.payment_status.charAt(0).toUpperCase() + member.payment_status.slice(1)}
                      </span>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Payment action panel — current user only */}
      {myMember && (
        <div className="payment-panel">
          <h2>Your Payment</h2>
          {myMember.payment_status === 'paid' ? (
            <div className="payment-paid">
              <span className="badge badge--paid">Paid</span>
              {myMember.transaction_hash && (
                <a
                  className="hashscan-link"
                  href={`https://hashscan.io/testnet/transaction/${myMember.transaction_hash}`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  View on HashScan
                </a>
              )}
            </div>
          ) : (
            <>
              {!myMember.wallet_address && (
                <p className="payment-warning">Connect your wallet to enable payment.</p>
              )}
              <div className="token-toggle">
                <span className="token-toggle-label">Pay with:</span>
                <button
                  className={`token-toggle-btn${myMember.payment_token === 'hbar' ? ' token-toggle-btn--active' : ''}`}
                  onClick={() => handleSelectToken('hbar')}
                  disabled={tokenLoading}
                >
                  HBAR
                </button>
                <button
                  className={`token-toggle-btn token-toggle-btn--usdc${myMember.payment_token === 'usdc' ? ' token-toggle-btn--active' : ''}`}
                  onClick={() => handleSelectToken('usdc')}
                  disabled={tokenLoading}
                >
                  USDC
                </button>
              </div>

              <div className="payment-amount">
                {myMember.payment_token === 'usdc' && (
                  <span>{formatDollars(settings.entry_fee_cents)} USDC</span>
                )}
                {myMember.payment_token === 'hbar' && (
                  <span>~[TBD] HBAR ({formatDollars(settings.entry_fee_cents)} equivalent)</span>
                )}
                {!myMember.payment_token && (
                  <span className="payment-amount--placeholder">Select a token above</span>
                )}
              </div>

              <button
                className="btn-primary payment-btn"
                onClick={handlePay}
                disabled={payLoading || !myMember.payment_token || !myMember.wallet_address}
              >
                {payLoading ? 'Processing…' : 'Pay Entry Fee'}
              </button>

              {payMessage && <p className="payment-message">{payMessage}</p>}
              {payError && <p className="payment-error">{payError}</p>}
            </>
          )}
        </div>
      )}

      {/* Invite banner (commissioner only) */}
      {isCommissioner && (
        <div className="invite-banner">
          <p>
            Share the invite link to let your league-mates connect their Wagr wallets and track
            payment status.
          </p>
        </div>
      )}
    </div>
  )
}
