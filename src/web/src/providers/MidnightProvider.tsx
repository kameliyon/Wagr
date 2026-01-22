import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type { MidnightWalletAPI, MidnightNetworkId } from '../types/midnight'

interface User {
    id: string
    wallet_address: string
    created_at: string
    updated_at: string
}

interface WalletState {
    address: string
    balance?: string
}

interface MidnightContextType {
    isConnected: boolean
    isConnecting: boolean
    walletState: WalletState | null
    walletApi: MidnightWalletAPI | null
    user: User | null
    token: string | null
    availableWallets: string[]
    connect: (walletName: string, networkId?: MidnightNetworkId) => Promise<void>
    disconnect: () => void
    authenticate: () => Promise<void>
    error: string | null
}

const MidnightContext = createContext<MidnightContextType | null>(null)

export function MidnightProvider({ children }: { children: ReactNode }) {
    const [isConnected, setIsConnected] = useState(false)
    const [isConnecting, setIsConnecting] = useState(false)
    const [walletState, setWalletState] = useState<WalletState | null>(null)
    const [walletApi, setWalletApi] = useState<MidnightWalletAPI | null>(null)
    const [user, setUser] = useState<User | null>(null)
    const [token, setToken] = useState<string | null>(() => localStorage.getItem('wagr_token'))
    const [availableWallets, setAvailableWallets] = useState<string[]>([])
    const [error, setError] = useState<string | null>(null)

    // Detect available wallets on mount
    useEffect(() => {
        const checkWallets = () => {
            if (window.midnight) {
                const wallets = Object.keys(window.midnight)
                setAvailableWallets(wallets)
            }
        }

        // Check immediately and after a delay (wallets may inject after page load)
        checkWallets()
        const timeout = setTimeout(checkWallets, 1000)
        return () => clearTimeout(timeout)
    }, [])

    // Restore session on mount
    useEffect(() => {
        if (token) {
            fetchCurrentUser(token)
        }
    }, [])

    const fetchCurrentUser = async (authToken: string) => {
        try {
            const response = await fetch('/api/auth/me', {
                headers: { Authorization: `Bearer ${authToken}` },
            })
            if (response.ok) {
                const userData = await response.json()
                setUser(userData)
                setIsConnected(true)
            } else {
                // Token is invalid, clear it
                localStorage.removeItem('wagr_token')
                setToken(null)
            }
        } catch {
            localStorage.removeItem('wagr_token')
            setToken(null)
        }
    }

    const connect = useCallback(async (walletName: string, networkId: MidnightNetworkId = 'preview') => {
        setIsConnecting(true)
        setError(null)

        try {
            if (!window.midnight) {
                throw new Error('Midnight wallet extension not detected. Please install a compatible wallet.')
            }

            const wallet = window.midnight[walletName]
            if (!wallet) {
                throw new Error(`Wallet "${walletName}" not found. Available wallets: ${availableWallets.join(', ')}`)
            }

            // Connect to wallet
            const api = await wallet.connect(networkId)

            console.log('Midnight API connected')

            // Get wallet address using Midnight-specific method
            const address = await api.getUnshieldedAddress()
            console.log('Wallet address:', address.unshieldedAddress)

            // Get balances (optional - for display purposes)
            const unshieldedBalances = await api.getUnshieldedBalances().catch(() => null)
            console.log('Unshielded balances:', unshieldedBalances)

            const state: WalletState = {
                address: address.unshieldedAddress,
                balance: unshieldedBalances ? JSON.stringify(unshieldedBalances) : undefined
            }

            console.log('Wallet state:', state)

            setWalletApi(api)
            setWalletState(state)
            setIsConnected(true)
        } catch (err) {
            const message = err instanceof Error ? err.message : 'Failed to connect wallet'
            setError(message)
            throw err
        } finally {
            setIsConnecting(false)
        }
    }, [availableWallets])

    const authenticate = useCallback(async () => {
        if (!walletApi || !walletState) {
            throw new Error('Wallet not connected')
        }

        setError(null)

        try {
            // Step 1: Get nonce from backend
            console.log('Step 1: Getting nonce for address:', walletState.address)
            const nonceResponse = await fetch('/api/auth/nonce', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ wallet_address: walletState.address }),
            })

            if (!nonceResponse.ok) {
                const errorData = await nonceResponse.json().catch(() => ({ message: 'Unknown error' }))
                console.error('Nonce request failed:', errorData)
                throw new Error(errorData.message || 'Failed to get authentication nonce')
            }

            const { message } = await nonceResponse.json()
            console.log('Step 2: Got message to sign:', message)

            // Step 2: Sign the message with wallet (Midnight API only takes the message)
            const signResult = await walletApi.signData(message)
            console.log('Sign result:', signResult)

            const signature = signResult.signature
            console.log('Extracted signature:', signature)

            // Try to get public key from configuration or use wallet address as fallback
            let publicKey = walletState.address // Use address as fallback
            try {
                const config = await walletApi.getConfiguration()
                console.log('Wallet configuration:', config)
                if (config && config.publicKey) {
                    publicKey = config.publicKey
                }
            } catch (err) {
                console.log('Could not get configuration, using address as public key')
            }

            // Step 3: Verify signature with backend
            const verifyPayload = {
                wallet_address: walletState.address,
                signature,
                public_key: publicKey,
            }
            console.log('Step 3: Verifying with payload:', verifyPayload)

            const verifyResponse = await fetch('/api/auth/verify', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(verifyPayload),
            })

            if (!verifyResponse.ok) {
                const errorData = await verifyResponse.json().catch(() => ({ message: 'Unknown error' }))
                console.error('Verify request failed:', verifyResponse.status, errorData)
                throw new Error(errorData.message || 'Authentication failed')
            }

            const { token: authToken, user: userData } = await verifyResponse.json()
            console.log('Authentication successful!')

            // Store token and user
            localStorage.setItem('wagr_token', authToken)
            setToken(authToken)
            setUser(userData)
        } catch (err) {
            const message = err instanceof Error ? err.message : 'Authentication failed'
            console.error('Authentication error:', err)
            setError(message)
            throw err
        }
    }, [walletApi, walletState])

    const disconnect = useCallback(() => {
        localStorage.removeItem('wagr_token')
        setIsConnected(false)
        setWalletState(null)
        setWalletApi(null)
        setUser(null)
        setToken(null)
        setError(null)
    }, [])

    return (
        <MidnightContext.Provider
            value={{
                isConnected,
                isConnecting,
                walletState,
                walletApi,
                user,
                token,
                availableWallets,
                connect,
                disconnect,
                authenticate,
                error,
            }}
        >
            {children}
        </MidnightContext.Provider>
    )
}

export function useMidnight() {
    const context = useContext(MidnightContext)
    if (!context) {
        throw new Error('useMidnight must be used within a MidnightProvider')
    }
    return context
}
