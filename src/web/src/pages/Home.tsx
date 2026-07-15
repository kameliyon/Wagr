import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { ShieldCheck, Trophy, Zap } from 'lucide-react'
import { useWallet } from '../hooks/useWallet'
import ImportLeagueModal from '../components/ImportLeagueModal'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Badge } from '../components/ui/badge'
import type { League } from '../types/league'
import { apiUrl } from '../utils/api'
import './Home.css'

// ── Unauthenticated marketing home ───────────────────────────

interface MarketingHomeProps {
  isConnecting: boolean
  isAuthenticating: boolean
  error: string | null
  onConnect: () => void
  onAuthenticate: () => void
  isConnected: boolean
}

function MarketingHome({ isConnecting, isAuthenticating, error, onConnect, onAuthenticate, isConnected }: MarketingHomeProps) {
  return (
    <div className="flex flex-col">
      {/* ── Section 1: Hero ── */}
      <section className="relative flex flex-col items-center text-center py-20 px-4 gap-6">
        <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_50%_40%,rgba(59,130,246,0.22)_0%,transparent_70%)] pointer-events-none" />

        <Badge variant="outline" className="theme-border-primary-40 theme-text-light theme-bg-primary-10 font-body">
          Beta · Hedera Testnet
        </Badge>

        <h1 className="font-brand text-5xl sm:text-6xl font-extrabold tracking-tight leading-tight theme-gradient-text max-w-3xl">
          Trustless Fantasy League Payments
        </h1>

        <p className="text-slate-400 text-lg max-w-xl leading-relaxed font-body">
          Automate entry fees and payouts for your fantasy sports league using smart contracts on Hedera — no middleman, no trust required.
        </p>

        {!isConnected ? (
          <Button
            size="lg"
            className="mt-2 bg-primary hover:brightness-110 text-primary-foreground theme-glow-shadow px-8 py-6 text-base font-semibold font-body"
            onClick={onConnect}
            disabled={isConnecting || isAuthenticating}
          >
            {isConnecting ? 'Connecting…' : isAuthenticating ? 'Waiting for signature…' : 'Connect Wallet'}
          </Button>
        ) : (
          <Button
            size="lg"
            className="mt-2 bg-primary hover:brightness-110 text-primary-foreground theme-glow-shadow px-8 py-6 text-base font-semibold font-body"
            onClick={onAuthenticate}
            disabled={isAuthenticating}
          >
            {isAuthenticating ? 'Signing…' : 'Retry Sign In'}
          </Button>
        )}

        {error && <p className="text-red-400 text-sm font-body">{error}</p>}
      </section>

      {/* ── Section 2: How It Works ── */}
      <section className="py-16 px-4">
        <h2 className="text-center font-brand text-2xl font-bold text-slate-100 mb-10">How It Works</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-4xl mx-auto">
          {[
            {
              step: '01',
              title: 'Connect Wallet',
              desc: 'Link your HashPack wallet to prove ownership of your Hedera account. No username, no password.',
            },
            {
              step: '02',
              title: 'Import Your League',
              desc: 'Pull in your Sleeper league in seconds. Your commissioner sets the entry fee and payout structure.',
            },
            {
              step: '03',
              title: 'Payouts Happen Automatically',
              desc: 'At season end, the smart contract distributes winnings directly to member wallets. No manual transfers, no delays.',
            },
          ].map(({ step, title, desc }) => (
            <Card key={step} className="bg-white/[0.04] border-white/[0.08] backdrop-blur-sm">
              <CardHeader className="pb-2">
                <span className="theme-text-light font-mono text-sm font-bold mb-1">{step}</span>
                <CardTitle className="text-slate-100 text-base font-semibold font-body">{title}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-slate-400 text-sm leading-relaxed font-body">{desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>

      {/* ── Section 3: Platform Support ── */}
      <section className="py-16 px-4">
        <h2 className="text-center font-brand text-2xl font-bold text-slate-100 mb-3">Supported Platforms</h2>
        <p className="text-center text-slate-400 text-sm mb-10 font-body">Import your leagues directly from your fantasy platform</p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center max-w-2xl mx-auto">
          <Card className="flex-1 bg-white/[0.04] theme-border-primary-30 backdrop-blur-sm">
            <CardContent className="pt-6 flex flex-col items-center gap-3 text-center">
              <div className="w-12 h-12 rounded-xl theme-bg-primary-15 flex items-center justify-center">
                <span className="theme-text-light font-brand font-bold text-xl">S</span>
              </div>
              <CardTitle className="text-slate-100 font-body font-semibold">Sleeper</CardTitle>
              <Badge className="bg-green-500/15 text-green-400 border border-green-500/30 font-body">Live</Badge>
              <p className="text-slate-400 text-xs font-body">Full import support for NFL, NBA, and other fantasy leagues</p>
            </CardContent>
          </Card>

          <Card className="flex-1 bg-white/[0.02] border-white/[0.06] backdrop-blur-sm opacity-60">
            <CardContent className="pt-6 flex flex-col items-center gap-3 text-center">
              <div className="w-12 h-12 rounded-xl bg-white/5 flex items-center justify-center">
                <span className="text-slate-500 font-brand font-bold text-xl">E</span>
              </div>
              <CardTitle className="text-slate-400 font-body font-semibold">ESPN</CardTitle>
              <Badge variant="outline" className="border-slate-600 text-slate-500 font-body">Coming Soon</Badge>
            </CardContent>
          </Card>

          <Card className="flex-1 bg-white/[0.02] border-white/[0.06] backdrop-blur-sm opacity-60">
            <CardContent className="pt-6 flex flex-col items-center gap-3 text-center">
              <div className="w-12 h-12 rounded-xl bg-white/5 flex items-center justify-center">
                <span className="text-slate-500 font-brand font-bold text-xl">Y</span>
              </div>
              <CardTitle className="text-slate-400 font-body font-semibold">Yahoo</CardTitle>
              <Badge variant="outline" className="border-slate-600 text-slate-500 font-body">Coming Soon</Badge>
            </CardContent>
          </Card>
        </div>
      </section>

      {/* ── Section 4: Privacy ── */}
      <section className="py-16 px-4 pb-24">
        <div className="max-w-2xl mx-auto">
          <Card className="bg-white/[0.04] border-white/[0.08]">
            <CardContent className="pt-6 flex flex-col sm:flex-row gap-5 items-start">
              <div className="w-10 h-10 rounded-lg theme-bg-primary-15 flex items-center justify-center flex-shrink-0">
                <ShieldCheck className="theme-text-light w-5 h-5" />
              </div>
              <div className="flex flex-col gap-2">
                <h3 className="text-slate-100 font-semibold text-base font-body">We store no personal data</h3>
                <p className="text-slate-400 text-sm leading-relaxed font-body">
                  WAGR does not collect or store any personal information. The only data we hold is
                  your wallet address and the league payout settings configured by commissioners.
                  Your fantasy stats and profile data come directly from Sleeper's public API and
                  are never persisted in WAGR's database.
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </section>
    </div>
  )
}

// ── League status badge ───────────────────────────────────────

function LeagueStatusBadge({ league }: { league: League }) {
  if (league.cancelled_at) {
    return <span className="status-badge status-cancelled">Cancelled</span>
  }
  switch (league.status) {
    case 'in_season':  return <span className="status-badge status-active">In Season</span>
    case 'pre_draft':  return <span className="status-badge status-draft">Pre-Draft</span>
    case 'post_season': return <span className="status-badge status-active">Post Season</span>
    case 'complete':   return <span className="status-badge status-complete">Complete</span>
    default:           return null
  }
}

// ── League card ───────────────────────────────────────────────

function LeagueCard({ league }: { league: League }) {
  const initial = league.platform === 'sleeper' ? 'S' : league.platform[0].toUpperCase()
  const platformLabel = league.platform === 'sleeper' ? 'Sleeper' : league.platform
  const sport = league.sport?.toUpperCase() ?? 'NFL'

  return (
    <Link to={`/leagues/${league.id}`} className="league-card">
      <div className="league-card-top">
        <div className="league-platform-icon">{initial}</div>
        <LeagueStatusBadge league={league} />
      </div>

      <div className="league-card-body">
        <h3 className="league-card-name">{league.name}</h3>
        <p className="league-card-detail">
          {platformLabel} · {sport} · {league.season}
        </p>
        <p className="league-card-teams">{league.total_rosters} teams</p>
      </div>

      <div className="league-card-footer">
        <span className={league.entry_fee_cents > 0 ? 'fee-set' : 'fee-unset'}>
          {league.entry_fee_cents > 0
            ? `$${(league.entry_fee_cents / 100).toFixed(0)} entry`
            : 'No fee set'}
        </span>
        {league.is_commissioner && (
          <span className="commissioner-badge">Commissioner</span>
        )}
      </div>
    </Link>
  )
}

// ── Authenticated dashboard ───────────────────────────────────

interface DashboardProps {
  accountId: string | null
  user: { created_at?: string } | null
  network: string | null
  leagues: League[]
  leaguesLoading: boolean
  onImport: () => void
}

function AuthenticatedDashboard({ accountId, user, network, leagues, leaguesLoading, onImport }: DashboardProps) {
  const totalPrizePool = leagues.reduce(
    (sum, l) => sum + (l.entry_fee_cents > 0 ? l.entry_fee_cents * l.total_rosters : 0),
    0
  )
  const totalTeams = leagues.reduce((sum, l) => sum + l.total_rosters, 0)
  const commishLeagues = leagues.filter(l => l.is_commissioner)
  const actionItems = commishLeagues.filter(l => l.entry_fee_cents === 0 && !l.cancelled_at)

  const stats = [
    { label: 'Leagues',       value: leaguesLoading ? '—' : String(leagues.length) },
    { label: 'Est. Prize Pool', value: leaguesLoading ? '—' : totalPrizePool > 0 ? `$${(totalPrizePool / 100).toLocaleString()}` : '—' },
    { label: 'Total Teams',   value: leaguesLoading ? '—' : totalTeams > 0 ? String(totalTeams) : '—' },
    { label: 'Commissioner',  value: leaguesLoading ? '—' : commishLeagues.length > 0 ? String(commishLeagues.length) : '—' },
  ]

  return (
    <div className="dashboard-root">
      {/* ── Welcome ── */}
      <div className="dashboard-welcome-card">
        <div
          className="dashboard-avatar"
          style={{ background: 'var(--theme-gradient)' }}
        >
          {accountId?.split('.').pop()?.slice(-2) ?? '??'}
        </div>
        <div className="dashboard-welcome-info">
          <div className="dashboard-welcome-name">
            Welcome back
            <span className="online-dot" />
          </div>
          <div className="dashboard-account-id">{accountId}</div>
          {user?.created_at && (
            <div className="dashboard-member-since">
              Member since{' '}
              {new Date(user.created_at).toLocaleDateString('en-US', { year: 'numeric', month: 'long' })}
            </div>
          )}
        </div>
        <Badge
          variant="outline"
          className="dashboard-network-badge theme-border-primary-40 theme-text-light capitalize font-body"
        >
          {network ?? 'testnet'}
        </Badge>
      </div>

      {/* ── Stats ── */}
      <div className="stats-grid">
        {stats.map(s => (
          <div key={s.label} className="stat-card">
            <div className="stat-label">{s.label}</div>
            <div className="stat-value">{s.value}</div>
          </div>
        ))}
      </div>

      {/* ── Action callout ── */}
      {!leaguesLoading && actionItems.length > 0 && (
        <div className="action-callout">
          <div className="action-callout-header">
            <Zap size={13} />
            Action needed
          </div>
          {actionItems.map(l => (
            <Link key={l.id} to={`/leagues/${l.id}/settings`} className="action-callout-item">
              <span>
                <strong>{l.name}</strong> — no entry fee configured
              </span>
              <span className="action-arrow">→</span>
            </Link>
          ))}
        </div>
      )}

      {/* ── Leagues ── */}
      <div className="leagues-section">
        <div className="leagues-section-header">
          <h2 className="leagues-section-title">My Leagues</h2>
          <Button
            variant="outline"
            size="sm"
            className="border-white/10 text-slate-300 hover:bg-white/5 font-body"
            onClick={onImport}
          >
            + Import League
          </Button>
        </div>

        {leaguesLoading ? (
          <div className="leagues-grid">
            {[1, 2, 3].map(i => (
              <div key={i} className="league-card-skeleton" />
            ))}
          </div>
        ) : leagues.length === 0 ? (
          <div className="leagues-empty">
            <div className="leagues-empty-icon">
              <Trophy size={26} />
            </div>
            <h3>No leagues yet</h3>
            <p>Import your first league to start managing payouts</p>
            <Button
              className="mt-5 bg-primary hover:brightness-110 text-primary-foreground theme-glow-shadow font-body"
              onClick={onImport}
            >
              Import Your First League
            </Button>
          </div>
        ) : (
          <div className="leagues-grid">
            {leagues.map(league => (
              <LeagueCard key={league.id} league={league} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Page root ─────────────────────────────────────────────────

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

  return (
    <div className="home">
      {isAuthenticated ? (
        <>
          <AuthenticatedDashboard
            accountId={accountId}
            user={user}
            network={network}
            leagues={leagues}
            leaguesLoading={leaguesLoading}
            onImport={() => setShowImport(true)}
          />
          <ImportLeagueModal
            isOpen={showImport}
            onClose={() => setShowImport(false)}
            onImported={handleImported}
            existingLeagues={leagues}
          />
        </>
      ) : (
        <MarketingHome
          isConnecting={isConnecting}
          isAuthenticating={isAuthenticating}
          error={error}
          onConnect={handleConnect}
          onAuthenticate={authenticate}
          isConnected={isConnected}
        />
      )}
    </div>
  )
}
