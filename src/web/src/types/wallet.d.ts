// Common wallet type definitions

export type WalletType = 'hedera' | 'evm'

export interface WalletState {
  type: WalletType
  address: string
  accountId?: string // Format: 0.0.12345
  balance?: string
  network?: string
  publicKey?: string
  keyType?: string // 'ED25519' or 'ECDSA_SECP256K1'
}

export interface SignatureResult {
  signature: string
  keyType?: string // 'ED25519' or 'ECDSA_SECP256K1'
}

export interface AvailableWallet {
  type: WalletType
  name: string // Internal wallet name (e.g., 'lace', 'hashpack')
  displayName: string // User-facing name (e.g., 'Lace Wallet', 'HashPack')
  icon?: string
}

export interface TransferHBARParams {
  fromAccountId: string
  toAccountId: string
  amountTinybars: number
  memo?: string
}

export interface TransferHTSParams {
  fromAccountId: string
  toAccountId: string
  tokenId: string       // Hedera token ID, e.g. "0.0.456858"
  amount: number        // In smallest unit (micro-USDC for USDC: $1.00 = 1_000_000)
  memo?: string
}

export interface PaymentResult {
  transactionId: string
  status: string
}

export interface WalletStrategy {
  readonly type: WalletType

  /**
   * Check if this wallet type is available in the browser
   */
  isAvailable(): Promise<boolean>

  /**
   * Get list of available wallet names for this type
   * @returns Array of wallet names (e.g., ['hashpack'] for Hedera)
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

  /**
   * Transfer HBAR to another account (optional — Hedera-specific)
   * TODO: implement using Hedera SDK CryptoTransferTransaction
   */
  transferHBAR?(params: TransferHBARParams): Promise<PaymentResult>

  /**
   * Transfer an HTS token (e.g. USDC) to another account (optional — Hedera-specific)
   * NOTE: Both sender and recipient must have associated the token before transfer.
   * This is a Hedera-specific requirement with no Ethereum equivalent.
   * TODO: implement using Hedera SDK TransferTransaction for HTS tokens
   */
  transferHTS?(params: TransferHTSParams): Promise<PaymentResult>
}

export interface WalletConfig {
  walletConnectProjectId?: string
  defaultNetwork?: Record<WalletType, string>
}

export {}
