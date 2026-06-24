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
          <stop stopColor="#c4b5fd" />
          <stop offset="1" stopColor="#818cf8" />
        </linearGradient>
      </defs>
    </svg>
  )
}

export default function Navbar() {
  const { isAuthenticated } = useWallet()

  return (
    <nav className="navbar">
      <div className="navbar-content">
        <Link to="/" className="navbar-brand">
          <LogoMark />
          <span>WAGRS</span>
        </Link>

        <div className="navbar-right">
          {isAuthenticated && (
            <NavLink to="/leagues" className={({ isActive }) => `navbar-link${isActive ? ' active' : ''}`}>
              My Leagues
            </NavLink>
          )}
          <div className="navbar-divider" />
          <ConnectWallet />
        </div>
      </div>
    </nav>
  )
}
