import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type { MidnightWalletAPI, MidnightWalletState } from '../types/midnight'

interface User {
  id: string
  wallet_address: string
  created_at: string
  updated_at: string
}

interface MidnightContextType {
  isConnected: boolean
  isConnecting: boolean
  walletState: MidnightWalletState | null
  walletApi: MidnightWalletAPI | null
  user: User | null
  token: string | null
  availableWallets: string[]
  connect: (walletName: string) => Promise<void>
  disconnect: () => void
  authenticate: () => Promise<void>
  error: string | null
}

const MidnightContext = createContext<MidnightContextType | null>(null)

export function MidnightProvider({ children }: { children: ReactNode }) {
  const [isConnected, setIsConnected] = useState(false)
  const [isConnecting, setIsConnecting] = useState(false)
  const [walletState, setWalletState] = useState<MidnightWalletState | null>(null)
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

  const connect = useCallback(async (walletName: string) => {
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

      const api = await wallet.enable()
      const state = await api.state()

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
      const nonceResponse = await fetch('/api/auth/nonce', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ wallet_address: walletState.address }),
      })

      if (!nonceResponse.ok) {
        throw new Error('Failed to get authentication nonce')
      }

      const { message } = await nonceResponse.json()

      // Step 2: Sign the message with wallet
      const { signature } = await walletApi.signData(message)

      // Step 3: Verify signature with backend
      const verifyResponse = await fetch('/api/auth/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          wallet_address: walletState.address,
          signature,
          public_key: walletState.publicKey,
        }),
      })

      if (!verifyResponse.ok) {
        const error = await verifyResponse.json()
        throw new Error(error.message || 'Authentication failed')
      }

      const { token: authToken, user: userData } = await verifyResponse.json()

      // Store token and user
      localStorage.setItem('wagr_token', authToken)
      setToken(authToken)
      setUser(userData)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Authentication failed'
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
