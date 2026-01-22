import { useState } from 'react'
import { useWallet } from '../hooks/useWallet'
import './ConnectWallet.css'

export default function ConnectWallet() {
    const {
        isConnected,
        isConnecting,
        isAuthenticated,
        address,
        availableWallets,
        hasWallet,
        connect,
        disconnect,
        authenticate,
        error,
    } = useWallet()

    const [showWalletSelect, setShowWalletSelect] = useState(false)
    const [isAuthenticating, setIsAuthenticating] = useState(false)

    const handleConnect = async (walletName: string) => {
        try {
            await connect(walletName)
            setShowWalletSelect(false)
        } catch {
            // Error is handled by the provider
        }
    }

    const handleAuthenticate = async () => {
        setIsAuthenticating(true)
        try {
            await authenticate()
        } catch {
            // Error is handled by the provider
        } finally {
            setIsAuthenticating(false)
        }
    }

    const formatAddress = (addr: string) => {
        console.log('Formatting address:', addr)
        return `${addr.slice(0, 6)}...${addr.slice(-4)}`
    }

    // Not connected - show connect button
    if (!isConnected) {
        return (
            <div className="connect-wallet">
                {!hasWallet ? (
                    <div className="no-wallet">
                        <p>No Midnight wallet detected</p>
                        <a href="https://www.lace.io/" target="_blank" rel="noopener noreferrer">
                            Install Lace Wallet
                        </a>
                    </div>
                ) : showWalletSelect ? (
                        <div className="wallet-select">
                            <p>Select a wallet:</p>
                            {availableWallets.map((wallet) => (
                                <button
                                    key={wallet}
                                    className="btn-secondary wallet-option"
                                    onClick={() => handleConnect(wallet)}
                                    disabled={isConnecting}
                                >
                                    {wallet}
                                </button>
                            ))}
                            <button
                                className="btn-secondary"
                                onClick={() => setShowWalletSelect(false)}
                            >
                                Cancel
                            </button>
                        </div>
                    ) : (
                            <button
                                className="btn-primary"
                                onClick={() => availableWallets.length === 1
                                    ? handleConnect(availableWallets[0])
                                    : setShowWalletSelect(true)
                                }
                                disabled={isConnecting}
                            >
                                {isConnecting ? 'Connecting...' : 'Connect Wallet'}
                            </button>
                        )}
                {error && <p className="error">{error}</p>}
            </div>
        )
    }

    // Connected but not authenticated - show sign in button
    if (!isAuthenticated) {
        return (
            <div className="connect-wallet">
                <div className="wallet-connected">
                    <span className="address">{address && formatAddress(address)}</span>
                    <button
                        className="btn-primary"
                        onClick={handleAuthenticate}
                        disabled={isAuthenticating}
                    >
                        {isAuthenticating ? 'Signing...' : 'Sign In'}
                    </button>
                    <button className="btn-secondary" onClick={disconnect}>
                        Disconnect
                    </button>
                </div>
                {error && <p className="error">{error}</p>}
            </div>
        )
    }

    // Fully authenticated
    return (
        <div className="connect-wallet authenticated">
            <span className="address">{address && formatAddress(address)}</span>
            <button className="btn-secondary" onClick={disconnect}>
                Disconnect
            </button>
        </div>
    )
}
