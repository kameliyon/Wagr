import { useWallet as useWalletContext } from '../providers/WalletProvider'

// Re-export the hook with additional computed properties
export function useWallet() {
  const wallet = useWalletContext()

  return {
    // Connection state
    isConnected: wallet.isConnected,
    isConnecting: wallet.isConnecting,
    isAuthenticating: wallet.isAuthenticating,

    // Wallet data
    walletType: wallet.walletState?.type ?? null,
    address: wallet.walletState?.address ?? null,
    accountId: wallet.walletState?.accountId ?? null, // For Hedera
    balance: wallet.walletState?.balance ?? null,
    network: wallet.walletState?.network ?? null,
    walletState: wallet.walletState,
    activeStrategy: wallet.activeStrategy,

    // Auth data
    user: wallet.user,
    token: wallet.token,
    isAuthenticated: !!wallet.user,

    // Available wallets
    availableWallets: wallet.availableWallets,
    hasWallet: wallet.availableWallets.length > 0,

    // Actions
    connect: wallet.connect,
    disconnect: wallet.disconnect,
    authenticate: wallet.authenticate,

    // Error state
    error: wallet.error,
  }
}
