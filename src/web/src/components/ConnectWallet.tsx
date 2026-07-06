import { useState } from 'react'
import { useWallet } from '../hooks/useWallet'
import type { WalletType } from '../types/wallet'
import { formatAddress as formatWalletAddress, getWalletInstallLink, WALLET_DISPLAY_NAMES } from '../utils/walletConstants'
import './ConnectWallet.css'

export default function ConnectWallet() {
    const {
        isConnected,
        isConnecting,
        isAuthenticating,
        isAuthenticated,
        address,
        accountId,
        walletType,
        availableWallets,
        hasWallet,
        connect,
        disconnect,
        authenticate,
        error,
    } = useWallet()

    const [showWalletSelect, setShowWalletSelect] = useState(false)

    const handleConnect = async (type: WalletType, walletName: string) => {
        try {
            await connect(type, walletName)
            setShowWalletSelect(false)
        } catch {
            // Error is handled by the provider
        }
    }

    const getDisplayAddress = () => {
        if (!address || !walletType) return ''

        // For Hedera, show account ID in full
        if (walletType === 'hedera' && accountId) {
            return accountId
        }

        // For EVM wallets, truncate the 0x address
        return formatWalletAddress(address, walletType)
    }

    // Not connected - show connect button
    if (!isConnected) {
        return (
            <div className="connect-wallet">
                {!hasWallet ? (
                    <div className="no-wallet">
                        <p>No wallet detected</p>
                        <div className="install-links">
                            <a href={getWalletInstallLink('hashpack') || '#'} target="_blank" rel="noopener noreferrer">
                                Install HashPack
                            </a>
                        </div>
                    </div>
                ) : showWalletSelect ? (
                        <div className="wallet-select">
                            <p>Select a wallet:</p>
                            {availableWallets.map((wallet) => (
                                <button
                                    key={`${wallet.type}-${wallet.name}`}
                                    className="btn-secondary wallet-option"
                                    onClick={() => handleConnect(wallet.type, wallet.name)}
                                    disabled={isConnecting}
                                >
                                    <span className="wallet-name">{wallet.displayName}</span>
                                    <span className="wallet-type-badge">{WALLET_DISPLAY_NAMES[wallet.type]}</span>
                                </button>
                            ))}
                            <button
                                className="btn-secondary"
                                onClick={() => setShowWalletSelect(false)}
                            >
                                Cancel
                            </button>
                        </div>
                    ) : isConnecting || isAuthenticating ? (
                        <div className="wallet-connecting">
                            <span className="connecting-label">
                                {isConnecting ? 'Connecting...' : 'Waiting for signature...'}
                            </span>
                            <button className="btn-secondary" onClick={disconnect}>
                                Cancel
                            </button>
                        </div>
                    ) : (
                            <button
                                className="btn-primary"
                                onClick={() => setShowWalletSelect(true)}
                            >
                                Connect Wallet
                            </button>
                        )}
                {error && <p className="error">{error}</p>}
            </div>
        )
    }

    // Connected but not authenticated - show retry button (only appears if auth failed)
    if (isConnected && !isAuthenticated) {
        return (
            <div className="connect-wallet">
                <div className="wallet-connected">
                    <span className="address">
                        {walletType && <span className="wallet-type-badge">{WALLET_DISPLAY_NAMES[walletType]}</span>}
                        {getDisplayAddress()}
                    </span>
                    <button
                        className="btn-primary"
                        onClick={() => authenticate()}
                        disabled={isAuthenticating}
                    >
                        {isAuthenticating ? 'Signing...' : 'Retry Sign In'}
                    </button>
                    <button className="btn-secondary" onClick={disconnect}>Disconnect</button>
                </div>
                {error && <p className="error">{error}</p>}
            </div>
        )
    }

    // Fully authenticated
    return (
        <div className="connect-wallet authenticated">
            <span className="address">
                {walletType && <span className="wallet-type-badge">{WALLET_DISPLAY_NAMES[walletType]}</span>}
                {getDisplayAddress()}
            </span>
            <button className="btn-secondary" onClick={disconnect}>
                Disconnect
            </button>
        </div>
    )
}
