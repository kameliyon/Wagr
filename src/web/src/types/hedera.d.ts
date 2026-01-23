// Type definitions for Hedera HashConnect integration

export type HederaNetworkId = 'mainnet' | 'testnet' | 'previewnet'

export interface HederaAccountId {
  shard: number
  realm: number
  num: number
  toString(): string // Returns format: "0.0.12345"
}

export interface HederaSignatureResult {
  signature: Uint8Array
  publicKey: string
}

export interface HederaConnectionState {
  accountId: string // Format: "0.0.12345"
  network: HederaNetworkId
  publicKey?: string
}

// HashConnect types (simplified from @hashgraph/hashconnect)
export interface HashConnectConnectionState {
  topic: string
  pairingString: string
  pairingData: {
    accountIds: string[]
    network: string
    metadata?: {
      name: string
      description: string
      icons: string[]
    }
  }
}

export interface HashConnectSignMessageParams {
  topic: string
  signingAcctId: string
  message: string
}

export interface HashConnectSignMessageResponse {
  success: boolean
  signedMessage?: {
    signature: Uint8Array
    publicKey: string
  }
  error?: string
}

export {}
