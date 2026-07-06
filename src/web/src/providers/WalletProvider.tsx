// Wallet provider for Hedera wallet integration

import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type { WalletType, WalletState, WalletStrategy, AvailableWallet } from '../types/wallet'
import { HederaStrategy } from '../strategies/HederaStrategy'
import { EVMStrategy } from '../strategies/EVMStrategy'
import { useWalletConfig } from './WalletConfig'
import { getWalletDisplayName } from '../utils/walletConstants'
import { apiUrl } from '../utils/api'

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
  isAuthenticating: boolean
  walletState: WalletState | null
  activeStrategy: WalletStrategy | null
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
  const [isAuthenticating, setIsAuthenticating] = useState(false)
  const [walletState, setWalletState] = useState<WalletState | null>(null)
  const [activeStrategy, setActiveStrategy] = useState<WalletStrategy | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('wagr_token'))
  const [availableWallets, setAvailableWallets] = useState<AvailableWallet[]>([])
  const [error, setError] = useState<string | null>(null)

  // Initialize strategies
  const [strategies] = useState<Record<WalletType, WalletStrategy>>({
    hedera: new HederaStrategy(config.walletConnectProjectId),
    evm: new EVMStrategy(config.walletConnectProjectId),
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
      const response = await fetch(apiUrl('/api/auth/me'), {
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

  const authenticateWith = useCallback(
    async (strategy: WalletStrategy, state: WalletState) => {
      // Step 1: Get nonce
      const nonceResponse = await fetch(apiUrl('/api/auth/nonce'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ wallet_address: state.address, wallet_type: state.type }),
      })
      if (!nonceResponse.ok) {
        const errorData = await nonceResponse.json().catch(() => ({ message: 'Unknown error' }))
        throw new Error(errorData.message || 'Failed to get authentication nonce')
      }
      const { message } = await nonceResponse.json()

      // Step 2: Sign
      const signResult = await strategy.signMessage(message, state)

      // Step 3: Verify
      const verifyResponse = await fetch(apiUrl('/api/auth/verify'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          wallet_address: state.address,
          message: message,
          signature: signResult.signature,
          wallet_type: state.type,
        }),
      })
      if (!verifyResponse.ok) {
        const errorData = await verifyResponse.json().catch(() => ({ message: 'Unknown error' }))
        throw new Error(errorData.message || 'Authentication failed')
      }
      const { token: authToken, user: userData } = await verifyResponse.json()
      localStorage.setItem('wagr_token', authToken)
      setToken(authToken)
      setUser(userData)
    },
    [] // reads only from parameters
  )

  const connect = useCallback(
    async (walletType: WalletType, walletName: string, networkId?: string) => {
      setIsConnecting(true)
      setError(null)

      let connectedStrategy: WalletStrategy | null = null
      let connectedState: WalletState | null = null

      try {
        const strategy = strategies[walletType]
        if (!strategy) throw new Error(`Unsupported wallet type: ${walletType}`)

        const network = networkId || config.defaultNetwork?.[walletType]
        const state = await strategy.connect(walletName, network)

        connectedStrategy = strategy
        connectedState = state

        setWalletState(state)
        setActiveStrategy(strategy)
        setIsConnected(true)
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to connect wallet'
        // User dismissed the modal — not an error worth displaying
        if (message !== 'Connection cancelled') {
          setError(message)
        }
        throw err
      } finally {
        setIsConnecting(false)
      }

      // Auth runs after the connection finally block so isConnecting is false
      // Any auth error only sets error — wallet stays connected for retry
      setIsAuthenticating(true)
      try {
        await authenticateWith(connectedStrategy!, connectedState!)
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Authentication failed'
        setError(message)
        // Do NOT throw — wallet remains connected
      } finally {
        setIsAuthenticating(false)
      }
    },
    [strategies, config, authenticateWith]
  )

  const authenticate = useCallback(async () => {
    if (!activeStrategy || !walletState) throw new Error('Wallet not connected')
    setError(null)
    setIsAuthenticating(true)
    try {
      await authenticateWith(activeStrategy, walletState)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Authentication failed'
      setError(message)
      throw err
    } finally {
      setIsAuthenticating(false)
    }
  }, [activeStrategy, walletState, authenticateWith])

  const disconnect = useCallback(async () => {
    // Disconnect the active strategy
    if (activeStrategy) {
      try {
        await activeStrategy.disconnect()
      } catch (err) {
        console.error('Error disconnecting wallet:', err)
      }
    }

    // Clear all state — including isConnecting/isAuthenticating so this
    // doubles as a Cancel when the user dismisses a connection modal
    localStorage.removeItem('wagr_token')
    setIsConnecting(false)
    setIsAuthenticating(false)
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
        isAuthenticating,
        walletState,
        activeStrategy,
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
