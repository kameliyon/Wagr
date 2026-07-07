import { Routes, Route } from 'react-router-dom'
import Navbar from './components/Navbar'
import Home from './pages/Home'
import Leagues from './pages/Leagues'
import LeagueSettingsPage from './pages/LeagueSettings'
import LeagueOverview from './pages/LeagueOverview'
import './components/TestnetBanner.css'

function TestnetBanner() {
  return (
    <div className="testnet-banner" role="alert">
      <span className="testnet-banner-icon">⚠</span>
      <span>
        <strong>Beta</strong> — This app is currently in beta and running on the Hedera Testnet.
        You must have a Hedera testnet account and wallet to connect.
      </span>
    </div>
  )
}

function App() {
  return (
    <div className="app">
      <Navbar />
      <TestnetBanner />
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/leagues" element={<Leagues />} />
          <Route path="/leagues/:leagueId/settings" element={<LeagueSettingsPage />} />
          <Route path="/leagues/:leagueId" element={<LeagueOverview />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
