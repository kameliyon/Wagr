// Wallet configuration provider

import { createContext, useContext, ReactNode } from 'react'
import type { WalletConfig } from '../types/wallet'
import { MIDNIGHT_DEFAULT_NETWORK, HEDERA_DEFAULT_NETWORK } from '../utils/walletConstants'

const WalletConfigContext = createContext<WalletConfig | null>(null)

interface WalletConfigProviderProps {
    children: ReactNode
    walletConnectProjectId?: string
}

export function WalletConfigProvider({ children, walletConnectProjectId }: WalletConfigProviderProps) {
    // Load configuration from environment variables if not provided
    const projectId = walletConnectProjectId || import.meta.env.VITE_WALLETCONNECT_PROJECT_ID

    const config: WalletConfig = {
        walletConnectProjectId: projectId,
        defaultNetwork: {
            midnight: MIDNIGHT_DEFAULT_NETWORK,
            hedera: HEDERA_DEFAULT_NETWORK,
        },
    }

    return (
        <WalletConfigContext.Provider value={config}>
            {children}
        </WalletConfigContext.Provider>
    )
}

export function useWalletConfig(): WalletConfig {
    const context = useContext(WalletConfigContext)
    if (!context) {
        throw new Error('useWalletConfig must be used within a WalletConfigProvider')
    }
    return context
}
