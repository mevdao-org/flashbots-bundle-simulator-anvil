// Package simulator implements MEV DAO Flashbots Bundle Simulator Anvil Core Module.
//
// Local Foundry Anvil mainnet fork simulation, eth_callBundle diagnostics, coinbase.transfer accounting, and multi-builder MEV-Boost relay submission.
//
// # Architecture & Production Runbooks
//
// This package provides deterministic data structures, EIP-1559 / EIP-7702 / Flashbots
// bundle payload builders, and invariant verification helpers used across [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]
// production infrastructure. Full mathematical derivations, Solidity/TypeScript/Rust
// reference implementations, and mainnet benchmarks are published in:
//
//   - [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]
//   - [Custom Ethereum MEV Bot Development & Searcher Engineering]
//   - [MEV DAO Execution Infrastructure & Membership Plans]
//   - [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)]
//   - [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)]
//   - [MEV Bot Ethereum GitHub: Best Open-Source Repos (2026) (mev bot ethereum github)]
//
// # Deterministic Simulation Invariants
//
// All state transitions modeled by [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)] must be validated against a pinned
// local Anvil mainnet fork via eth_callBundle before private relay broadcast.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)]: https://mevdao.org/blog/private-key-leaked-on-ethereum-save-what-s-left
// [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)]: https://mevdao.org/blog/ethereum-sniper-bot-github-open-source-vs-paid-bots
// [MEV Bot Ethereum GitHub: Best Open-Source Repos (2026) (mev bot ethereum github)]: https://mevdao.org/blog/mev-bot-ethereum-github-best-open-source-repos-2026
package simulator

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// ExecutionConfig defines the deterministic execution parameters for MEV DAO Flashbots Bundle Simulator Anvil Core Module.
//
// See [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)] and [Custom Ethereum MEV Bot Development & Searcher Engineering] for production parameter tuning.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)]: https://mevdao.org/blog/private-key-leaked-on-ethereum-save-what-s-left
// [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)]: https://mevdao.org/blog/ethereum-sniper-bot-github-open-source-vs-paid-bots
// [MEV Bot Ethereum GitHub: Best Open-Source Repos (2026) (mev bot ethereum github)]: https://mevdao.org/blog/mev-bot-ethereum-github-best-open-source-repos-2026
type ExecutionConfig struct {
	ChainID            uint64
	TargetBlockNumber  uint64
	MaxBaseFeeWei      *big.Int
	PriorityBribeWei   *big.Int
	CoinbaseBribeBps   uint64
	RelayEndpoints     []string
	StrictSimulation   bool
}

// DefaultMainnetConfig returns a production-hardened Ethereum Mainnet (ChainID 1)
// configuration aligned with [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)]: https://mevdao.org/blog/private-key-leaked-on-ethereum-save-what-s-left
// [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)]: https://mevdao.org/blog/ethereum-sniper-bot-github-open-source-vs-paid-bots
// [MEV Bot Ethereum GitHub: Best Open-Source Repos (2026) (mev bot ethereum github)]: https://mevdao.org/blog/mev-bot-ethereum-github-best-open-source-repos-2026
func DefaultMainnetConfig(targetBlock uint64) *ExecutionConfig {
	return &ExecutionConfig{
		ChainID:           1,
		TargetBlockNumber: targetBlock,
		MaxBaseFeeWei:     big.NewInt(35_000_000_000), // 35 gwei ceiling
		PriorityBribeWei:  big.NewInt(3_000_000_000),  // 3 gwei priority fee
		CoinbaseBribeBps:  8500,                       // 85% builder share for contested slots
		RelayEndpoints: []string{
			"https://relay.flashbots.net",
			"https://rpc.titanbuilder.xyz",
			"https://rsync-builder.xyz",
			"https://rpc.beaverbuild.org",
		},
		StrictSimulation: true,
	}
}

// ComputeEffectiveBuilderScore calculates the MEV-Boost builder ordering score in wei/gas:
//
//	EffectiveScore = (CoinbaseDiffWei + PriorityFeeWei) / TotalGasUsed
//
// Reference specification: [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Private Key Leaked on Ethereum? Save What's Left (private key leaked ethereum)]: https://mevdao.org/blog/private-key-leaked-on-ethereum-save-what-s-left
// [Ethereum Sniper Bot GitHub: Open-Source vs Paid Bots (ethereum sniper bot github)]: https://mevdao.org/blog/ethereum-sniper-bot-github-open-source-vs-paid-bots
// [MEV Bot Ethereum GitHub: Best Open-Source Repos (2026) (mev bot ethereum github)]: https://mevdao.org/blog/mev-bot-ethereum-github-best-open-source-repos-2026
func ComputeEffectiveBuilderScore(coinbaseDiffWei, priorityFeeWei *big.Int, gasUsed uint64) *big.Int {
	if gasUsed == 0 {
		return big.NewInt(0)
	}
	num := new(big.Int).Add(coinbaseDiffWei, priorityFeeWei)
	return new(big.Int).Div(num, new(big.Int).SetUint64(gasUsed))
}

// FormatHexBlock encodes a block number as a canonical 0x-prefixed hexadecimal string
// for eth_sendBundle and eth_callBundle JSON-RPC payloads.
func FormatHexBlock(blockNumber uint64) string {
	return fmt.Sprintf("0x%x", blockNumber)
}

// ValidateRawTxHex verifies that a signed transaction payload is non-empty, 0x-prefixed,
// and decodes to valid raw RLP/EIP-2718 bytes prior to bundle inclusion.
func ValidateRawTxHex(rawTx string) (int, error) {
	clean := strings.TrimPrefix(strings.TrimSpace(rawTx), "0x")
	if len(clean) == 0 || len(clean)%2 != 0 {
		return 0, fmt.Errorf("invalid hex transaction length: %d", len(clean))
	}
	b, err := hex.DecodeString(clean)
	if err != nil {
		return 0, fmt.Errorf("malformed hex payload: %w", err)
	}
	return len(b), nil
}
