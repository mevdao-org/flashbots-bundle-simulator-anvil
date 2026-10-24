import "dotenv/config";
import {
  createPublicClient,
  createTestClient,
  formatEther,
  http,
  parseEther,
  type Hex,
} from "viem";
import { mainnet } from "viem/chains";

/**
 * MEV DAO Local Anvil Mainnet Fork Bundle Simulator
 * Replays an array of raw signed transactions with `anvil_setAutoMine(false)`,
 * mines a single deterministic block with `evm_mine`, and calculates the exact
 * `block.coinbase` balance delta and per-transaction gas consumption.
 *
 * Full Guide:
 * https://mevdao.org/blog/how-to-test-an-mev-bot-on-a-mainnet-fork-with-anvil
 */

export async function simulateBundleOnAnvilFork(
  anvilRpcUrl: string,
  signedTransactions: Hex[]
) {
  const publicClient = createPublicClient({
    chain: mainnet,
    transport: http(anvilRpcUrl),
  });
  const testClient = createTestClient({
    chain: mainnet,
    mode: "anvil",
    transport: http(anvilRpcUrl),
  });

  const snapshotId = await testClient.snapshot();

  try {
    // 1. Disable automine so all bundle transactions land in the exact same block in order
    await testClient.setAutomine(false);

    const headBlock = await publicClient.getBlock();
    const coinbaseAddress = headBlock.miner;
    const coinbaseBalanceBefore = await publicClient.getBalance({ address: coinbaseAddress });

    // 2. Queue each raw transaction into Anvil's pending block pool
    const txHashes: Hex[] = [];
    for (const rawTx of signedTransactions) {
      const hash = await publicClient.sendRawTransaction({ serializedTransaction: rawTx });
      txHashes.push(hash);
    }

    // 3. Mine exactly 1 block containing the ordered bundle
    await testClient.mine({ blocks: 1 });

    // 4. Inspect receipts & compute builder profit (coinbaseDiff)
    let totalGasUsed = 0n;
    const receipts = [];
    for (const hash of txHashes) {
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      totalGasUsed += receipt.gasUsed;
      receipts.push({
        txHash: hash,
        status: receipt.status,
        gasUsed: receipt.gasUsed.toString(),
        effectiveGasPriceGwei: Number(receipt.effectiveGasPrice) / 1e9,
      });
    }

    const coinbaseBalanceAfter = await publicClient.getBalance({ address: coinbaseAddress });
    const coinbaseDiffWei = coinbaseBalanceAfter - coinbaseBalanceBefore;
    const effectiveBundleGasPriceWei =
      totalGasUsed > 0n ? coinbaseDiffWei / totalGasUsed : 0n;

    return {
      simulatedBlockNumber: (headBlock.number ?? 0n) + 1n,
      totalGasUsed: totalGasUsed.toString(),
      coinbaseDiffETH: formatEther(coinbaseDiffWei),
      effectiveBundleGasPriceGwei: Number(effectiveBundleGasPriceWei) / 1e9,
      receipts,
    };
  } finally {
    await testClient.setAutomine(true).catch(() => {});
    await testClient.revert({ id: snapshotId });
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log("MEV DAO Anvil Mainnet Fork Bundle Simulator Ready.");
  console.log("Start Anvil with: anvil --fork-url $ETH_HTTP_RPC_URL --fork-block-number 21000000");
  console.log("Guide: https://mevdao.org/blog/how-to-test-an-mev-bot-on-a-mainnet-fork-with-anvil");
}
