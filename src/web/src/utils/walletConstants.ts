// Wallet type constants and network mappings

import type { WalletType } from '../types/wallet'
import type { HederaNetworkId } from '../types/hedera'

export const WALLET_TYPES = {
  HEDERA: 'hedera' as const,
}

export const WALLET_DISPLAY_NAMES: Record<WalletType, string> = {
  hedera: 'Hedera',
}

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
}

// Wallet display names for specific wallet implementations
export const SPECIFIC_WALLET_NAMES: Record<string, string> = {
  hashpack: 'HashPack',
}

/**
 * Format an address for display
 */
export function formatAddress(address: string, _walletType: WalletType): string {
  // Hedera account IDs are short (0.0.12345), display in full
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
