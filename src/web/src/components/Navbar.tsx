import { Link } from 'react-router-dom'
import ConnectWallet from './ConnectWallet'
import { useWallet } from '../hooks/useWallet'
import './Navbar.css'

export default function Navbar() {
    const { isAuthenticated } = useWallet()

    return (
        <nav className="navbar">
            <div className="navbar-content">
                <Link to="/" className="navbar-brand">
                    WAGR
                </Link>

                <div className="navbar-right">
                    {isAuthenticated && (
                        <Link to="/leagues" className="navbar-link">
                            My Leagues
                        </Link>
                    )}
                    <ConnectWallet />
                </div>
            </div>
        </nav>
    )
}
