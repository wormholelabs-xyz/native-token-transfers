package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/amulet"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/disclosure"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/observer"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/state"
	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// ----------------------------------------------------------------------
// decodeHex32 / trimHexPrefix
// ----------------------------------------------------------------------

func TestTrimHexPrefix(t *testing.T) {
	cases := map[string]string{
		"0xabcd": "abcd",
		"0Xabcd": "abcd",
		"abcd":   "abcd",
		"0x":     "",
		"a":      "a",
		"":       "",
	}
	for in, want := range cases {
		if got := trimHexPrefix(in); got != want {
			t.Fatalf("trimHexPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeHex32(t *testing.T) {
	t.Run("0x-prefixed value is right-padded", func(t *testing.T) {
		out, err := decodeHex32("0x" + strings.Repeat("ab", 4))
		if err != nil {
			t.Fatalf("decodeHex32: %v", err)
		}
		// A short value is right-aligned (copy(out[32-len(raw):], raw)) -- the leading
		// bytes must stay zero.
		for i := 0; i < 28; i++ {
			if out[i] != 0 {
				t.Fatalf("expected zero-padding, got %x", out)
			}
		}
		if out[31] != 0xab {
			t.Fatalf("expected the last byte to be 0xab, got %x", out)
		}
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := decodeHex32("not-hex")
		if err == nil {
			t.Fatalf("expected an error for invalid hex")
		}
	})

	t.Run("too long", func(t *testing.T) {
		_, err := decodeHex32(strings.Repeat("ab", 33))
		if err == nil || !strings.Contains(err.Error(), "longer than 32 bytes") {
			t.Fatalf("expected a too-long error, got %v", err)
		}
	})

	t.Run("exactly 32 bytes", func(t *testing.T) {
		full := strings.Repeat("ff", 32)
		out, err := decodeHex32(full)
		if err != nil {
			t.Fatalf("decodeHex32: %v", err)
		}
		for _, b := range out {
			if b != 0xff {
				t.Fatalf("expected every byte to be 0xff, got %x", out)
			}
		}
	})
}

// ----------------------------------------------------------------------
// formatDecimal
// ----------------------------------------------------------------------

func TestFormatDecimal(t *testing.T) {
	cases := []struct {
		raw      int64
		decimals int
		want     string
	}{
		{12345, 0, "12345"},
		{12345, -1, "12345"},
		{12345, 2, "123.45"},
		{5, 4, "0.0005"},
		{0, 6, "0.000000"},
		{100000000, 8, "1.00000000"},
	}
	for _, c := range cases {
		if got := formatDecimal(c.raw, c.decimals); got != c.want {
			t.Fatalf("formatDecimal(%d, %d) = %q, want %q", c.raw, c.decimals, got, c.want)
		}
	}
}

// ----------------------------------------------------------------------
// decimalLiteral
// ----------------------------------------------------------------------

func TestDecimalLiteral_MarshalUnmarshalString(t *testing.T) {
	var d decimalLiteral
	if err := d.UnmarshalJSON([]byte("1.50000000")); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if d.String() != "1.50000000" {
		t.Fatalf("String: got %q", d.String())
	}
	raw, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(raw) != "1.50000000" {
		t.Fatalf("MarshalJSON: got %q", raw)
	}

	// Round-trip through encoding/json to prove the bare-literal (non-quoted) contract holds.
	type wrapper struct {
		V decimalLiteral `json:"v"`
	}
	out, err := json.Marshal(wrapper{V: "2.5"})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(out) != `{"v":2.5}` {
		t.Fatalf("expected a bare numeric literal, got %s", out)
	}
}

// ----------------------------------------------------------------------
// toAmuletSeamJSON
// ----------------------------------------------------------------------

func TestToAmuletSeamJSON_Empty(t *testing.T) {
	out, err := toAmuletSeamJSON(amulet.TransferFactory{FactoryID: "00f", ChoiceContextData: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("toAmuletSeamJSON: %v", err)
	}
	if out.FactoryCid != "00f" || len(out.Disclosed) != 0 {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestToAmuletSeamJSON_DecodesBase64ToHex(t *testing.T) {
	// base64("hello") == "aGVsbG8="
	f := amulet.TransferFactory{
		FactoryID: "00f",
		DisclosedContracts: []amulet.DisclosedContract{
			{TemplateID: "T1", ContractID: "00c", CreatedEventBlob: "aGVsbG8="},
		},
	}
	out, err := toAmuletSeamJSON(f)
	if err != nil {
		t.Fatalf("toAmuletSeamJSON: %v", err)
	}
	if len(out.Disclosed) != 1 {
		t.Fatalf("expected 1 disclosed contract, got %+v", out.Disclosed)
	}
	want := "68656c6c6f" // hex("hello")
	if out.Disclosed[0].Blob != want {
		t.Fatalf("blob mismatch: got %q want %q", out.Disclosed[0].Blob, want)
	}
	if out.Disclosed[0].TemplateID != "T1" || out.Disclosed[0].ContractID != "00c" {
		t.Fatalf("field passthrough mismatch: %+v", out.Disclosed[0])
	}
}

func TestToAmuletSeamJSON_MalformedBase64(t *testing.T) {
	f := amulet.TransferFactory{
		DisclosedContracts: []amulet.DisclosedContract{
			{TemplateID: "T1", ContractID: "00c", CreatedEventBlob: "not-valid-base64!!"},
		},
	}
	_, err := toAmuletSeamJSON(f)
	if err == nil || !strings.Contains(err.Error(), "decode createdEventBlob") {
		t.Fatalf("expected a decode error, got %v", err)
	}
}

// ----------------------------------------------------------------------
// knownHints
// ----------------------------------------------------------------------

func TestKnownHints(t *testing.T) {
	s := state.New()
	s.Operator = "op::abc"
	s.GuardianGovernance = "gov::abc"
	s.GuardianObserver = "obs::abc"
	s.Deployments["ntt1"] = state.Deployment{Admin: "admin1::abc"}
	s.Users["Alice"] = "alice::abc"
	// A Users entry that collides with a well-known id must win (per knownHints' doc
	// comment: "Users entries win on conflict").
	s.Users["CustomOperatorHint"] = "op::abc"

	hints := knownHints(s)
	if hints["op::abc"] != "CustomOperatorHint" {
		t.Fatalf("expected the Users entry to win over the built-in Operator hint, got %q", hints["op::abc"])
	}
	if hints["gov::abc"] != "GuardianGovernance" {
		t.Fatalf("GuardianGovernance hint mismatch: %q", hints["gov::abc"])
	}
	if hints["obs::abc"] != "GuardianObserver" {
		t.Fatalf("GuardianObserver hint mismatch: %q", hints["obs::abc"])
	}
	if hints["admin1::abc"] != "ntt1-admin" {
		t.Fatalf("deployment admin hint mismatch: %q", hints["admin1::abc"])
	}
	if hints["alice::abc"] != "Alice" {
		t.Fatalf("user hint mismatch: %q", hints["alice::abc"])
	}
}

func TestKnownHints_EmptyState(t *testing.T) {
	s := state.New()
	hints := knownHints(s)
	if len(hints) != 0 {
		t.Fatalf("expected no hints for an empty state, got %+v", hints)
	}
}

// ----------------------------------------------------------------------
// toObservedOutput
// ----------------------------------------------------------------------

// ----------------------------------------------------------------------
// resolveParticipantRole: hint -> participant-role precedence
// (.claude/tasks/e2e-separate-participants.md §3: state.UserParticipants > config
// partyHosting > config "*" wildcard > "" (caller defers to profile.Endpoint's own default))
// ----------------------------------------------------------------------

func TestResolveParticipantRole_StateWinsOverConfig(t *testing.T) {
	s := state.New()
	s.UserParticipants["Alice"] = "bob" // deliberately "wrong" vs cfg, to prove state wins
	cfg := disclosure.Config{PartyHosting: map[string]string{"Alice": "app-user", "*": "app-provider"}}

	got := resolveParticipantRole("Alice", cfg, s)
	if got != "bob" {
		t.Fatalf("expected the state-persisted role to win, got %q", got)
	}
}

func TestResolveParticipantRole_ConfigWinsOverWildcard(t *testing.T) {
	s := state.New()
	cfg := disclosure.Config{PartyHosting: map[string]string{"Bob": "bob", "*": "app-provider"}}

	got := resolveParticipantRole("Bob", cfg, s)
	if got != "bob" {
		t.Fatalf("expected the config's specific mapping to win over the wildcard, got %q", got)
	}
}

func TestResolveParticipantRole_WildcardFallback(t *testing.T) {
	s := state.New()
	cfg := disclosure.Config{PartyHosting: disclosure.DefaultPartyHosting()}

	got := resolveParticipantRole("SomeoneNotListed", cfg, s)
	if got != "app-provider" {
		t.Fatalf("expected the config's \"*\" wildcard fallback, got %q", got)
	}
}

func TestResolveParticipantRole_NoMatchReturnsEmpty(t *testing.T) {
	s := state.New()
	cfg := disclosure.Config{} // no PartyHosting at all -- not even a "*" entry

	got := resolveParticipantRole("Anyone", cfg, s)
	if got != "" {
		t.Fatalf("expected an empty role when nothing resolves (deferring to profile.Endpoint's own default), got %q", got)
	}
}

func TestResolveParticipantRole_NilStateSkipsToConfig(t *testing.T) {
	cfg := disclosure.Config{PartyHosting: map[string]string{"Bob": "bob"}}
	got := resolveParticipantRole("Bob", cfg, nil)
	if got != "bob" {
		t.Fatalf("expected a nil state to fall through to the config mapping, got %q", got)
	}
}

func TestToObservedOutput(t *testing.T) {
	o := observer.Observed{
		EmitterAddress:   "aabb",
		Sequence:         5,
		Nonce:            6,
		ConsistencyLevel: 1,
		Payload:          "cc",
		EffectiveAt:      "2026-07-20T00:00:00Z",
		UpdateID:         "upd-1",
	}
	out := toObservedOutput(o)
	if out.EmitterChain != 72 {
		t.Fatalf("expected the constant cantonChainId=72, got %d", out.EmitterChain)
	}
	if out.EmitterAddress != "aabb" || out.Sequence != 5 || out.Nonce != 6 || out.ConsistencyLevel != 1 ||
		out.Payload != "cc" || out.EffectiveAt != o.EffectiveAt || out.UpdateID != "upd-1" {
		t.Fatalf("field passthrough mismatch: %+v", out)
	}
}

// TestCantonChainIDConstant pins wire.CantonChainID -- the single Go-side source of truth
// for the playground's fixed Wormhole chain id -- against the raw value every other pin in
// this codebase (pure_test.go/cmd_wiring_test.go/internal/wire/ntt_test.go's bare `72`
// assertions, and Test.TestNtt.cantonChainId on the Daml side) still checks independently.
func TestCantonChainIDConstant(t *testing.T) {
	if wire.CantonChainID != 72 {
		t.Fatalf("expected wire.CantonChainID == 72, got %d", wire.CantonChainID)
	}
}
