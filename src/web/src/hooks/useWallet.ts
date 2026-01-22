import { useMidnight } from '../providers/MidnightProvider'

// Re-export the hook for convenience and potential future abstraction
export function useWallet() {
  const midnight = useMidnight()

  return {
    // Connection state
    isConnected: midnight.isConnected,
    isConnecting: midnight.isConnecting,

    // Wallet data
    address: midnight.walletState?.address ?? null,
    balance: midnight.walletState?.balance ?? null,

    // Auth data
    user: midnight.user,
    token: midnight.token,
    isAuthenticated: !!midnight.user,

    // Available wallets
    availableWallets: midnight.availableWallets,
    hasWallet: midnight.availableWallets.length > 0,

    // Actions
    connect: midnight.connect,
    disconnect: midnight.disconnect,
    authenticate: midnight.authenticate,

    // Error state
    error: midnight.error,
  }
}
