import { Link } from 'react-router-dom'
import ConnectWallet from './ConnectWallet'
import './Navbar.css'

export default function Navbar() {
  return (
    <nav className="navbar">
      <div className="navbar-content">
        <Link to="/" className="navbar-brand">
          WAGR
        </Link>

        <div className="navbar-right">
          <ConnectWallet />
        </div>
      </div>
    </nav>
  )
}
