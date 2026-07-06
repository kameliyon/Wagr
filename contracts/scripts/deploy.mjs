import { createRequire } from 'module'
import { readFileSync } from 'fs'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'

const require = createRequire(import.meta.url)
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

const {
  Client,
  AccountId,
  PrivateKey,
  Hbar,
  FileCreateTransaction,
  FileAppendTransaction,
  ContractCreateTransaction,
  ContractFunctionParameters,
} = await import('@hashgraph/sdk')

// --- Env ---
const operatorId = process.env.HEDERA_OPERATOR_ID
const operatorKey = process.env.HEDERA_OPERATOR_KEY
const network = process.env.HEDERA_NETWORK || 'testnet'
const usdcEvmAddress = process.env.HEDERA_USDC_EVM_ADDRESS

if (!operatorId || !operatorKey) {
  console.error('Missing HEDERA_OPERATOR_ID or HEDERA_OPERATOR_KEY')
  process.exit(1)
}
if (!usdcEvmAddress) {
  console.error('Missing HEDERA_USDC_EVM_ADDRESS (e.g. 0x0000000000000000000000000000000000456858)')
  process.exit(1)
}

// --- Compile ---
const solc = require('solc')
const solPath = join(__dirname, '..', 'src', 'LeagueEscrow.sol')
const source = readFileSync(solPath, 'utf8')

const input = {
  language: 'Solidity',
  sources: { 'LeagueEscrow.sol': { content: source } },
  settings: { outputSelection: { '*': { '*': ['evm.bytecode.object', 'abi'] } } },
}

const output = JSON.parse(solc.compile(JSON.stringify(input)))

if (output.errors) {
  const errors = output.errors.filter(e => e.severity === 'error')
  if (errors.length > 0) {
    console.error('Compilation errors:', errors.map(e => e.formattedMessage).join('\n'))
    process.exit(1)
  }
  output.errors.forEach(e => console.warn(e.formattedMessage))
}

const contract = output.contracts['LeagueEscrow.sol']['LeagueEscrow']
const bytecodeHex = contract.evm.bytecode.object

// Hedera ContractCreateTransaction expects the file to contain the hex string (UTF-8), not raw binary
const fullBytecode = Buffer.from(bytecodeHex, 'utf8')

// --- Deploy ---
const client = network === 'mainnet' ? Client.forMainnet() : Client.forTestnet()
// DER-encoded keys start with '30' in hex; raw hex keys need explicit type
const parsedKey = operatorKey.toLowerCase().startsWith('30')
  ? PrivateKey.fromStringDer(operatorKey)
  : PrivateKey.fromStringECDSA(operatorKey)
client.setOperator(AccountId.fromString(operatorId), parsedKey)

console.log(`Deploying LeagueEscrow to Hedera ${network}...`)
console.log(`USDC EVM address: ${usdcEvmAddress}`)
console.log(`Bytecode size: ${fullBytecode.length} bytes`)

// Upload bytecode in chunks (Hedera max file content per tx = 4096 bytes)
const CHUNK = 4096
const totalChunks = Math.ceil(fullBytecode.length / CHUNK)
console.log(`Uploading bytecode in ${totalChunks} chunk(s)...`)

console.log('Step 1/3: Creating bytecode file...')
const fileCreateTx = await new FileCreateTransaction()
  .setKeys([parsedKey])
  .setContents(fullBytecode.slice(0, CHUNK))
  .setMaxTransactionFee(new Hbar(50))
  .execute(client)

const fileReceipt = await fileCreateTx.getReceipt(client)
const fileId = fileReceipt.fileId
console.log(`Bytecode file: ${fileId}`)

let chunkNum = 1
for (let offset = CHUNK; offset < fullBytecode.length; offset += CHUNK) {
  chunkNum++
  console.log(`Step 1/${totalChunks} append chunk ${chunkNum}/${totalChunks}...`)
  const chunk = fullBytecode.slice(offset, Math.min(offset + CHUNK, fullBytecode.length))
  await (
    await new FileAppendTransaction()
      .setFileId(fileId)
      .setContents(chunk)
      .setMaxTransactionFee(new Hbar(50))
      .execute(client)
  ).getReceipt(client)
}

// The constructor calls associateToken on the HTS precompile (~600K gas).
// Gas limit determines the fee estimate: keep it low enough that
// gasLimit × gasPrice stays under maxTransactionFee.
console.log('Step 2/3: Creating contract...')
const contractTx = await new ContractCreateTransaction()
  .setBytecodeFileId(fileId)
  .setConstructorParameters(new ContractFunctionParameters().addAddress(usdcEvmAddress))
  .setGas(900_000)
  .setMaxTransactionFee(new Hbar(1000))
  .execute(client)

console.log('Step 3/3: Getting receipt...')
const contractReceipt = await contractTx.getReceipt(client)
const contractId = contractReceipt.contractId

console.log(`\n✅ LeagueEscrow deployed!`)
console.log(`Contract ID: ${contractId}`)
console.log(`\nAdd to your .env:`)
console.log(`HEDERA_ESCROW_CONTRACT_ID=${contractId}`)
console.log(`VITE_HEDERA_ESCROW_CONTRACT_ID=${contractId}`)
