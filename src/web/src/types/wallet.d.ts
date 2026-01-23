// Common wallet type definitions for multi-wallet support

export type WalletType = 'midnight' | 'hedera'

export interface WalletState {
  type: WalletType
  address: string
  accountId?: string // For Hedera (format: 0.0.12345)
  balance?: string
  network?: string
  publicKey?: string
}

export interface SignatureResult {
  signature: string
  publicKey: string
}

export interface AvailableWallet {
  type: WalletType
  name: string // Internal wallet name (e.g., 'lace', 'hashpack')
  displayName: string // User-facing name (e.g., 'Lace Wallet', 'HashPack')
  icon?: string
}

export interface WalletStrategy {
  readonly type: WalletType

  /**
   * Check if this wallet type is available in the browser
   */
  isAvailable(): Promise<boolean>

  /**
   * Get list of available wallet names for this type
   * @returns Array of wallet names (e.g., ['lace', 'nami'] for Midnight)
   */
  getAvailableWallets(): Promise<string[]>

  /**
   * Connect to a specific wallet
   * @param walletName - The internal name of the wallet to connect
   * @param networkId - Optional network identifier (wallet-specific format)
   * @returns Connected wallet state
   */
  connect(walletName: string, networkId?: string): Promise<WalletState>

  /**
   * Disconnect from the current wallet
   */
  disconnect(): Promise<void>

  /**
   * Sign a message with the connected wallet
   * @param message - The message to sign
   * @param walletState - Current wallet state
   * @returns Signature result with signature and public key
   */
  signMessage(message: string, walletState: WalletState): Promise<SignatureResult>

  /**
   * Get the default network ID for this wallet type
   */
  getDefaultNetwork(): string

  /**
   * Get list of supported network IDs for this wallet type
   */
  getSupportedNetworks(): string[]
}

export interface WalletConfig {
  walletConnectProjectId?: string
  defaultNetwork?: Record<WalletType, string>
}

export {}
