// Type definitions for Midnight wallet connector API

export type MidnightNetworkId = 'mainnet' | 'preprod' | 'preview' | 'undeployed'

export interface MidnightWalletAPI {
  // Address methods
  getUnshieldedAddress(): Promise<MidnightUnsheildedAddress>
  getShieldedAddresses(): Promise<string[]>
  getDustAddress(): Promise<string>

  // Balance methods
  getUnshieldedBalances(): Promise<any>
  getShieldedBalances(): Promise<any>
  getDustBalance(): Promise<any>

  // Transaction methods
  getTxHistory(): Promise<any[]>
  balanceUnsealedTransaction(tx: any): Promise<any>
  balanceSealedTransaction(tx: any): Promise<any>
  makeTransfer(params: any): Promise<any>
  makeIntent(params: any): Promise<any>
  submitTransaction(tx: any): Promise<any>

  // Signing
  signData(data: string): Promise<{ signature: string }>

  // Configuration
  getConfiguration(): Promise<any>
  getConnectionStatus(): Promise<any>
  hintUsage(hint: any): Promise<void>
}

export interface MidnightWalletConnector {
  connect(networkId?: MidnightNetworkId): Promise<MidnightWalletAPI>
  isEnabled(): Promise<boolean>
  name: string
  icon: string
  apiVersion: string
}

export interface MidnightUnsheildedAddress {
    unshieldedAddress: string
}

declare global {
  interface Window {
    midnight?: {
      [walletName: string]: MidnightWalletConnector
    }
  }
}

export {}
