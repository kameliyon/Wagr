import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import { HederaStrategy } from '../strategies/HederaStrategy'
import type { LeagueDetail, LeagueSettings, LeagueMember, PaymentToken, PaymentInstructions } from '../types/league'
import './LeagueOverview.css'
import { apiUrl } from '../utils/api'

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
  const pendingTxKey = leagueId ? `wagr_pending_tx_${leagueId}` : null
  const [pendingTxId, setPendingTxId] = useState<string | null>(
    () => (pendingTxKey ? localStorage.getItem(pendingTxKey) : null)
  )

  // Refund panel state
  const [refundStage, setRefundStage] = useState<string | null>(null)
  const [refundError, setRefundError] = useState<string | null>(null)
  const [refundLoading, setRefundLoading] = useState(false)
  const pendingRefundTxKey = leagueId ? `wagr_pending_refund_${leagueId}` : null
  const [pendingRefundTxId, setPendingRefundTxId] = useState<string | null>(
    () => (pendingRefundTxKey ? localStorage.getItem(pendingRefundTxKey) : null)
  )
  const [cancelLoading, setCancelLoading] = useState(false)
  const [reactivateLoading, setReactivateLoading] = useState(false)

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!token || !leagueId) return
    const fetchAll = async () => {
      try {
        const [detailRes, settingsRes] = await Promise.all([
          fetch(apiUrl(`/api/leagues/${leagueId}`), { headers: { Authorization: `Bearer ${token}` } }),
          fetch(apiUrl(`/api/leagues/${leagueId}/settings`), { headers: { Authorization: `Bearer ${token}` } }),
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
      res = await fetch(apiUrl(`/api/leagues/${leagueId}/confirm-payment`), {
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
      const payRes = await fetch(apiUrl(`/api/leagues/${leagueId}/pay`), {
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

  const confirmRefundWithRetry = async (txId: string) => {
    const MAX_ATTEMPTS = 8
    const RETRY_MS = 5000
    let res: Response | null = null
    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
      setRefundStage(
        attempt === 1
          ? 'Waiting for on-chain confirmation…'
          : `Confirming refund… (${attempt}/${MAX_ATTEMPTS})`
      )
      if (attempt > 1) await new Promise((resolve) => setTimeout(resolve, RETRY_MS))
      res = await fetch(apiUrl(`/api/leagues/${leagueId}/confirm-refund`), {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ transaction_id: txId }),
      })
      if (res.status !== 402) break
    }
    if (!res) throw new Error('Confirmation failed')
    if (res.status === 402) {
      throw new Error('On-chain refund not confirmed — it may still be propagating. Use "Confirm Existing Refund" to retry.')
    }
    if (res.status === 409) return
    if (!res.ok) {
      const msg = await res.text()
      throw new Error(msg || `Refund confirmation failed (${res.status})`)
    }
  }

  const markLocalRefunded = (txId: string) => {
    if (pendingRefundTxKey) localStorage.removeItem(pendingRefundTxKey)
    setPendingRefundTxId(null)
    setDetail((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        members: prev.members.map((m) =>
          m.user_id === user?.id
            ? { ...m, payment_status: 'refunded' as const, transaction_hash: txId }
            : m,
        ),
      }
    })
  }

  const handleCancelLeague = async () => {
    if (!token || !leagueId) return
    if (!window.confirm('Cancel this league? This cannot be undone. Paid members will be able to claim refunds.')) return
    setCancelLoading(true)
    try {
      const res = await fetch(apiUrl(`/api/leagues/${leagueId}/cancel`), {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || `Failed to cancel league (${res.status})`)
      }
      setDetail((prev) => prev ? { ...prev, league: { ...prev.league, cancelled_at: new Date().toISOString() } } : prev)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel league')
    } finally {
      setCancelLoading(false)
    }
  }

  const handleReactivateLeague = async () => {
    if (!token || !leagueId) return
    if (!window.confirm('Reactivate this league? Members who claimed refunds will need to re-pay their entry fee.')) return
    setReactivateLoading(true)
    try {
      const res = await fetch(apiUrl(`/api/leagues/${leagueId}/reactivate`), {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || `Failed to reactivate league (${res.status})`)
      }
      setDetail((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          league: { ...prev.league, cancelled_at: null },
          members: prev.members.map((m) =>
            m.payment_status === 'refunded'
              ? { ...m, payment_status: 'unpaid' as const, transaction_hash: undefined }
              : m,
          ),
        }
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reactivate league')
    } finally {
      setReactivateLoading(false)
    }
  }

  const handleClaimRefund = async () => {
    if (!token || !leagueId) return
    if (!walletState || !activeStrategy) {
      setRefundError('Wallet not connected. Please reconnect your wallet.')
      return
    }
    if (!(activeStrategy instanceof HederaStrategy)) {
      setRefundError('Only Hedera wallets are supported for USDC refunds')
      return
    }

    const contractId = import.meta.env.VITE_HEDERA_ESCROW_CONTRACT_ID
    if (!contractId) {
      setRefundError('Contract ID not configured')
      return
    }

    setRefundLoading(true)
    setRefundStage(null)
    setRefundError(null)

    try {
      setRefundStage('Sending refund transaction in HashPack…')
      let transactionId: string
      try {
        const result = await activeStrategy.claimRefund({ leagueId, contractId, walletState })
        transactionId = result.transactionId
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        if (msg.toLowerCase().includes('reject') || msg.toLowerCase().includes('cancel')) {
          throw new Error('Transaction rejected in HashPack')
        }
        if (msg.includes('CONTRACT_REVERT_EXECUTED')) {
          // Extract transactionId from the StatusError message and look up the Solidity revert reason
          const txMatch = msg.match(/transaction ([\d.@]+)/)
          if (txMatch) {
            // "0.0.12345@1234567890.123456789" → "0.0.12345-1234567890-123456789"
            const mirrorTxId = txMatch[1].replace('@', '-').replace(/\.(\d+)$/, '-$1')
            try {
              const mirrorRes = await fetch(
                `https://testnet.mirrornode.hedera.com/api/v1/contracts/results/${mirrorTxId}`,
              )
              if (mirrorRes.ok) {
                const mirrorData = await mirrorRes.json()
                const reason = mirrorData?.error_message
                if (reason) throw new Error(`Contract reverted: "${reason}"`)
              }
            } catch (innerErr) {
              if (innerErr instanceof Error && innerErr.message.startsWith('Contract reverted')) throw innerErr
            }
          }
        }
        throw new Error(`HashPack transaction failed: ${msg}`)
      }

      if (pendingRefundTxKey) localStorage.setItem(pendingRefundTxKey, transactionId)
      setPendingRefundTxId(transactionId)

      await confirmRefundWithRetry(transactionId)
      markLocalRefunded(transactionId)
      setRefundStage(null)
    } catch (err) {
      setRefundError(err instanceof Error ? err.message : 'Refund failed')
      setRefundStage(null)
    } finally {
      setRefundLoading(false)
    }
  }

  const handleRetryRefundConfirm = async () => {
    if (!token || !leagueId || !pendingRefundTxId) return
    setRefundLoading(true)
    setRefundStage(null)
    setRefundError(null)
    try {
      await confirmRefundWithRetry(pendingRefundTxId)
      markLocalRefunded(pendingRefundTxId)
      setRefundStage(null)
    } catch (err) {
      setRefundError(err instanceof Error ? err.message : 'Confirmation failed')
      setRefundStage(null)
    } finally {
      setRefundLoading(false)
    }
  }

  const handleDismissPendingRefund = () => {
    if (pendingRefundTxKey) localStorage.removeItem(pendingRefundTxKey)
    setPendingRefundTxId(null)
    setRefundError(null)
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
  const isCancelled = !!league.cancelled_at

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
          {isCommissioner && !isCancelled && (
            <button className="btn-danger" onClick={handleCancelLeague} disabled={cancelLoading}>
              {cancelLoading ? 'Cancelling…' : 'Cancel League'}
            </button>
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

      {/* Cancelled banner */}
      {isCancelled && (
        <div className="cancelled-banner">
          <span>This league has been cancelled. Paid members may claim their refund below.</span>
          {isCommissioner && (
            <button className="btn-reactivate" onClick={handleReactivateLeague} disabled={reactivateLoading}>
              {reactivateLoading ? 'Reactivating…' : 'Reactivate League'}
            </button>
          )}
        </div>
      )}

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
                <th>Wagrs</th>
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
          {myMember.payment_status === 'refunded' ? (
            <div className="payment-paid">
              <span className="badge badge--refunded">Refunded</span>
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
          ) : myMember.payment_status === 'paid' && isCancelled ? (
            <>
              <div className="payment-paid">
                <span className="badge badge--paid">Paid</span>
                {myMember.transaction_hash && (
                  <a
                    className="hashscan-link"
                    href={`https://hashscan.io/testnet/transaction/${myMember.transaction_hash}`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    View payment on HashScan
                  </a>
                )}
              </div>

              {pendingRefundTxId ? (
                <>
                  <p className="payment-warning">
                    Refund transaction submitted but not yet confirmed.{' '}
                    <a
                      href={`https://hashscan.io/testnet/transaction/${pendingRefundTxId}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      View on HashScan
                    </a>
                  </p>
                  <div style={{ display: 'flex', gap: '0.6rem', alignItems: 'center', flexWrap: 'wrap' }}>
                    <button
                      className="btn-primary payment-btn"
                      onClick={handleRetryRefundConfirm}
                      disabled={refundLoading}
                    >
                      {refundLoading ? (refundStage ?? 'Confirming…') : 'Confirm Existing Refund'}
                    </button>
                    <button
                      className="btn-link-muted"
                      onClick={handleDismissPendingRefund}
                      disabled={refundLoading}
                    >
                      Try New Transaction
                    </button>
                  </div>
                </>
              ) : (
                <button
                  className="btn-primary payment-btn"
                  onClick={handleClaimRefund}
                  disabled={refundLoading || !walletState}
                >
                  {refundLoading ? (refundStage ?? 'Processing…') : 'Claim Refund'}
                </button>
              )}

              {!walletState && !pendingRefundTxId && (
                <p className="payment-warning">Wallet disconnected — reconnect to claim your refund.</p>
              )}
              {refundError && <p className="payment-error">{refundError}</p>}
            </>
          ) : myMember.payment_status === 'paid' ? (
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
          ) : isCancelled ? (
            <p className="payment-warning">This league has been cancelled. No payment is required.</p>
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
            Share the invite link to let your league-mates connect their Wagrs wallets and track
            payment status.
          </p>
        </div>
      )}
    </div>
  )
}
