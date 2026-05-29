import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import { HederaStrategy } from '../strategies/HederaStrategy'
import type { LeagueDetail, LeagueSettings, LeagueMember, PaymentToken, PaymentInstructions } from '../types/league'
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
  const { isAuthenticated, token, user, walletState, activeStrategy } = useWallet()
  const navigate = useNavigate()

  const [detail, setDetail] = useState<LeagueDetail | null>(null)
  const [settings, setSettings] = useState<LeagueSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notMember, setNotMember] = useState(false)
  const [copied, setCopied] = useState(false)

  // Payment panel state
  const [payStage, setPayStage] = useState<string | null>(null)
  const [payError, setPayError] = useState<string | null>(null)
  const [payLoading, setPayLoading] = useState(false)
  // Persisted transaction ID for a submitted-but-not-yet-confirmed payment.
  // Saved before confirmation so a page refresh doesn't lose it.
  const pendingTxKey = leagueId ? `wagr_pending_tx_${leagueId}` : null
  const [pendingTxId, setPendingTxId] = useState<string | null>(
    () => (pendingTxKey ? localStorage.getItem(pendingTxKey) : null)
  )

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

  // Polls confirm-payment with retries to tolerate Mirror Node indexing lag.
  const confirmPaymentWithRetry = async (txId: string) => {
    const MAX_ATTEMPTS = 8
    const RETRY_MS = 5000
    let res: Response | null = null
    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
      setPayStage(
        attempt === 1
          ? 'Waiting for on-chain confirmation…'
          : `Confirming on-chain… (${attempt}/${MAX_ATTEMPTS})`
      )
      if (attempt > 1) await new Promise((resolve) => setTimeout(resolve, RETRY_MS))
      res = await fetch(`/api/leagues/${leagueId}/confirm-payment`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ transaction_id: txId }),
      })
      if (res.status !== 402) break
    }
    if (!res) throw new Error('Confirmation failed')
    if (res.status === 402) {
      throw new Error('On-chain payment not found — it may still be propagating. Use "Confirm Existing Payment" to retry.')
    }
    if (res.status === 409) return // already confirmed — treat as success
    if (!res.ok) {
      const msg = await res.text()
      throw new Error(msg || `Confirmation failed (${res.status})`)
    }
  }

  const markLocalPaid = (txId: string) => {
    if (pendingTxKey) localStorage.removeItem(pendingTxKey)
    setPendingTxId(null)
    setDetail((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        members: prev.members.map((m) =>
          m.user_id === user?.id
            ? { ...m, payment_status: 'paid' as const, transaction_hash: txId }
            : m,
        ),
      }
    })
  }

  const handlePay = async () => {
    if (!token || !leagueId) return
    if (!walletState || !activeStrategy) {
      setPayError('Wallet not connected. Please reconnect your wallet.')
      return
    }
    if (!(activeStrategy instanceof HederaStrategy)) {
      setPayError('Only Hedera wallets are supported for USDC payments')
      return
    }

    setPayLoading(true)
    setPayStage(null)
    setPayError(null)

    try {
      // Step 1: Get payment instructions from backend
      setPayStage('Fetching payment details…')
      const payRes = await fetch(`/api/leagues/${leagueId}/pay`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (payRes.status === 409) {
        throw new Error('Entry fee already paid')
      }
      if (!payRes.ok) {
        const msg = await payRes.text()
        throw new Error(msg || `Failed to initiate payment (${payRes.status})`)
      }
      const instructions = await payRes.json() as PaymentInstructions

      // Step 2: Send transaction via HashPack
      setPayStage(`Approving ${instructions.amount_formatted} in HashPack…`)
      let transactionId: string
      try {
        const result = await activeStrategy.payEntryFeeUSDC({
          leagueId,
          amountUSDC: instructions.amount_usdc,
          contractId: instructions.contract_id,
          usdcTokenId: instructions.usdc_token_id,
          walletState,
        })
        transactionId = result.transactionId
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        if (msg.toLowerCase().includes('reject') || msg.toLowerCase().includes('cancel')) {
          throw new Error('Transaction rejected in HashPack')
        }
        throw new Error(`HashPack transaction failed: ${msg}`)
      }

      // Persist before confirming — if the page reloads before confirmation succeeds
      // the user can retry without re-sending the transaction.
      if (pendingTxKey) localStorage.setItem(pendingTxKey, transactionId)
      setPendingTxId(transactionId)

      // Step 3: Confirm with backend
      await confirmPaymentWithRetry(transactionId)
      markLocalPaid(transactionId)
      setPayStage(null)
    } catch (err) {
      setPayError(err instanceof Error ? err.message : 'Payment failed')
      setPayStage(null)
    } finally {
      setPayLoading(false)
    }
  }

  const handleRetryConfirm = async () => {
    if (!token || !leagueId || !pendingTxId) return
    setPayLoading(true)
    setPayStage(null)
    setPayError(null)
    try {
      await confirmPaymentWithRetry(pendingTxId)
      markLocalPaid(pendingTxId)
      setPayStage(null)
    } catch (err) {
      setPayError(err instanceof Error ? err.message : 'Confirmation failed')
      setPayStage(null)
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
              {myMember.wallet_address && !walletState && !pendingTxId && (
                <p className="payment-warning">Wallet disconnected — reconnect to pay your entry fee.</p>
              )}

              <div className="payment-amount">
                <span>{formatDollars(settings.entry_fee_cents)} USDC</span>
              </div>

              {pendingTxId ? (
                <>
                  <p className="payment-warning">
                    Transaction submitted but not yet confirmed.{' '}
                    <a
                      href={`https://hashscan.io/testnet/transaction/${pendingTxId}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      View on HashScan
                    </a>
                  </p>
                  <button
                    className="btn-primary payment-btn"
                    onClick={handleRetryConfirm}
                    disabled={payLoading}
                  >
                    {payLoading ? (payStage ?? 'Confirming…') : 'Confirm Existing Payment'}
                  </button>
                </>
              ) : (
                <button
                  className="btn-primary payment-btn"
                  onClick={handlePay}
                  disabled={payLoading || !myMember.wallet_address || !walletState}
                >
                  {payLoading ? (payStage ?? 'Processing…') : 'Pay Entry Fee'}
                </button>
              )}

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
