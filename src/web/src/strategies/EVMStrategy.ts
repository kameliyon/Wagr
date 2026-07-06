// EVM wallet strategy — supports MetaMask, Coinbase Wallet, and WalletConnect

import type { WalletStrategy, WalletState, SignatureResult } from '../types/wallet'
import { HEDERA_EVM_CHAINS } from '../utils/walletConstants'

// Minimal EIP-1193 provider interface
interface EIP1193Provider {
  request(args: { method: string; params?: unknown[] }): Promise<unknown>
  on(event: string, handler: (...args: unknown[]) => void): void
  removeListener(event: string, handler: (...args: unknown[]) => void): void
}

interface InjectedProvider extends EIP1193Provider {
  isMetaMask?: boolean
  isCoinbaseWallet?: boolean
  providers?: InjectedProvider[]
}

// Typed window accessor — avoids conflicting with vite/client's window.ethereum declaration
function getInjectedEthereum(): InjectedProvider | undefined {
  return (window as unknown as { ethereum?: InjectedProvider }).ethereum
}

export class EVMStrategy implements WalletStrategy {
  readonly type = 'evm' as const
  private provider: EIP1193Provider | null = null
  private projectId: string

  constructor(projectId?: string) {
    this.projectId = projectId || ''
  }

  async isAvailable(): Promise<boolean> {
    // Always available: Coinbase SDK works without an extension, WalletConnect covers mobile
    return true
  }

  async getAvailableWallets(): Promise<string[]> {
    const wallets: string[] = []

    const eth = getInjectedEthereum()
    if (eth) {
      // Some browsers inject multiple providers under eth.providers[]
      const providers = eth.providers ?? [eth]
      const hasMetaMask = providers.some((p) => p.isMetaMask && !p.isCoinbaseWallet)
      const hasCoinbaseExt = providers.some((p) => p.isCoinbaseWallet)

      if (hasMetaMask) wallets.push('metamask')
      if (hasCoinbaseExt) wallets.push('coinbase')
    }

    // Coinbase SDK works even without the extension (opens a QR / deeplink)
    if (!wallets.includes('coinbase')) {
      wallets.push('coinbase')
    }

    // WalletConnect covers MetaMask Mobile and other wallets
    if (this.projectId) {
      wallets.push('walletconnect')
    }

    return wallets
  }

  async connect(walletName: string, _networkId?: string): Promise<WalletState> {
    const chain = HEDERA_EVM_CHAINS.testnet

    let provider: EIP1193Provider

    if (walletName === 'metamask') {
      provider = this.getMetaMaskProvider()
    } else if (walletName === 'coinbase') {
      provider = await this.getCoinbaseProvider()
    } else if (walletName === 'walletconnect') {
      provider = await this.getWalletConnectProvider(chain.chainId)
    } else {
      throw new Error(`Unknown EVM wallet: ${walletName}`)
    }

    // Request accounts
    const accounts = (await provider.request({ method: 'eth_requestAccounts' })) as string[]
    if (!accounts || accounts.length === 0) {
      throw new Error('No accounts returned from wallet')
    }

    // Switch/add Hedera network
    await this.switchToHederaNetwork(provider, chain)

    this.provider = provider

    return {
      type: 'evm',
      address: accounts[0],
      network: 'testnet',
      keyType: 'ECDSA_SECP256K1',
    }
  }

  async disconnect(): Promise<void> {
    // EIP-1193 has no standard disconnect; WalletConnect providers expose one
    const p = this.provider as EIP1193Provider & { disconnect?(): Promise<void> }
    if (p?.disconnect) {
      await p.disconnect()
    }
    this.provider = null
  }

  async signMessage(message: string, walletState: WalletState): Promise<SignatureResult> {
    if (!this.provider) throw new Error('Wallet not connected')

    const signature = (await this.provider.request({
      method: 'personal_sign',
      params: [message, walletState.address],
    })) as string

    return { signature, keyType: 'ECDSA_SECP256K1' }
  }

  getDefaultNetwork(): string {
    return 'testnet'
  }

  getSupportedNetworks(): string[] {
    return ['testnet', 'mainnet']
  }

  async payEntryFeeUSDC(params: {
    leagueId: string
    amountUSDC: number
    contractEvmAddress: string
    usdcEvmAddress: string
    walletState: WalletState
    onStage?: (stage: string) => void
  }): Promise<{ transactionId: string }> {
    if (!this.provider) throw new Error('Wallet not connected')

    const { BrowserProvider, Contract } = await import('ethers')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const ethersProvider = new BrowserProvider(this.provider as any)
    const signer = await ethersProvider.getSigner()

    // UUID → bytes32: strip dashes, left-align in 32 bytes, right-pad with zeros
    const leagueIdBytes32 = '0x' + params.leagueId.replace(/-/g, '').padEnd(64, '0')
    const amount = BigInt(params.amountUSDC)

    // Step 1: ensure USDC is associated with the user's Hedera account
    // Hedera HTS tokens require explicit association before approve/transferFrom can work.
    params.onStage?.('Checking USDC token association…')
    await this.ensureTokenAssociated(signer, params.walletState.address, params.usdcEvmAddress, params.onStage)

    // Step 2: approve the escrow to spend USDC
    params.onStage?.('Approving USDC spend in MetaMask…')
    const usdc = new Contract(
      params.usdcEvmAddress,
      ['function approve(address spender, uint256 amount) returns (bool)'],
      signer,
    )
    const approveTx = await usdc.approve(params.contractEvmAddress, amount)
    await approveTx.wait()

    // Step 3: call payEntryFee on the escrow contract
    params.onStage?.('Confirm payment in MetaMask…')
    const escrow = new Contract(
      params.contractEvmAddress,
      ['function payEntryFee(bytes32 leagueId, uint256 amount)'],
      signer,
    )
    const payTx = await escrow.payEntryFee(leagueIdBytes32, amount)
    await payTx.wait()

    return { transactionId: payTx.hash as string }
  }

  // Associates an HTS token with the user's Hedera account via the HTS precompile.
  // On Hedera, accounts must be explicitly associated with tokens before they can
  // set allowances or receive transfers — this is a no-op if already associated.
  private async ensureTokenAssociated(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    signer: any,
    accountAddress: string,
    tokenAddress: string,
    onStage?: (stage: string) => void,
  ): Promise<void> {
    const { Contract } = await import('ethers')
    // HTS system contract precompile — same address on all Hedera networks
    const HTS_PRECOMPILE = '0x0000000000000000000000000000000000000167'
    const hts = new Contract(
      HTS_PRECOMPILE,
      ['function associateToken(address account, address token) external returns (int64 responseCode)'],
      signer,
    )

    // Use staticCall to check first — avoids sending a tx if already associated.
    // Response codes: 22 = SUCCESS, 167 = TOKEN_ALREADY_ASSOCIATED_TO_ACCOUNT
    try {
      const responseCode = await hts.associateToken.staticCall(accountAddress, tokenAddress)
      const code = Number(responseCode)
      if (code === 167) {
        // Already associated — nothing to do
        return
      }
      if (code === 22) {
        // Needs association — send the actual transaction
        onStage?.('Associating USDC with your Hedera account…')
        const tx = await hts.associateToken(accountAddress, tokenAddress)
        await tx.wait()
        return
      }
      // Non-standard response code — log and proceed; the approve will surface any real error
      console.warn('HTS associateToken returned unexpected code:', code)
    } catch {
      // Static call failed (e.g. precompile not available in this context).
      // Proceed and let the approve surface the real error.
    }
  }

  async payEntryFeeHBAR(params: {
    leagueId: string
    amountWeibars: string   // BigInt string from backend; 1 HBAR = 10^18 weibars
    contractEvmAddress: string
    walletState: WalletState
    onStage?: (stage: string) => void
  }): Promise<{ transactionId: string }> {
    if (!this.provider) throw new Error('Wallet not connected')

    const { BrowserProvider, Contract } = await import('ethers')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const ethersProvider = new BrowserProvider(this.provider as any)
    const signer = await ethersProvider.getSigner()

    const leagueIdBytes32 = '0x' + params.leagueId.replace(/-/g, '').padEnd(64, '0')
    const amountWeibars = BigInt(params.amountWeibars)

    params.onStage?.('Confirm HBAR payment in MetaMask…')
    const escrow = new Contract(
      params.contractEvmAddress,
      ['function payEntryFeeHBAR(bytes32 leagueId) payable'],
      signer,
    )
    const tx = await escrow.payEntryFeeHBAR(leagueIdBytes32, { value: amountWeibars })
    await tx.wait()

    return { transactionId: tx.hash as string }
  }

  async claimRefundHBAR(params: {
    leagueId: string
    contractEvmAddress: string
    walletState: WalletState
  }): Promise<{ transactionId: string }> {
    if (!this.provider) throw new Error('Wallet not connected')

    const { BrowserProvider, Contract } = await import('ethers')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const ethersProvider = new BrowserProvider(this.provider as any)
    const signer = await ethersProvider.getSigner()

    const leagueIdBytes32 = '0x' + params.leagueId.replace(/-/g, '').padEnd(64, '0')

    const escrow = new Contract(
      params.contractEvmAddress,
      ['function claimRefundHBAR(bytes32 leagueId)'],
      signer,
    )
    const tx = await escrow.claimRefundHBAR(leagueIdBytes32)
    await tx.wait()

    return { transactionId: tx.hash as string }
  }

  async claimRefund(params: {
    leagueId: string
    contractEvmAddress: string
    walletState: WalletState
  }): Promise<{ transactionId: string }> {
    if (!this.provider) throw new Error('Wallet not connected')

    const { BrowserProvider, Contract } = await import('ethers')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const ethersProvider = new BrowserProvider(this.provider as any)
    const signer = await ethersProvider.getSigner()

    const leagueIdBytes32 = '0x' + params.leagueId.replace(/-/g, '').padEnd(64, '0')

    const escrow = new Contract(
      params.contractEvmAddress,
      ['function claimRefund(bytes32 leagueId)'],
      signer,
    )
    const tx = await escrow.claimRefund(leagueIdBytes32)
    await tx.wait()

    return { transactionId: tx.hash as string }
  }

  // ── private helpers ──────────────────────────────────────────────────────

  private getMetaMaskProvider(): EIP1193Provider {
    const eth = getInjectedEthereum()
    if (!eth) throw new Error('MetaMask not found — please install the extension')

    // When multiple providers are injected, pick the MetaMask one
    if (eth.providers) {
      const mm = eth.providers.find((p) => p.isMetaMask && !p.isCoinbaseWallet)
      if (mm) return mm
    }

    if (!eth.isMetaMask) throw new Error('MetaMask not found')
    return eth
  }

  private async getCoinbaseProvider(): Promise<EIP1193Provider> {
    // If the Coinbase extension is already injected, prefer it
    const eth = getInjectedEthereum()
    if (eth) {
      const providers = eth.providers ?? [eth]
      const ext = providers.find((p) => p.isCoinbaseWallet)
      if (ext) return ext
    }

    // Fall back to the Coinbase Wallet SDK (opens QR code / deeplink)
    const { CoinbaseWalletSDK } = await import('@coinbase/wallet-sdk')
    const sdk = new CoinbaseWalletSDK({ appName: 'WAGRS' })
    return sdk.makeWeb3Provider() as unknown as EIP1193Provider
  }

  private async getWalletConnectProvider(chainId: string): Promise<EIP1193Provider> {
    if (!this.projectId) throw new Error('WalletConnect project ID not configured')

    const { EthereumProvider } = await import('@walletconnect/ethereum-provider')
    const chainIdNum = parseInt(chainId, 16)
    const chain = HEDERA_EVM_CHAINS.testnet

    const provider = await EthereumProvider.init({
      projectId: this.projectId,
      chains: [chainIdNum],
      rpcMap: { [chainIdNum]: chain.rpcUrl },
      showQrModal: true,
      metadata: {
        name: 'WAGRS',
        description: 'Web3 Fantasy Sports Payment Management',
        url: window.location.origin,
        icons: [],
      },
    })

    await provider.enable()
    return provider as unknown as EIP1193Provider
  }

  private async switchToHederaNetwork(
    provider: EIP1193Provider,
    chain: { chainId: string; rpcUrl: string; name: string }
  ): Promise<void> {
    try {
      await provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: chain.chainId }],
      })
    } catch (err: unknown) {
      // Error code 4902 means the chain is not yet added to the wallet
      const code = (err as { code?: number })?.code
      if (code === 4902) {
        await provider.request({
          method: 'wallet_addEthereumChain',
          params: [
            {
              chainId: chain.chainId,
              chainName: chain.name,
              rpcUrls: [chain.rpcUrl],
              nativeCurrency: { name: 'HBAR', symbol: 'HBAR', decimals: 8 },
              blockExplorerUrls: ['https://hashscan.io/testnet'],
            },
          ],
        })
      } else {
        throw err
      }
    }
  }
}
