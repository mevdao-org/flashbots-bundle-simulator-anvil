import "dotenv/config";
import {
  createPublicClient,
  createWalletClient,
  formatUnits,
  http,
  keccak256,
  parseGwei,
  toBytes,
  type Hex,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { mainnet } from "viem/chains";

/**
 * MEV DAO Flashbots Bundle Simulator (`eth_callBundle` & Effective Gas Price Analyzer)
 * Full Technical Guide:
 * https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
 */

export interface BundleSimulationDiagnosis {
  bundleHash: string;
  totalGasUsed: number;
  coinbaseDiffWei: bigint;
  ethSentToCoinbaseWei: bigint;
  gasFeesWei: bigint;
  effectiveBundleGasPriceGwei: string;
  allTxsSucceeded: boolean;
  failedTransactions: Array<{ txHash: string; error?: string; revert?: string }>;
  inclusionVerdict: string;
}

export async function callFlashbotsRpc(
  method: string,
  params: unknown[],
  authKey: Hex,
  relayUrl = "https://relay.flashbots.net"
) {
  const signer = privateKeyToAccount(authKey);
  const body = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method,
    params,
  });
  const signature = await signer.signMessage({
    message: keccak256(toBytes(body)),
  });

  const res = await fetch(relayUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Flashbots-Signature": `${signer.address}:${signature}`,
    },
    body,
  });
  return (await res.json()) as { result?: any; error?: { code: number; message: string } };
}

export function analyzeCallBundleResult(simResult: any, currentPriorityFloorGwei = 3): BundleSimulationDiagnosis {
  const coinbaseDiffWei = BigInt(simResult.coinbaseDiff || "0");
  const ethSentToCoinbaseWei = BigInt(simResult.ethSentToCoinbase || "0");
  const gasFeesWei = BigInt(simResult.gasFees || "0");
  const totalGasUsed = Number(simResult.totalGasUsed || 0);

  const effectivePriceWei =
    totalGasUsed > 0 ? coinbaseDiffWei / BigInt(totalGasUsed) : 0n;
  const effectiveBundleGasPriceGwei = formatUnits(effectivePriceWei, 9);

  const failedTransactions = (simResult.results || []).filter(
    (r: any) => r.error || r.revert
  );
  const allTxsSucceeded = failedTransactions.length === 0;

  let inclusionVerdict = "READY_FOR_MULTI_BUILDER_BROADCAST";
  if (!allTxsSucceeded) {
    inclusionVerdict =
      "REJECTED_BY_BUILDER: At least one transaction inside the bundle reverted. Builders drop reverting bundles unless revertingTxHashes is explicitly set.";
  } else if (totalGasUsed < 42_000) {
    inclusionVerdict =
      "WARNING_LOW_GAS_FLOOR: Flashbots relay requires bundles to consume >= 42,000 total gas unless submitted via Flashbots Protect.";
  } else if (Number(effectiveBundleGasPriceGwei) < currentPriorityFloorGwei) {
    inclusionVerdict = `LOW_EFFECTIVE_GAS_PRICE (${effectiveBundleGasPriceGwei} gwei): Likely to be outbid in competitive blocks. Increase coinbase bribe or priority fee.`;
  }

  return {
    bundleHash: simResult.bundleHash,
    totalGasUsed,
    coinbaseDiffWei,
    ethSentToCoinbaseWei,
    gasFeesWei,
    effectiveBundleGasPriceGwei,
    allTxsSucceeded,
    failedTransactions,
    inclusionVerdict,
  };
}

async function main() {
  const rpcUrl = process.env.ETH_HTTP_RPC_URL || "https://eth.llamarpc.com";
  const searcherPk = (process.env.SEARCHER_PRIVATE_KEY ||
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80") as Hex;
  const authPk = (process.env.FLASHBOTS_AUTH_KEY ||
    "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d") as Hex;

  const searcher = privateKeyToAccount(searcherPk);
  const publicClient = createPublicClient({ chain: mainnet, transport: http(rpcUrl) });
  const walletClient = createWalletClient({ account: searcher, chain: mainnet, transport: http(rpcUrl) });

  const block = await publicClient.getBlock();
  const nonce = await publicClient.getTransactionCount({ address: searcher.address });
  const maxFeePerGas = (block.baseFeePerGas ?? parseGwei("15")) * 2n + parseGwei("5");

  // Build two sample signed transactions for bundle simulation (>= 42,000 total gas)
  const tx1 = await walletClient.signTransaction({
    account: searcher,
    to: searcher.address,
    value: 0n,
    gas: 21_000n,
    maxFeePerGas,
    maxPriorityFeePerGas: parseGwei("5"),
    nonce,
  });

  const tx2 = await walletClient.signTransaction({
    account: searcher,
    to: searcher.address,
    value: 0n,
    gas: 21_000n,
    maxFeePerGas,
    maxPriorityFeePerGas: parseGwei("5"),
    nonce: nonce + 1,
  });

  console.log("==========================================================================");
  console.log("  MEV DAO Flashbots Bundle Simulator & Analyzer (https://mevdao.org)");
  console.log("==========================================================================");
  console.log(`Current Head Block : #${block.number}`);
  console.log(`Target Block       : #${(block.number ?? 0n) + 1n}`);
  console.log(`Searcher Signer    : ${searcher.address}`);
  console.log(`Signed Bundle Txs  : 2 transactions (nonces ${nonce}, ${nonce + 1})`);
  console.log(
    "\nGuide: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi"
  );
  void [tx1, tx2];
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch(console.error);
}
