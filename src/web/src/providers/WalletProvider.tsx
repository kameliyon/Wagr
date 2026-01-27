// Unified wallet provider supporting multiple wallet types

import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type { WalletType, WalletState, WalletStrategy, AvailableWallet } from '../types/wallet'
import { MidnightStrategy } from '../strategies/MidnightStrategy'
import { HederaStrategy } from '../strategies/HederaStrategy'
import { useWalletConfig } from './WalletConfig'
import { getWalletDisplayName } from '../utils/walletConstants'

interface User {
  id: string
  wallet_address: string
  wallet_type: string
  created_at: string
  updated_at: string
}

interface WalletContextType {
  isConnected: boolean
  isConnecting: boolean
  walletState: WalletState | null
  user: User | null
  token: string | null
  availableWallets: AvailableWallet[]
  error: string | null

  connect: (walletType: WalletType, walletName: string, networkId?: string) => Promise<void>
  disconnect: () => void
  authenticate: () => Promise<void>
}

const WalletContext = createContext<WalletContextType | null>(null)

export function WalletProvider({ children }: { children: ReactNode }) {
  const config = useWalletConfig()
  const [isConnected, setIsConnected] = useState(false)
  const [isConnecting, setIsConnecting] = useState(false)
  const [walletState, setWalletState] = useState<WalletState | null>(null)
  const [activeStrategy, setActiveStrategy] = useState<WalletStrategy | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('wagr_token'))
  const [availableWallets, setAvailableWallets] = useState<AvailableWallet[]>([])
  const [error, setError] = useState<string | null>(null)

  // Initialize strategies
  const [strategies] = useState<Record<WalletType, WalletStrategy>>({
    midnight: new MidnightStrategy(),
    hedera: new HederaStrategy(config.walletConnectProjectId),
  })

  // Detect available wallets on mount
  useEffect(() => {
    const detectWallets = async () => {
      const detected: AvailableWallet[] = []

      // Check each strategy
      for (const [walletType, strategy] of Object.entries(strategies)) {
        try {
          const isAvailable = await strategy.isAvailable()
          if (isAvailable) {
            const wallets = await strategy.getAvailableWallets()
            for (const walletName of wallets) {
              detected.push({
                type: walletType as WalletType,
                name: walletName,
                displayName: getWalletDisplayName(walletName),
              })
            }
          }
        } catch (err) {
          console.error(`Error detecting ${walletType} wallets:`, err)
        }
      }

      console.log('Available wallets detected:', detected)
      setAvailableWallets(detected)
    }

    // Check immediately and after a delay (wallets may inject after page load)
    detectWallets()
    const timeout = setTimeout(detectWallets, 1500)
    return () => clearTimeout(timeout)
  }, [strategies])

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

  const connect = useCallback(
    async (walletType: WalletType, walletName: string, networkId?: string) => {
      setIsConnecting(true)
      setError(null)

      try {
        const strategy = strategies[walletType]
        if (!strategy) {
          throw new Error(`Unsupported wallet type: ${walletType}`)
        }

        console.log(`Connecting to ${walletType} wallet: ${walletName}`)

        // Use default network from config if not specified
        const network = networkId || config.defaultNetwork?.[walletType]

        // Connect using the appropriate strategy
        const state = await strategy.connect(walletName, network)
        console.log('Wallet connected:', state)

        setWalletState(state)
        setActiveStrategy(strategy)
        setIsConnected(true)
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to connect wallet'
        console.error('Connection error:', err)
        setError(message)
        throw err
      } finally {
        setIsConnecting(false)
      }
    },
    [strategies, config]
  )

  const authenticate = useCallback(async () => {
    if (!activeStrategy || !walletState) {
      throw new Error('Wallet not connected')
    }

    setError(null)

    try {
      // Step 1: Get nonce from backend
      console.log('Step 1: Getting nonce for wallet:', walletState)
      const nonceResponse = await fetch('/api/auth/nonce', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          wallet_address: walletState.address,
          wallet_type: walletState.type,
        }),
      })

      if (!nonceResponse.ok) {
        const errorData = await nonceResponse.json().catch(() => ({ message: 'Unknown error' }))
        console.error('Nonce request failed:', errorData)
        throw new Error(errorData.message || 'Failed to get authentication nonce')
      }

      const { message } = await nonceResponse.json()

      // Step 2: Sign the message with wallet
      const signResult = await activeStrategy.signMessage(message, walletState)
      console.log('Step 3: Signature obtained:', {
        signature: signResult.signature.substring(0, 20) + '...',
        publicKey: signResult.publicKey,
      })

      // Step 3: Verify signature with backend
      const verifyPayload = {
        wallet_address: walletState.address,
        wallet_type: walletState.type,
        signature: signResult.signature,
        public_key: signResult.publicKey,
        key_type: signResult.keyType, // 'ED25519' or 'ECDSA_SECP256K1' for Hedera
      }
      console.log('Step 4: Verifying signature...', { keyType: signResult.keyType })

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
  }, [activeStrategy, walletState])

  const disconnect = useCallback(async () => {
    // Disconnect the active strategy
    if (activeStrategy) {
      try {
        await activeStrategy.disconnect()
      } catch (err) {
        console.error('Error disconnecting wallet:', err)
      }
    }

    // Clear state
    localStorage.removeItem('wagr_token')
    setIsConnected(false)
    setWalletState(null)
    setActiveStrategy(null)
    setUser(null)
    setToken(null)
    setError(null)
  }, [activeStrategy])

  return (
    <WalletContext.Provider
      value={{
        isConnected,
        isConnecting,
        walletState,
        user,
        token,
        availableWallets,
        error,
        connect,
        disconnect,
        authenticate,
      }}
    >
      {children}
    </WalletContext.Provider>
  )
}

export function useWallet(): WalletContextType {
  const context = useContext(WalletContext)
  if (!context) {
    throw new Error('useWallet must be used within a WalletProvider')
  }
  return context
}
