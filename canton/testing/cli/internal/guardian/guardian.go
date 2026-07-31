// Package guardian implements the playground's 1/1 Wormhole guardian: key management and
// VAA assembly/signing. It generalizes signNttTransferVAA from
// canton/testing/go/ntt_recipient_match_integration_test.go into a reusable signer that
// covers inbound NTT transfers, arbitrary payloads, and Core governance VAAs.
//
// The playground always runs a single guardian set of size 1 (guardianSetIndex 0, guardian
// index 0, one signature) -- this is a devnet-only trust model; see the CLI README's MainNet
// path section for what a real deployment requires instead.
package guardian

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// Key is a guardian's secp256k1 signing key.
type Key struct {
	priv *ecdsa.PrivateKey
}

// NewRandomKey generates a fresh guardian key -- the playground's default, proving the
// system works with an arbitrary 1/1 guardian set, not just the well-known devnet key.
func NewRandomKey() (*Key, error) {
	priv, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("guardian: generate key: %w", err)
	}
	return &Key{priv: priv}, nil
}

// KeyFromHex loads a guardian key from a hex-encoded secp256k1 private key (with or
// without a 0x prefix) -- e.g. the standard devnet guardian key
// cfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0.
func KeyFromHex(hexKey string) (*Key, error) {
	priv, err := crypto.HexToECDSA(strings.TrimPrefix(hexKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("guardian: parse private key: %w", err)
	}
	return &Key{priv: priv}, nil
}

// HexPrivate returns the private key as a bare hex string (no 0x prefix) -- for persisting
// in playground.state.json (devnet-only convenience; see state.Guardian).
func (k *Key) HexPrivate() string {
	return fmt.Sprintf("%064x", k.priv.D)
}

// Address is the guardian's 20-byte Ethereum-style address (the last 20 bytes of
// keccak256 of the uncompressed public key, sans the 0x04 prefix) -- what a guardian set's
// `keys : [Bytes20]` stores.
func (k *Key) Address() [20]byte {
	var addr [20]byte
	copy(addr[:], crypto.PubkeyToAddress(k.priv.PublicKey).Bytes())
	return addr
}

// PubKeyUncompressed is the 65-byte uncompressed public key (0x04 || X || Y), the form
// ParseAndVerifyVAA/VerifyAndConsumeVAA's pubKeys hints carry.
func (k *Key) PubKeyUncompressed() []byte {
	return crypto.FromECDSAPub(&k.priv.PublicKey)
}

// VAAParams are the fields of a VAA body, everything except the guardian signature set
// (always exactly one signature, guardian index 0, guardian set index 0 in this playground).
type VAAParams struct {
	Timestamp        uint32 // defaults to now if zero
	Nonce            uint32
	EmitterChain     uint16
	EmitterAddress   [32]byte
	Sequence         uint64
	ConsistencyLevel uint8
	Payload          []byte
}

// EncodeBody encodes a VAA body: timestamp(4) || nonce(4) || emitterChain(2) ||
// emitterAddress(32) || sequence(8) || consistencyLevel(1) || payload.
func (p VAAParams) EncodeBody() []byte {
	ts := p.Timestamp
	if ts == 0 {
		ts = uint32(time.Now().Unix()) //nolint:gosec // devnet playground timestamps, not security relevant
	}
	body := make([]byte, 0, 4+4+2+32+8+1+len(p.Payload))
	body = beAppend(body, uint64(ts), 4)
	body = beAppend(body, uint64(p.Nonce), 4)
	body = beAppend(body, uint64(p.EmitterChain), 2)
	body = append(body, p.EmitterAddress[:]...)
	body = beAppend(body, p.Sequence, 8)
	body = append(body, p.ConsistencyLevel)
	body = append(body, p.Payload...)
	return body
}

// Sign assembles and signs a full VAA: version(1)=1, guardianSetIndex(4)=0, sigCount(1)=1,
// guardianIndex(1)=0, signature(65), body. Digest is keccak256(keccak256(body)), matching
// Wormhole.Core.VAA's verification and the reference Go signer.
func Sign(key *Key, p VAAParams) ([]byte, error) {
	body := p.EncodeBody()
	digest := crypto.Keccak256(crypto.Keccak256(body))
	sig, err := crypto.Sign(digest, key.priv)
	if err != nil {
		return nil, fmt.Errorf("guardian: sign VAA: %w", err)
	}

	vaa := make([]byte, 0, 1+4+1+1+65+len(body))
	vaa = append(vaa, 0x01)   // version
	vaa = beAppend(vaa, 0, 4) // guardianSetIndex 0
	vaa = append(vaa, 0x01)   // sigCount 1
	vaa = append(vaa, 0x00)   // guardianIndex 0
	vaa = append(vaa, sig...) // 65-byte signature
	vaa = append(vaa, body...)
	return vaa, nil
}

// TransferParams describes an inbound NTT transfer to sign as if it came from a peer chain
// -- the CLI's `guardian sign-transfer`. sourceManager/sourceTransceiver are the configured
// peer's addresses for sourceChain (the deployment's Peer record); recipientManager is this
// deployment's own managerAddress (the VAA's recipientManager).
type TransferParams struct {
	SourceChain       uint16
	SourceManager     [32]byte // peer.managerAddress
	SourceTransceiver [32]byte // peer.transceiverAddress (the VAA's emitterAddress)
	RecipientManager  [32]byte // this deployment's managerAddress
	SourceToken       [32]byte
	RecipientAddress  [32]byte // wire.RecipientAddressFor(recipientPartyText)
	Decimals          uint8
	Amount            uint64
	Sequence          uint64 // must vary across VAAs so digests never collide
	Nonce             uint32
	ConsistencyLevel  uint8
}

// SignTransfer builds the three nested NTT structures and signs the resulting VAA -- the
// general form of ntt_recipient_match_integration_test.go's signNttTransferVAA.
func SignTransfer(key *Key, p TransferParams) ([]byte, error) {
	ntt := wire.NativeTokenTransfer{
		Decimals:         p.Decimals,
		Amount:           p.Amount,
		SourceToken:      p.SourceToken,
		RecipientAddress: p.RecipientAddress,
		RecipientChain:   wire.CantonChainID,
	}
	var id [32]byte
	binary.BigEndian.PutUint64(id[24:], p.Sequence)
	mm := wire.NttManagerMessage{
		ID:      id,
		Sender:  p.SourceManager,
		Payload: wire.EncodeNativeTokenTransfer(ntt),
	}
	wm := wire.WormholeTransceiverMessage{
		SourceManager:    p.SourceManager,
		RecipientManager: p.RecipientManager,
		ManagerPayload:   wire.EncodeNttManagerMessage(mm),
	}
	return Sign(key, VAAParams{
		Nonce:            p.Nonce,
		EmitterChain:     p.SourceChain,
		EmitterAddress:   p.SourceTransceiver,
		Sequence:         p.Sequence,
		ConsistencyLevel: p.ConsistencyLevel,
		Payload:          wire.EncodeWormholeTransceiverMessage(wm),
	})
}

// ----------------------------------------------------------------------
// Core governance VAAs
// ----------------------------------------------------------------------

// coreGovernanceModule is "Core" left-padded to 32 bytes, matching the module field of
// Test.TestCore:govSetFeeVAA's payload (decoded: 32-byte module || action || chain || ...).
var coreGovernanceModule = func() [32]byte {
	var m [32]byte
	copy(m[32-4:], []byte("Core"))
	return m
}()

// SetMessageFeeAction is the Core governance module's SetMessageFee action code.
const SetMessageFeeAction = 3

// DefaultGovernanceChain is the standard Wormhole governance emitter chain (Solana),
// matching Wormhole.Core.Setup:solanaGovernanceChainId.
const DefaultGovernanceChain = 1

// DefaultGovernanceAddress is the standard governance emitter address, 0x00..04, matching
// Wormhole.Core.Setup:defaultGovernanceContract.
var DefaultGovernanceAddress = [32]byte{31: 0x04}

// GovernanceParams are the VAA envelope fields for a governance VAA -- everything but the
// module-specific payload.
type GovernanceParams struct {
	Timestamp        uint32 // defaults to now if zero
	EmitterChain     uint16 // defaults to DefaultGovernanceChain if zero
	EmitterAddress   [32]byte
	Sequence         uint64
	Nonce            uint32
	ConsistencyLevel uint8
}

func (p GovernanceParams) withDefaults() GovernanceParams {
	if p.EmitterChain == 0 {
		p.EmitterChain = DefaultGovernanceChain
		p.EmitterAddress = DefaultGovernanceAddress
	}
	return p
}

// SignSetMessageFee builds and signs a Core SetMessageFee governance VAA targeting
// cantonChain (72 in production) with the given fee -- payload =
// module(32="Core") || action(1=3) || chain(2) || fee(32, big-endian uint256). Reverse-
// engineered from and pinned against Test.TestCore:govSetFeeVAA (TestCore.daml:31).
func SignSetMessageFee(key *Key, p GovernanceParams, cantonChain uint16, fee uint64) ([]byte, error) {
	p = p.withDefaults()
	payload := make([]byte, 0, 32+1+2+32)
	payload = append(payload, coreGovernanceModule[:]...)
	payload = append(payload, SetMessageFeeAction)
	payload = beAppend(payload, uint64(cantonChain), 2)
	var feeBytes [32]byte
	binary.BigEndian.PutUint64(feeBytes[24:], fee)
	payload = append(payload, feeBytes[:]...)

	return Sign(key, VAAParams{
		Timestamp:        p.Timestamp,
		Nonce:            p.Nonce,
		EmitterChain:     p.EmitterChain,
		EmitterAddress:   p.EmitterAddress,
		Sequence:         p.Sequence,
		ConsistencyLevel: p.ConsistencyLevel,
		Payload:          payload,
	})
}

// ----------------------------------------------------------------------
// NTT governance VAAs
// ----------------------------------------------------------------------

// NttGovernanceModule is "Ntt" left-padded to 32 bytes, the NTT module identifier
// carried by every NTT governance packet -- distinct from coreGovernanceModule
// ("Core"). Matches Wormhole.Ntt.Payload:nttGovernanceModule (Payload.daml).
var NttGovernanceModule = func() [32]byte {
	var m [32]byte
	copy(m[32-3:], []byte("Ntt"))
	return m
}()

// AcceptAdminAction is the NTT governance module's AcceptAdminTransferToGovernance
// action code.
const AcceptAdminAction = 1

// RegisterBurnMintManagerAction is the NTT governance module's RegisterBurnMintManager
// action code -- the guardian quorum co-signing a gg-minted burn/mint deployment's
// registration alongside the registering admin.
const RegisterBurnMintManagerAction = 2

// SignAcceptAdmin builds and signs an NTT AcceptAdminTransferToGovernance
// governance VAA targeting cantonChain (72 in production), authorizing the
// guardian quorum's acceptance of the admin role for the manager identified by
// managerAddress at factoryEpoch -- payload = module(32="Ntt") || action(1=1) ||
// chain(2) || managerAddress(32) || factoryEpoch(8, big-endian uint64). Mirrors
// Wormhole.Ntt.Payload:encodeNttGovernance (Payload.daml) and
// SignSetMessageFee's shape for the Core module.
func SignAcceptAdmin(key *Key, p GovernanceParams, cantonChain uint16, managerAddress [32]byte, factoryEpoch uint64) ([]byte, error) {
	p = p.withDefaults()
	payload := make([]byte, 0, 32+1+2+32+8)
	payload = append(payload, NttGovernanceModule[:]...)
	payload = append(payload, AcceptAdminAction)
	payload = beAppend(payload, uint64(cantonChain), 2)
	payload = append(payload, managerAddress[:]...)
	payload = beAppend(payload, factoryEpoch, 8)

	return Sign(key, VAAParams{
		Timestamp:        p.Timestamp,
		Nonce:            p.Nonce,
		EmitterChain:     p.EmitterChain,
		EmitterAddress:   p.EmitterAddress,
		Sequence:         p.Sequence,
		ConsistencyLevel: p.ConsistencyLevel,
		Payload:          payload,
	})
}

// SignRegisterBurnMintManager builds and signs an NTT RegisterBurnMintManager governance VAA,
// co-signing a gg-minted burn/mint deployment's registration alongside the registering admin
// (Wormhole.Ntt.Governance.RegisterManagerByVaa) -- payload = module(32="Ntt") ||
// action(1=2) || chain(2) || registrationBinding(32) || tokenDecimals(1). registrationBinding
// is the caller's wire.DerivedAddress(wire.RegistrationBindingTag, operatorText, adminText,
// nonce) -- computed off this function since it needs the genuinely-allocated operator/admin
// party ids, not just the deployment's config. Mirrors SignAcceptAdmin's shape.
func SignRegisterBurnMintManager(key *Key, p GovernanceParams, cantonChain uint16, registrationBinding [32]byte, tokenDecimals uint8) ([]byte, error) {
	p = p.withDefaults()
	payload := make([]byte, 0, 32+1+2+32+1)
	payload = append(payload, NttGovernanceModule[:]...)
	payload = append(payload, RegisterBurnMintManagerAction)
	payload = beAppend(payload, uint64(cantonChain), 2)
	payload = append(payload, registrationBinding[:]...)
	payload = append(payload, tokenDecimals)

	return Sign(key, VAAParams{
		Timestamp:        p.Timestamp,
		Nonce:            p.Nonce,
		EmitterChain:     p.EmitterChain,
		EmitterAddress:   p.EmitterAddress,
		Sequence:         p.Sequence,
		ConsistencyLevel: p.ConsistencyLevel,
		Payload:          payload,
	})
}

// beAppend appends v's low n bytes, big-endian, to b.
func beAppend(b []byte, v uint64, n int) []byte {
	tmp := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp, v)
	return append(b, tmp[8-n:]...)
}
