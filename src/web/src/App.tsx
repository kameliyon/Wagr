import { Routes, Route } from 'react-router-dom'
import Navbar from './components/Navbar'
import Home from './pages/Home'
import Leagues from './pages/Leagues'
import LeagueSettingsPage from './pages/LeagueSettings'
import LeagueOverview from './pages/LeagueOverview'

function App() {
  return (
    <div className="app">
      <Navbar />
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
