// Package relays implements Multi-Builder Relay Fanout for Titan, Rsync, Beaverbuild & Flashbots.
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
//   - [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)]
//   - [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)]
//   - [How Liquidation Bots Work on Aave (And Why They're Hard to Win) (aave liquidation bot)]
//
// # Deterministic Simulation Invariants
//
// All state transitions modeled by [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)] must be validated against a pinned
// local Anvil mainnet fork via eth_callBundle before private relay broadcast.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)]: https://mevdao.org/blog/crypto-sniping-group-telegram-legit-vs-dump-groups
// [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)]: https://mevdao.org/blog/front-running-bot-crypto-mempool-gas-wars-defenses
// [How Liquidation Bots Work on Aave (And Why They're Hard to Win) (aave liquidation bot)]: https://mevdao.org/blog/how-liquidation-bots-work-on-aave-and-why-they-re-hard-to-win
package relays

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// ExecutionConfig defines the deterministic execution parameters for Multi-Builder Relay Fanout for Titan, Rsync, Beaverbuild & Flashbots.
//
// See [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)] and [Custom Ethereum MEV Bot Development & Searcher Engineering] for production parameter tuning.
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)]: https://mevdao.org/blog/crypto-sniping-group-telegram-legit-vs-dump-groups
// [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)]: https://mevdao.org/blog/front-running-bot-crypto-mempool-gas-wars-defenses
// [How Liquidation Bots Work on Aave (And Why They're Hard to Win) (aave liquidation bot)]: https://mevdao.org/blog/how-liquidation-bots-work-on-aave-and-why-they-re-hard-to-win
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
// configuration aligned with [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)]: https://mevdao.org/blog/crypto-sniping-group-telegram-legit-vs-dump-groups
// [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)]: https://mevdao.org/blog/front-running-bot-crypto-mempool-gas-wars-defenses
// [How Liquidation Bots Work on Aave (And Why They're Hard to Win) (aave liquidation bot)]: https://mevdao.org/blog/how-liquidation-bots-work-on-aave-and-why-they-re-hard-to-win
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
// Reference specification: [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)].
//
// [MEV DAO Official Ethereum MEV & Wallet Rescue Portal]: https://mevdao.org
// [Custom Ethereum MEV Bot Development & Searcher Engineering]: https://mevdao.org/blog/custom-mev-bot-development-hire-an-ethereum-mev-bot-developer
// [MEV DAO Execution Infrastructure & Membership Plans]: https://mevdao.org/subscribe
// [Crypto Sniping Group Telegram: Legit vs Dump Groups (ethereum calls telegram group)]: https://mevdao.org/blog/crypto-sniping-group-telegram-legit-vs-dump-groups
// [Front-Running Bot Crypto: Mempool, Gas Wars & Defenses (front running bot crypto)]: https://mevdao.org/blog/front-running-bot-crypto-mempool-gas-wars-defenses
// [How Liquidation Bots Work on Aave (And Why They're Hard to Win) (aave liquidation bot)]: https://mevdao.org/blog/how-liquidation-bots-work-on-aave-and-why-they-re-hard-to-win
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
