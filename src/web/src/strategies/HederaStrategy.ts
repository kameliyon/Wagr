// Hedera wallet strategy implementation using Hedera Wallet Connect

import type { WalletStrategy, WalletState, SignatureResult } from '../types/wallet'
import type { HederaNetworkId } from '../types/hedera'
import { HEDERA_DEFAULT_NETWORK } from '../utils/walletConstants'

// Hedera Wallet Connect types
interface DAppConnector {
  init(params: {
    name: string
    description: string
    url: string
    icons: string[]
  }): Promise<void>
  openModal(): Promise<void>
  closeModal(): void
  disconnect(): Promise<void>
  signers?: any[]
  signMessages(messages: Uint8Array[]): Promise<Uint8Array[]>
  onSessionConnected?: (callback: (session: any) => void) => void
}

// Dynamic import placeholder
let HederaWalletConnect: any = null

export class HederaStrategy implements WalletStrategy {
  readonly type = 'hedera' as const
  private connector: DAppConnector | null = null
  private projectId: string = ''
  private connectionTimeout: NodeJS.Timeout | null = null

  constructor(projectId?: string) {
    this.projectId = projectId || ''
  }

  async isAvailable(): Promise<boolean> {
    try {
      if (!HederaWalletConnect) {
        const module = await import('@hashgraph/hedera-wallet-connect')
        HederaWalletConnect = module
      }
      return true
    } catch (err) {
      console.error('Hedera Wallet Connect library not available:', err)
      return false
    }
  }

  async getAvailableWallets(): Promise<string[]> {
    return ['hashpack']
  }

  async connect(_walletName: string, networkId?: string): Promise<WalletState> {
    if (!this.projectId) {
      throw new Error('WalletConnect Project ID not configured. Please set VITE_WALLETCONNECT_PROJECT_ID.')
    }

    // Load required modules
    if (!HederaWalletConnect) {
      const module = await import('@hashgraph/hedera-wallet-connect')
      HederaWalletConnect = module
    }

    const { LedgerId } = await import('@hashgraph/sdk')
    const networkString = (networkId || HEDERA_DEFAULT_NETWORK) as HederaNetworkId

    // Convert network string to LedgerId
    const ledgerId = networkString === 'mainnet' ? LedgerId.MAINNET
      : networkString === 'previewnet' ? LedgerId.PREVIEWNET
      : LedgerId.TESTNET

    try {
      const { DAppConnector } = HederaWalletConnect

      if (!DAppConnector) {
        throw new Error('DAppConnector not found in @hashgraph/hedera-wallet-connect')
      }

      // Create connector
      this.connector = new DAppConnector(
        {
          name: 'WAGR',
          description: 'Web3 Fantasy Sports Payment Management',
          url: window.location.origin,
          icons: ['https://wagr.app/icon.png'],
        },
        ledgerId,
        this.projectId
      )

      if (!this.connector) {
        throw new Error('Failed to create DApp connector')
      }

      // Set up connection detection
      const connectionPromise = new Promise<string>((resolve, reject) => {
        this.connectionTimeout = setTimeout(() => {
          clearInterval(pollInterval)
          reject(new Error('Connection timeout. Please approve the connection in HashPack.'))
        }, 90000)

        // Listen for session connection event
        if (this.connector?.onSessionConnected) {
          this.connector.onSessionConnected((session: any) => {
            if (this.connectionTimeout) {
              clearTimeout(this.connectionTimeout)
              this.connectionTimeout = null
            }
            clearInterval(pollInterval)

            const accountId = session?.accountIds?.[0]
            if (accountId) {
              resolve(accountId)
            } else {
              reject(new Error('No account ID in session'))
            }
          })
        }

        // Polling fallback - check for connected signers
        const pollInterval = setInterval(() => {
          if ((this.connector as any)?.signers?.length > 0) {
            const signer = (this.connector as any).signers[0]
            let accountId: string | null = null

            // Extract account ID from signer
            if (signer.getAccountId) {
              accountId = signer.getAccountId().toString()
            } else if (signer.accountId) {
              accountId = typeof signer.accountId === 'string'
                ? signer.accountId
                : signer.accountId.toString()
            }

            if (accountId) {
              if (this.connectionTimeout) {
                clearTimeout(this.connectionTimeout)
                this.connectionTimeout = null
              }
              clearInterval(pollInterval)
              resolve(accountId)
            }
          }
        }, 1000)
      })

      // Initialize and open modal
      await this.connector.init({
        name: 'WAGR',
        description: 'Web3 Fantasy Sports Payment Management',
        url: window.location.origin,
        icons: ['https://wagr.app/icon.png'],
      })

      await this.connector.openModal()

      // Wait for connection
      const accountId = await connectionPromise

      // Close modal
      try {
        this.connector.closeModal()
      } catch (err) {
        console.error('Error closing modal:', err)
      }

      return {
        type: 'hedera',
        address: accountId,
        accountId,
        network: networkString,
      }
    } catch (err) {
      // Clean up on error
      if (this.connectionTimeout) {
        clearTimeout(this.connectionTimeout)
        this.connectionTimeout = null
      }
      if (this.connector) {
        try {
          this.connector.closeModal()
        } catch (closeErr) {
          console.error('Error closing modal:', closeErr)
        }
      }
      throw err
    }
  }

  async disconnect(): Promise<void> {
    if (this.connector) {
      try {
        await this.connector.disconnect()
      } catch (err) {
        console.error('Error disconnecting from Hedera Wallet Connect:', err)
      }
    }
    this.connector = null

    if (this.connectionTimeout) {
      clearTimeout(this.connectionTimeout)
      this.connectionTimeout = null
    }
  }

  async signMessage(message: string, walletState: WalletState): Promise<SignatureResult> {
    if (!this.connector) {
      throw new Error('Wallet not connected. Call connect() first.')
    }

    if (!walletState.accountId) {
      throw new Error('Account ID not found in wallet state')
    }

    try {
      const messageBytes = new TextEncoder().encode(message)
      const signatures = await this.connector.signMessages([messageBytes])

      if (!signatures || signatures.length === 0) {
        throw new Error('No signature received from wallet')
      }

      const signatureHex = this.uint8ArrayToHex(signatures[0])

      return {
        signature: signatureHex,
        publicKey: walletState.accountId,
      }
    } catch (err) {
      console.error('Error signing message with Hedera Wallet Connect:', err)
      throw err
    }
  }

  getDefaultNetwork(): string {
    return HEDERA_DEFAULT_NETWORK
  }

  getSupportedNetworks(): string[] {
    return ['mainnet', 'testnet', 'previewnet']
  }

  private uint8ArrayToHex(bytes: Uint8Array): string {
    return Array.from(bytes)
      .map((b) => b.toString(16).padStart(2, '0'))
      .join('')
  }
}
