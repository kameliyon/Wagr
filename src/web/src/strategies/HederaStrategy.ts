// Hedera wallet strategy implementation using Hedera Wallet Connect

import type { WalletStrategy, WalletState, SignatureResult, TransferHBARParams, TransferHTSParams, PaymentResult } from '../types/wallet'
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
    signMessage(params: {
        signerAccountId: string
        message: string
    }): Promise<{
        signatureMap: any
    }>
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
        // Always return true - we'll lazy load the library when connecting
        // This avoids loading the heavy Hedera SDK on page load
        return true
    }

    async getAvailableWallets(): Promise<string[]> {
        // Return hashpack as available - WalletConnect will handle the actual connection
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

            // Set up connection detection - only need accountId, public key comes from signing
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

            // Fetch the public key and key type from the Mirror Node API
            // This is the recommended and secure way to get the public key
            const { key: publicKey, keyType } = await this.fetchPublicKeyFromMirrorNode(accountId, networkString)

            return {
                type: 'hedera',
                address: accountId,
                accountId,
                network: networkString,
                publicKey,
                keyType,
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
            // Convert account ID to HIP-30 format: hedera:<network>:<accountId>
            // e.g., "0.0.12345" -> "hedera:testnet:0.0.12345"
            const network = walletState.network || 'testnet'
            const signerAccountId = `hedera:${network}:${walletState.accountId}`

            // Call signMessage with proper params
            const result = await this.connector.signMessage({
                signerAccountId,
                message,
            })

            if (!result || !result.signatureMap) {
                throw new Error('No signature received from wallet')
            }

            // Extract signature from signatureMap
            let signatureHex: string

            if (typeof result.signatureMap === 'string') {
                // It's a base64-encoded protobuf SignaturePair
                // Structure: 0x0a 0x20 [32-byte pubKeyPrefix] 0x1a 0x40 [64-byte signature]
                const signatureBytes = this.base64ToUint8Array(result.signatureMap)

                // Parse the protobuf to extract the signature
                const extracted = this.extractSignatureFromProtobuf(signatureBytes)
                if (extracted) {
                    signatureHex = this.uint8ArrayToHex(extracted)
                } else {
                    // Fallback: use raw bytes (will likely fail verification)
                    console.warn('Could not parse protobuf SignaturePair, using raw bytes')
                    signatureHex = this.uint8ArrayToHex(signatureBytes)
                }
            } else if (result.signatureMap.sigPair) {
                // SignatureMap has sigPair array
                const sigPair = Array.isArray(result.signatureMap.sigPair)
                    ? result.signatureMap.sigPair[0]
                    : result.signatureMap.sigPair

                // Extract signature
                if (sigPair.ed25519) {
                    signatureHex = this.uint8ArrayToHex(sigPair.ed25519)
                } else if (sigPair.ECDSASecp256k1) {
                    signatureHex = this.uint8ArrayToHex(sigPair.ECDSASecp256k1)
                } else {
                    throw new Error('Unsupported signature type in signatureMap')
                }
            } else {
                // Fallback: try to convert entire signatureMap
                signatureHex = JSON.stringify(result.signatureMap)
            }

            // Use the public key and key type from walletState (fetched from mirror node during connect)
            const publicKey = walletState.publicKey
            if (!publicKey) {
                throw new Error('Public key not found in wallet state. Was the wallet connected properly?')
            }

            return {
                signature: signatureHex,
                publicKey,
                keyType: walletState.keyType,
            }
        } catch (err) {
            console.error('Error signing message with Hedera Wallet Connect:', err)
            throw err
        }
    }

    /**
     * Stub: Transfer HBAR between accounts.
     * TODO: implement using Hedera SDK CryptoTransferTransaction via WalletConnect signer.
     */
    async transferHBAR(params: TransferHBARParams): Promise<PaymentResult> {
        console.log('[WAGR Payment Stub] transferHBAR called', params)
        // Stub returns a mock transaction ID — no real transaction is submitted.
        const mockTxId = `0.0.stub@${Date.now()}.000000000`
        return { transactionId: mockTxId, status: 'stub' }
    }

    /**
     * Stub: Transfer an HTS token (e.g. USDC) between accounts.
     * TODO: implement using Hedera SDK TransferTransaction for HTS tokens.
     * ⚠️ IMPORTANT: Both sender and recipient must have associated the token (tokenId)
     * before any transfer can succeed — this is a Hedera-specific requirement with no
     * Ethereum equivalent. The smart contract phase must handle auto-association or
     * pre-associate all accounts before transfers are attempted.
     */
    async transferHTS(params: TransferHTSParams): Promise<PaymentResult> {
        console.log('[WAGR Payment Stub] transferHTS called', params)
        const mockTxId = `0.0.stub@${Date.now()}.000000000`
        return { transactionId: mockTxId, status: 'stub' }
    }

    /**
     * Pay a USDC entry fee via the LeagueEscrow smart contract.
     *
     * Two-step flow signed in HashPack:
     *   1. AccountAllowanceApproveTransaction — approve contract to spend USDC
     *   2. ContractExecuteTransaction — call payEntryFee(bytes32 leagueId, uint256 amount)
     *
     * @returns The Hedera transaction ID of the contract call
     */
    async payEntryFeeUSDC(params: {
        leagueId: string      // WAGR league UUID
        amountUSDC: number    // 6-decimal USDC units (e.g. $50.00 = 50_000_000)
        contractId: string    // Hedera contract ID, e.g. "0.0.12345"
        usdcTokenId: string   // Hedera token ID, e.g. "0.0.456858"
        walletState: WalletState
    }): Promise<{ transactionId: string }> {
        if (!this.connector) {
            throw new Error('Wallet not connected. Call connect() first.')
        }

        const { leagueId, amountUSDC, contractId, usdcTokenId, walletState } = params

        if (!walletState.accountId) {
            throw new Error('Account ID not found in wallet state')
        }

        const {
            AccountAllowanceApproveTransaction,
            ContractExecuteTransaction,
            ContractFunctionParameters,
            AccountId,
            TokenId,
            ContractId,
        } = await import('@hashgraph/sdk')

        const signers = (this.connector as any).signers
        if (!signers || signers.length === 0) {
            throw new Error('No WalletConnect signer available. Is HashPack connected?')
        }
        const signer = signers[0]

        // Convert UUID to bytes32: remove dashes → 32 hex chars (16 bytes), right-pad with zeros to 32 bytes
        const leagueIdHex = leagueId.replace(/-/g, '').padEnd(64, '0')
        const leagueIdBytes32 = new Uint8Array(32)
        for (let i = 0; i < 32; i++) {
            leagueIdBytes32[i] = parseInt(leagueIdHex.substring(i * 2, i * 2 + 2), 16)
        }

        // Step 1: Approve the escrow contract to spend USDC from the user's account
        const approveTx = await new AccountAllowanceApproveTransaction()
            .approveTokenAllowance(
                TokenId.fromString(usdcTokenId),
                AccountId.fromString(walletState.accountId),
                AccountId.fromString(contractId),
                amountUSDC,
            )
            .freezeWithSigner(signer)

        await approveTx.executeWithSigner(signer)

        // Step 2: Call payEntryFee on the escrow contract
        const contractCallTx = await new ContractExecuteTransaction()
            .setContractId(ContractId.fromString(contractId))
            .setGas(150_000)
            .setFunction(
                'payEntryFee',
                new ContractFunctionParameters()
                    .addBytes32(leagueIdBytes32)
                    .addUint256(amountUSDC),
            )
            .freezeWithSigner(signer)

        const response = await contractCallTx.executeWithSigner(signer)

        const transactionId = response.transactionId?.toString() ?? String(response)
        return { transactionId }
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

    private base64ToUint8Array(base64: string): Uint8Array {
        const binaryString = atob(base64)
        const bytes = new Uint8Array(binaryString.length)
        for (let i = 0; i < binaryString.length; i++) {
            bytes[i] = binaryString.charCodeAt(i)
        }
        return bytes
    }

    /**
     * Extract the signature from a protobuf-encoded SignaturePair.
     * Hedera SignaturePair structure:
     * - Field 1 (pubKeyPrefix): 0x0a [length] [bytes]
     * - Field 3 (ed25519): 0x1a [length] [bytes]
     * - Field 4 (ECDSASecp256k1): 0x22 [length] [bytes]
     */
    private extractSignatureFromProtobuf(data: Uint8Array): Uint8Array | null {
        let offset = 0

        // Skip outer wrapper if present (field 1, length-delimited)
        if (data[0] === 0x0a && data.length > 2) {
            const outerLen = data[1]
            if (outerLen === data.length - 2) {
                // It's wrapped, skip the wrapper
                offset = 2
            }
        }

        while (offset < data.length - 2) {
            const fieldTag = data[offset]
            const wireType = fieldTag & 0x07

            if (wireType !== 2) {
                // Not length-delimited, skip
                offset++
                continue
            }

            const length = data[offset + 1]
            const fieldStart = offset + 2
            const fieldEnd = fieldStart + length

            if (fieldEnd > data.length) {
                console.warn('Protobuf field extends beyond data')
                break
            }

            // Field 3 = ed25519 signature (0x1a = (3 << 3) | 2)
            // Field 4 = ECDSA signature (0x22 = (4 << 3) | 2)
            if (fieldTag === 0x1a && length === 64) {
                return data.slice(fieldStart, fieldEnd)
            }
            if (fieldTag === 0x22) {
                return data.slice(fieldStart, fieldEnd)
            }

            offset = fieldEnd
        }

        // Fallback: if data is large enough, try extracting last 64 bytes as signature
        if (data.length >= 100) {
            const sigStart = data.length - 64
            return data.slice(sigStart)
        }

        return null
    }

    /**
     * Fetch the public key and key type for a Hedera account from the Mirror Node API.
     * This is the recommended way to get a trusted public key for verification.
     */
    private async fetchPublicKeyFromMirrorNode(accountId: string, network: string): Promise<{ key: string; keyType: string }> {
        // Determine the mirror node URL based on network
        const mirrorNodeUrls: Record<string, string> = {
            mainnet: 'https://mainnet.mirrornode.hedera.com',
            testnet: 'https://testnet.mirrornode.hedera.com',
            previewnet: 'https://previewnet.mirrornode.hedera.com',
        }

        const baseUrl = mirrorNodeUrls[network] || mirrorNodeUrls.testnet
        const url = `${baseUrl}/api/v1/accounts/${accountId}`

        try {
            const response = await fetch(url)

            if (!response.ok) {
                throw new Error(`Mirror node returned ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()

            // The key field contains: { _type: "ED25519" | "ECDSA_SECP256K1", key: "hex_string" }
            if (data.key && data.key.key) {
                return {
                    key: data.key.key,
                    keyType: data.key._type || 'ED25519', // Default to ED25519 if not specified
                }
            }

            throw new Error('No public key found in mirror node response')
        } catch (err) {
            console.error('Error fetching public key from mirror node:', err)
            throw new Error(`Failed to fetch public key from mirror node: ${err}`)
        }
    }
}
