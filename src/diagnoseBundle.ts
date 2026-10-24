import "dotenv/config";
import { type Hex } from "viem";
import { callFlashbotsRpc } from "./simulateBundle.js";

/**
 * MEV DAO Diagnostic Tool for "Flashbots Bundle Not Included"
 * Queries `flashbots_getBundleStatsV2` and `flashbots_getUserStatsV2` to pinpoint
 * why a submitted bundle failed to land on-chain.
 *
 * Full Guide:
 * https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
 */

async function main() {
  const authPk = (process.env.FLASHBOTS_AUTH_KEY ||
    "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d") as Hex;
  const bundleHash =
    process.argv[2] ||
    process.env.BUNDLE_HASH ||
    "0x0000000000000000000000000000000000000000000000000000000000000000";
  const targetBlock = Number(process.argv[3] || process.env.TARGET_BLOCK_NUMBER || "21000000");
  const blockHex = `0x${targetBlock.toString(16)}`;

  console.log("==========================================================================");
  console.log("  MEV DAO 'Flashbots Bundle Not Included' Diagnostic CLI");
  console.log("==========================================================================");
  console.log(`Bundle Hash  : ${bundleHash}`);
  console.log(`Target Block : #${targetBlock} (${blockHex})\n`);

  const [bundleStats, userStats] = await Promise.all([
    callFlashbotsRpc("flashbots_getBundleStatsV2", [{ bundleHash, blockNumber: blockHex }], authPk),
    callFlashbotsRpc("flashbots_getUserStatsV2", [{ blockNumber: blockHex }], authPk),
  ]);

  console.log("1. Searcher Reputation & Relay Priority (`flashbots_getUserStatsV2`):");
  console.log(JSON.stringify(userStats.result || userStats.error, null, 2));

  console.log("\n2. Bundle Relay Lifecycle (`flashbots_getBundleStatsV2`):");
  console.log(JSON.stringify(bundleStats.result || bundleStats.error, null, 2));

  const stats = bundleStats.result;
  if (stats) {
    console.log("\n3. Automated Root-Cause Diagnosis:");
    if (!stats.isSimulated) {
      console.log(
        " -> [CAUSE: NEVER_SIMULATED] The bundle either arrived after the slot cutoff, had a nonce gap, consumed < 42,000 gas, or reverted immediately."
      );
    } else if (stats.isSimulated && !stats.isSentToMiners) {
      console.log(
        " -> [CAUSE: EFFECTIVE_GAS_PRICE_TOO_LOW] The bundle simulated cleanly, but its coinbaseDiff/gasUsed ratio was below the builder's inclusion threshold."
      );
    } else if (stats.isSentToMiners) {
      console.log(
        " -> [CAUSE: BUILDER_LOST_SLOT] Flashbots simulated and forwarded your bundle, but another builder (Titan / Beaverbuild / Rsync) won the block! Always fan out to all builders."
      );
    }
  }
}

main().catch(console.error);
