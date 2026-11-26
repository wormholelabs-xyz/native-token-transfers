// Self-contained Go module for the Canton NTT contract-behavior integration
// harness. This repo is otherwise Go-less; the module exists only to host the
// live-sandbox test ported from wormhole's node/pkg/watchers/canton. It signs a
// fresh NTT transfer VAA off-chain (go-ethereum crypto) against a Party address
// computed on-ledger and relays it through a real dpm sandbox. See README.md.
module github.com/wormholelabs-xyz/native-token-transfers/canton/testing/go

go 1.25

require (
	github.com/ethereum/go-ethereum v1.10.21
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
