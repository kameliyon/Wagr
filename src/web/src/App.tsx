import { Routes, Route } from 'react-router-dom'
import Navbar from './components/Navbar'
import Home from './pages/Home'
import Leagues from './pages/Leagues'
import LeagueSettingsPage from './pages/LeagueSettings'

function App() {
  return (
    <div className="app">
      <Navbar />
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/leagues" element={<Leagues />} />
          <Route path="/leagues/:leagueId/settings" element={<LeagueSettingsPage />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
