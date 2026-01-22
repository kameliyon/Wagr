// Type definitions for Midnight wallet connector API

export interface MidnightWalletState {
  address: string
  publicKey: string
  balance?: string
}

export interface MidnightWalletAPI {
  state(): Promise<MidnightWalletState>
  signData(data: string): Promise<{ signature: string }>
}

export interface MidnightWalletConnector {
  enable(): Promise<MidnightWalletAPI>
  isEnabled(): Promise<boolean>
  name: string
  icon: string
}

declare global {
  interface Window {
    midnight?: {
      [walletName: string]: MidnightWalletConnector
    }
  }
}

export {}
