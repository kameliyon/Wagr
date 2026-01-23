// Midnight wallet strategy implementation

import type { WalletStrategy, WalletState, SignatureResult } from '../types/wallet'
import type { MidnightWalletAPI, MidnightNetworkId } from '../types/midnight'
import { MIDNIGHT_DEFAULT_NETWORK } from '../utils/walletConstants'

export class MidnightStrategy implements WalletStrategy {
  readonly type = 'midnight' as const
  private walletApi: MidnightWalletAPI | null = null

  async isAvailable(): Promise<boolean> {
    return typeof window !== 'undefined' && window.midnight !== undefined
  }

  async getAvailableWallets(): Promise<string[]> {
    if (!window.midnight) {
      return []
    }
    return Object.keys(window.midnight)
  }

  async connect(walletName: string, networkId?: string): Promise<WalletState> {
    if (!window.midnight) {
      throw new Error('Midnight wallet extension not detected. Please install a compatible wallet.')
    }

    const wallet = window.midnight[walletName]
    if (!wallet) {
      const available = await this.getAvailableWallets()
      throw new Error(`Wallet "${walletName}" not found. Available wallets: ${available.join(', ')}`)
    }

    // Connect to wallet
    const network = (networkId || MIDNIGHT_DEFAULT_NETWORK) as MidnightNetworkId
    const api = await wallet.connect(network)
    this.walletApi = api

    // Get wallet address
    const addressResult = await api.getUnshieldedAddress()
    const address = addressResult.unshieldedAddress

    // Get balances (optional)
    let balance: string | undefined
    try {
      const unshieldedBalances = await api.getUnshieldedBalances()
      balance = JSON.stringify(unshieldedBalances)
    } catch {
      // Balance fetch is optional, ignore errors
    }

    // Get public key from configuration
    let publicKey: string | undefined
    try {
      const config = await api.getConfiguration()
      if (config && config.publicKey) {
        publicKey = config.publicKey
      }
    } catch {
      // Public key from config is optional, will use address as fallback
    }

    return {
      type: 'midnight',
      address,
      balance,
      network,
      publicKey: publicKey || address,
    }
  }

  async disconnect(): Promise<void> {
    this.walletApi = null
  }

  async signMessage(message: string, walletState: WalletState): Promise<SignatureResult> {
    if (!this.walletApi) {
      throw new Error('Wallet not connected. Call connect() first.')
    }

    const signResult = await this.walletApi.signData(message)
    let publicKey = walletState.publicKey || walletState.address

    // Try to get public key from configuration if not in wallet state
    if (!walletState.publicKey) {
      try {
        const config = await this.walletApi.getConfiguration()
        if (config && config.publicKey) {
          publicKey = config.publicKey
        }
      } catch {
        // Use address as fallback
      }
    }

    return {
      signature: signResult.signature,
      publicKey,
    }
  }

  getDefaultNetwork(): string {
    return MIDNIGHT_DEFAULT_NETWORK
  }

  getSupportedNetworks(): string[] {
    return ['mainnet', 'preprod', 'preview', 'undeployed']
  }
}
