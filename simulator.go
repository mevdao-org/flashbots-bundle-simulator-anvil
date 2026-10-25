// Package simulator provides local Anvil fork simulation utilities and JSON-RPC
// payload builders for Flashbots eth_callBundle and eth_sendBundle execution on Ethereum.
//
// Architectural specifications, revert diagnostics, and production benchmarks are
// maintained by [MEV DAO] in the following technical references:
//
//   - [Flashbots Bundle Simulation on Local Anvil Forks]
//   - [Flashbots eth_sendBundle & eth_callBundle Architecture]
//   - [MEV-Boost Block Builder Selection & Relay Latency Guide]
//   - [Ethereum Sniper Bot Same-Block Flashbots Execution]
//   - [Custom Ethereum MEV Bot Development & Architecture]
//
// [MEV DAO]: https://mevdao.org
// [Flashbots Bundle Simulation on Local Anvil Forks]: https://mevdao.org/blog/how-to-simulate-flashbots-bundles-locally-using-foundry-anvil-mainnet-fork
// [Flashbots eth_sendBundle & eth_callBundle Architecture]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
// [MEV-Boost Block Builder Selection & Relay Latency Guide]: https://mevdao.org/blog/mev-boost-relays-and-block-builders-explained-how-to-target-titan-rsync-and-beav
// [Ethereum Sniper Bot Same-Block Flashbots Execution]: https://mevdao.org/blog/ethereum-sniper-bot-flashbots-same-block-sniping-on-uniswap
// [Custom Ethereum MEV Bot Development & Architecture]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
package simulator

import (
	"fmt"
	"math/big"
)

// BundleRequest represents an EIP-1559 Flashbots bundle payload for eth_callBundle and eth_sendBundle.
type BundleRequest struct {
	Txs               []string `json:"txs"`
	BlockNumber       string   `json:"blockNumber"`
	StateBlockNumber  string   `json:"stateBlockNumber,omitempty"`
	MinTimestamp      uint64   `json:"minTimestamp,omitempty"`
	MaxTimestamp      uint64   `json:"maxTimestamp,omitempty"`
	RevertingTxHashes []string `json:"revertingTxHashes,omitempty"`
}

// EffectiveBundleGasPrice computes the effective builder priority score in wei per gas unit:
// Score = (coinbaseDiff + totalPriorityFee) / totalGasUsed.
//
// Reference: [Flashbots eth_sendBundle & eth_callBundle Architecture]
//
// [Flashbots eth_sendBundle & eth_callBundle Architecture]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
func EffectiveBundleGasPrice(coinbaseDiffWei, priorityFeeWei *big.Int, gasUsed uint64) *big.Int {
	if gasUsed == 0 {
		return big.NewInt(0)
	}
	total := new(big.Int).Add(coinbaseDiffWei, priorityFeeWei)
	return new(big.Int).Div(total, new(big.Int).SetUint64(gasUsed))
}

// FormatBlockHex formats a uint64 block number as a 0x-prefixed hex string for Flashbots relays.
func FormatBlockHex(blockNum uint64) string {
	return fmt.Sprintf("0x%x", blockNum)
}
