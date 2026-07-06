// Wallet type constants and network mappings

import type { WalletType } from '../types/wallet'
import type { HederaNetworkId } from '../types/hedera'

export const WALLET_TYPES = {
  HEDERA: 'hedera' as const,
  EVM: 'evm' as const,
}

export const WALLET_DISPLAY_NAMES: Record<WalletType, string> = {
  hedera: 'Hedera',
  evm: 'EVM',
}

// Hedera JSON-RPC relay endpoints for EVM-compatible wallets (MetaMask, Coinbase, etc.)
export const HEDERA_EVM_CHAINS = {
  testnet: { chainId: '0x128', rpcUrl: 'https://testnet.hashio.io/api', name: 'Hedera Testnet' },
  mainnet: { chainId: '0x127', rpcUrl: 'https://mainnet.hashio.io/api', name: 'Hedera Mainnet' },
} as const

// Hedera network configurations
export const HEDERA_NETWORKS: Record<HederaNetworkId, { name: string; isTestnet: boolean }> = {
  mainnet: { name: 'Mainnet', isTestnet: false },
  testnet: { name: 'Testnet', isTestnet: true },
  previewnet: { name: 'Previewnet', isTestnet: true },
}

export const HEDERA_DEFAULT_NETWORK: HederaNetworkId = 'testnet'

// Wallet installation links
export const WALLET_INSTALL_LINKS: Record<string, string> = {
  hashpack: 'https://www.hashpack.app/',
  metamask: 'https://metamask.io/',
  coinbase: 'https://www.coinbase.com/wallet',
}

// Wallet display names for specific wallet implementations
export const SPECIFIC_WALLET_NAMES: Record<string, string> = {
  hashpack: 'HashPack',
  metamask: 'MetaMask',
  coinbase: 'Coinbase Wallet',
  walletconnect: 'WalletConnect',
}

/**
 * Format an address for display.
 * EVM addresses (0x...) are truncated; Hedera account IDs shown in full.
 */
export function formatAddress(address: string, walletType: WalletType): string {
  if (walletType === 'evm' && address.startsWith('0x') && address.length === 42) {
    return `${address.slice(0, 6)}...${address.slice(-4)}`
  }
  return address
}

/**
 * Get user-friendly wallet name
 */
export function getWalletDisplayName(walletName: string): string {
  return SPECIFIC_WALLET_NAMES[walletName.toLowerCase()] || walletName
}

/**
 * Get install link for a wallet
 */
export function getWalletInstallLink(walletName: string): string | undefined {
  return WALLET_INSTALL_LINKS[walletName.toLowerCase()]
}
