import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useWallet } from '../hooks/useWallet'
import ImportLeagueModal from '../components/ImportLeagueModal'
import type { League } from '../types/league'
import { apiUrl } from '../utils/api'
import './Home.css'

const STEPS = [
  { label: 'Connect Wallet' },
  { label: 'Play' },
]

function WizardStepper({ activeStep }: { activeStep: number }) {
  return (
    <div className="wizard-stepper">
      {STEPS.map((step, i) => {
        const stepNum = i + 1
        const isDone = stepNum < activeStep
        const isActive = stepNum === activeStep
        return (
          <div key={i} className="wizard-step-wrapper">
            {i > 0 && (
              <div className={`wizard-connector${isDone ? ' passed' : ''}`} />
            )}
            <div className={`wizard-step${isActive ? ' active' : isDone ? ' done' : ' locked'}`}>
              <div className="wizard-step-circle">
                {isDone ? '✓' : stepNum}
              </div>
              <span className="wizard-step-label">{step.label}</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export default function Home() {
  const {
    isAuthenticated, isConnected, isConnecting, isAuthenticating,
    availableWallets, connect, authenticate, error,
    user, accountId, network, token,
  } = useWallet()

  const [leagues, setLeagues] = useState<League[]>([])
  const [leaguesLoading, setLeaguesLoading] = useState(false)
  const [showImport, setShowImport] = useState(false)

  useEffect(() => {
    if (!isAuthenticated || !token) return
    setLeaguesLoading(true)
    fetch(apiUrl('/api/leagues'), { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json())
      .then(data => setLeagues(Array.isArray(data) ? data : []))
      .catch(() => setLeagues([]))
      .finally(() => setLeaguesLoading(false))
  }, [isAuthenticated, token])

  const handleConnect = () => {
    if (availableWallets.length >= 1) {
      connect(availableWallets[0].type, availableWallets[0].name)
    }
  }

  const handleImported = () => {
    setShowImport(false)
    if (!token) return
    fetch(apiUrl('/api/leagues'), { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json())
      .then(data => setLeagues(Array.isArray(data) ? data : []))
      .catch(() => {})
  }

  // Step 1 = connect, step 2 = auth retry (edge case: connected but signature failed)
  const wizardStep = !isConnected ? 1 : 2

  return (
    <div className="home">
      <div className="hero">
        <h1>Welcome to WAGRS</h1>
        <p className="subtitle">Web3 payments for Fantasy Sports leagues</p>
      </div>

      {isAuthenticated ? (
        <>
          <div className="dash-welcome card">
            <div className="dash-avatar">
              {accountId?.split('.').pop()?.slice(-2) ?? '??'}
            </div>
            <div className="dash-info">
              <div className="dash-greeting">
                <span>Welcome back</span>
                <span className="dash-dot" />
              </div>
              <p className="dash-account">{accountId}</p>
              {user?.created_at && (
                <p className="dash-since">
                  Member since{' '}
                  {new Date(user.created_at).toLocaleDateString('en-US', {
                    year: 'numeric',
                    month: 'long',
                  })}
                </p>
              )}
            </div>
          </div>

          <div className="dash-stats">
            <div className="stat-card card">
              <span className="stat-label">Leagues</span>
              <span className="stat-value">{leaguesLoading ? '—' : leagues.length}</span>
            </div>

            <div className="stat-card card">
              <span className="stat-label">Network</span>
              <span className={`network-badge ${network ?? 'testnet'}`}>
                {network ?? 'testnet'}
              </span>
            </div>
          </div>

          <div className="leagues-card card">
            <div className="leagues-card-header">
              <h2>My Leagues</h2>
              <button className="btn-secondary btn-sm" onClick={() => setShowImport(true)}>
                + Import League
              </button>
            </div>

            {leaguesLoading ? (
              <p className="leagues-empty">Loading…</p>
            ) : leagues.length === 0 ? (
              <p className="leagues-empty">
                No leagues yet. Import one to get started.
              </p>
            ) : (
              <>
                <ul className="league-list">
                  {leagues.slice(0, 3).map(league => (
                    <li key={league.id}>
                      <Link to={`/leagues/${league.id}`} className="league-row">
                        <div className="league-row-main">
                          <span className="league-name">{league.name}</span>
                          <span className="league-meta">
                            {league.season} · {league.total_rosters} teams
                            {league.entry_fee_cents > 0 && ` · $${(league.entry_fee_cents / 100).toFixed(0)} entry`}
                          </span>
                        </div>
                        <div className="league-row-right">
                          {league.is_commissioner && (
                            <span className="commissioner-badge">Commissioner</span>
                          )}
                          <span className="action-arrow">→</span>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
                {leagues.length > 3 && (
                  <Link to="/leagues" className="leagues-view-all">
                    View all {leagues.length} leagues →
                  </Link>
                )}
                {leagues.length <= 3 && (
                  <Link to="/leagues" className="leagues-view-all">
                    Manage leagues →
                  </Link>
                )}
              </>
            )}
          </div>

          <ImportLeagueModal
            isOpen={showImport}
            onClose={() => setShowImport(false)}
            onImported={handleImported}
            existingLeagues={leagues}
          />
        </>
      ) : (
        <div className="onboarding">
          <WizardStepper activeStep={wizardStep} />

          <div className="wizard-panel card">
            {wizardStep === 1 && (
              <div className="wizard-content">
                <div className="wizard-icon">🔗</div>
                <h2>Connect Your Wallet</h2>
                <p>
                  WAGRS uses the Hedera network for secure, low-fee payments.
                  Connect with <strong>HashPack</strong> — via browser extension or
                  mobile app — to get started.
                </p>
                <ul className="features">
                  <li>Secure, transparent entry fee collection</li>
                  <li>Automated payouts via smart contracts</li>
                  <li>Low transaction fees on Hedera Network</li>
                </ul>
                <button
                  className="btn-primary wizard-btn"
                  onClick={handleConnect}
                  disabled={isConnecting || isAuthenticating}
                >
                  {isConnecting
                    ? 'Connecting…'
                    : isAuthenticating
                    ? 'Waiting for signature…'
                    : 'Connect Wallet'}
                </button>
                {error && <p className="wizard-error">{error}</p>}
              </div>
            )}

            {wizardStep === 2 && (
              <div className="wizard-content">
                <div className="wizard-icon">✍️</div>
                <h2>Sign In</h2>
                <p>
                  Your wallet is connected but sign-in didn't complete. Sign the
                  authentication message in HashPack to finish.
                </p>
                <button
                  className="btn-primary wizard-btn"
                  onClick={() => authenticate()}
                  disabled={isAuthenticating}
                >
                  {isAuthenticating ? 'Signing…' : 'Retry Sign In'}
                </button>
                {error && <p className="wizard-error">{error}</p>}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
