import { useWallet } from '../hooks/useWallet'
import './Home.css'

export default function Home() {
  const { isAuthenticated, user, address } = useWallet()

  return (
    <div className="home">
      <div className="hero">
        <h1>Welcome to WAGR</h1>
        <p className="subtitle">
          Web3 payments for Fantasy Sports leagues
        </p>
      </div>

      {isAuthenticated ? (
        <div className="dashboard">
          <div className="card">
            <h2>Your Profile</h2>
            <div className="profile-info">
              <div className="info-row">
                <span className="label">User ID:</span>
                <span className="value">{user?.id}</span>
              </div>
              <div className="info-row">
                <span className="label">Wallet:</span>
                <span className="value mono">{address}</span>
              </div>
              <div className="info-row">
                <span className="label">Member since:</span>
                <span className="value">
                  {user?.created_at && new Date(user.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>
          </div>

          <div className="card">
            <h2>Your Leagues</h2>
            <p className="placeholder">
              League management coming soon. Connect your Sleeper account to import your leagues.
            </p>
          </div>
        </div>
      ) : (
        <div className="welcome">
          <div className="card">
            <h2>Get Started</h2>
            <p>
              Connect your Hedera wallet to start managing your fantasy sports payments
              on the blockchain.
            </p>
            <ul className="features">
              <li>Secure, transparent entry fee collection</li>
              <li>Automated payouts via smart contracts</li>
              <li>Low transaction fees on Hedera Network</li>
              <li>Fast, secure payments</li>
            </ul>
          </div>
        </div>
      )}
    </div>
  )
}
