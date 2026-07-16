import { useState } from 'react'
import { Link, NavLink } from 'react-router-dom'
import ConnectWallet from './ConnectWallet'
import { useWallet } from '../hooks/useWallet'
import './Navbar.css'

export default function Navbar() {
  const { isAuthenticated } = useWallet()
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <nav className="navbar">
      <div className="navbar-content">
        <Link to="/" className="navbar-brand">
          <img src="/svg/icon-badge.svg" className="brand-mark" width="28" height="28" alt="" aria-hidden="true" />
          <span>WAGRS</span>
        </Link>

        {/* Desktop nav */}
        <div className="navbar-right hidden md:flex">
          {isAuthenticated && (
            <NavLink to="/leagues" className={({ isActive }) => `navbar-link${isActive ? ' active' : ''}`}>
              My Leagues
            </NavLink>
          )}
          <div className="navbar-divider" />
          <ConnectWallet />
        </div>

        {/* Mobile hamburger */}
        <button
          className="md:hidden flex flex-col gap-1.5 p-2 rounded-lg hover:bg-white/5 transition-colors border-0"
          style={{ background: 'none' }}
          onClick={() => setMenuOpen(m => !m)}
          aria-label="Toggle menu"
          aria-expanded={menuOpen}
        >
          <span
            className="block w-5 h-0.5 bg-slate-300 transition-all duration-200"
            style={{ transform: menuOpen ? 'rotate(45deg) translateY(8px)' : 'none' }}
          />
          <span
            className="block w-5 h-0.5 bg-slate-300 transition-all duration-200"
            style={{ opacity: menuOpen ? 0 : 1 }}
          />
          <span
            className="block w-5 h-0.5 bg-slate-300 transition-all duration-200"
            style={{ transform: menuOpen ? 'rotate(-45deg) translateY(-8px)' : 'none' }}
          />
        </button>
      </div>

      {/* Mobile dropdown */}
      {menuOpen && (
        <div className="md:hidden border-t border-white/[0.08] bg-[#060b18]/95 backdrop-blur-xl px-6 py-4 flex flex-col gap-4">
          {isAuthenticated && (
            <NavLink
              to="/leagues"
              className={({ isActive }) =>
                `text-sm font-medium transition-colors ${isActive ? 'text-white' : 'text-slate-400 hover:text-slate-200'}`
              }
              onClick={() => setMenuOpen(false)}
            >
              My Leagues
            </NavLink>
          )}
          <ConnectWallet />
        </div>
      )}
    </nav>
  )
}
