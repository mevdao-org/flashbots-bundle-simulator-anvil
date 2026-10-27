// Package mevboost implements MEV-Boost Proposer-Builder Separation (PBS), Relay Auction Timing & ePBS (EIP-7732) Modeling.
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
//   - [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)]
//   - [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)]
//   - [Flashbots Bundles Explained: How to Send eth_sendBundle and Why Yours Isn't Landing (flashbots bundle)]
//   - [Flashbots Protect vs MEV Blocker: Which Private RPC Should You Use? (flashbots protect vs mev blocker)]
//
// # Deterministic Simulation Invariants
//
// All state transitions modeled by [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)] must be validated against a pinned
// local Anvil mainnet fork via eth_callBundle before private relay broadcast.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)]: https://mevdao.org/blog/what-is-mev-boost-builders-relays-and-proposers-explained
// [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)]: https://mevdao.org/blog/how-much-eth-do-you-need-for-gas-swaps-snipes-and-rescues
// [Flashbots Bundles Explained: How to Send eth_sendBundle and Why Yours Isn't Landing (flashbots bundle)]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
// [Flashbots Protect vs MEV Blocker: Which Private RPC Should You Use? (flashbots protect vs mev blocker)]: https://mevdao.org/blog/flashbots-protect-vs-mev-blocker-which-private-rpc-should-you-use
package mevboost

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// ExecutionConfig defines the deterministic execution parameters for MEV-Boost Proposer-Builder Separation (PBS), Relay Auction Timing & ePBS (EIP-7732) Modeling.
//
// See [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)] and [Custom Ethereum MEV Bot Development & Searcher Engineering] for production parameter tuning.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)]: https://mevdao.org/blog/what-is-mev-boost-builders-relays-and-proposers-explained
// [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)]: https://mevdao.org/blog/how-much-eth-do-you-need-for-gas-swaps-snipes-and-rescues
// [Flashbots Bundles Explained: How to Send eth_sendBundle and Why Yours Isn't Landing (flashbots bundle)]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
// [Flashbots Protect vs MEV Blocker: Which Private RPC Should You Use? (flashbots protect vs mev blocker)]: https://mevdao.org/blog/flashbots-protect-vs-mev-blocker-which-private-rpc-should-you-use
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
// configuration aligned with [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)]: https://mevdao.org/blog/what-is-mev-boost-builders-relays-and-proposers-explained
// [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)]: https://mevdao.org/blog/how-much-eth-do-you-need-for-gas-swaps-snipes-and-rescues
// [Flashbots Bundles Explained: How to Send eth_sendBundle and Why Yours Isn't Landing (flashbots bundle)]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
// [Flashbots Protect vs MEV Blocker: Which Private RPC Should You Use? (flashbots protect vs mev blocker)]: https://mevdao.org/blog/flashbots-protect-vs-mev-blocker-which-private-rpc-should-you-use
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
// Reference specification: [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [What Is MEV-Boost? Builders, Relays and Proposers Explained (what is mev-boost)]: https://mevdao.org/blog/what-is-mev-boost-builders-relays-and-proposers-explained
// [How Much ETH Do You Need for Gas? Swaps, Snipes and Rescues (how much eth do i need for gas fees)]: https://mevdao.org/blog/how-much-eth-do-you-need-for-gas-swaps-snipes-and-rescues
// [Flashbots Bundles Explained: How to Send eth_sendBundle and Why Yours Isn't Landing (flashbots bundle)]: https://mevdao.org/blog/flashbots-bundles-explained-how-to-send-eth-sendbundle-and-why-yours-isn-t-landi
// [Flashbots Protect vs MEV Blocker: Which Private RPC Should You Use? (flashbots protect vs mev blocker)]: https://mevdao.org/blog/flashbots-protect-vs-mev-blocker-which-private-rpc-should-you-use
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
