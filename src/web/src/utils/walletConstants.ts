// Wallet type constants and network mappings

import type { WalletType } from '../types/wallet'
import type { MidnightNetworkId } from '../types/midnight'
import type { HederaNetworkId } from '../types/hedera'

export const WALLET_TYPES = {
  MIDNIGHT: 'midnight' as const,
  HEDERA: 'hedera' as const,
}

export const WALLET_DISPLAY_NAMES: Record<WalletType, string> = {
  midnight: 'Midnight',
  hedera: 'Hedera',
}

// Midnight network configurations
export const MIDNIGHT_NETWORKS: Record<MidnightNetworkId, { name: string; isTestnet: boolean }> = {
  mainnet: { name: 'Mainnet', isTestnet: false },
  preprod: { name: 'Pre-production', isTestnet: true },
  preview: { name: 'Preview', isTestnet: true },
  undeployed: { name: 'Undeployed', isTestnet: true },
}

export const MIDNIGHT_DEFAULT_NETWORK: MidnightNetworkId = 'preview'

// Hedera network configurations
export const HEDERA_NETWORKS: Record<HederaNetworkId, { name: string; isTestnet: boolean }> = {
  mainnet: { name: 'Mainnet', isTestnet: false },
  testnet: { name: 'Testnet', isTestnet: true },
  previewnet: { name: 'Previewnet', isTestnet: true },
}

export const HEDERA_DEFAULT_NETWORK: HederaNetworkId = 'testnet'

// Wallet installation links
export const WALLET_INSTALL_LINKS: Record<string, string> = {
  lace: 'https://www.lace.io/',
  hashpack: 'https://www.hashpack.app/',
}

// Wallet display names for specific wallet implementations
export const SPECIFIC_WALLET_NAMES: Record<string, string> = {
  lace: 'Lace Wallet',
  nami: 'Nami Wallet',
  hashpack: 'HashPack',
}

/**
 * Format an address for display (truncate middle for long addresses)
 */
export function formatAddress(address: string, walletType: WalletType): string {
  // Hedera account IDs are short (0.0.12345), display in full
  if (walletType === 'hedera') {
    return address
  }

  // Midnight addresses are long hex strings, truncate
  if (address.length > 16) {
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
