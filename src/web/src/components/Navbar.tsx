import { useState } from 'react'
import { Link, NavLink } from 'react-router-dom'
import ConnectWallet from './ConnectWallet'
import { useWallet } from '../hooks/useWallet'
import './Navbar.css'

function LogoMark() {
  return (
    <svg className="brand-mark" viewBox="0 0 32 32" width="26" height="26" fill="none" aria-hidden="true">
      <path
        d="M16 2L30 11.5L16 30L2 11.5Z"
        stroke="url(#brandGrad)"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path
        d="M8.5 12.5L12 20L16 13.5L20 20L23.5 12.5"
        stroke="url(#brandGrad)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <defs>
        <linearGradient id="brandGrad" x1="2" y1="2" x2="30" y2="30" gradientUnits="userSpaceOnUse">
          <stop stopColor="var(--theme-svg-stop1, #bfdbfe)" />
          <stop offset="1" stopColor="var(--theme-svg-stop2, #60a5fa)" />
        </linearGradient>
      </defs>
    </svg>
  )
}

export default function Navbar() {
  const { isAuthenticated } = useWallet()
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <nav className="navbar">
      <div className="navbar-content">
        <Link to="/" className="navbar-brand">
          <LogoMark />
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
