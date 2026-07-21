package observer

import (
	"encoding/hex"
	"testing"

	"github.com/wormholelabs-xyz/native-token-transfers/canton/testing/cli/internal/wire"
)

// exercisedUpdateFixture returns one `POST /v2/updates` (or WS) response frame carrying a
// single consuming PublishMessage ExercisedEvent, with sequence/emitterId/nonce encoded as
// JSON numbers or strings depending on numericAsString. Splice LocalNet's JSON Ledger API
// emits both forms, so the decoder must tolerate either. The shape (including the Update-kind
// "value" wrapper and the flat ExercisedEvent fields) matches what a running 0.6.12 stack
// returns from the `POST /v2/updates` blocking-list fallback.
func exercisedUpdateFixture(t *testing.T, sequence, emitterID, nonce string, numericAsString bool) []byte {
	t.Helper()
	quote := func(v string) string {
		if numericAsString {
			return `"` + v + `"`
		}
		return v
	}
	raw := `{
	  "update": {
	    "Transaction": {
	      "value": {
	        "updateId": "upd-1",
	        "commandId": "cmd-1",
	        "workflowId": "",
	        "effectiveAt": "2026-07-20T12:00:00Z",
	        "offset": 42,
	        "synchronizerId": "global-domain::1220deadbeef",
	        "recordTime": "2026-07-20T12:00:01Z",
	        "events": [
	          {
	            "ExercisedEvent": {
	              "offset": 42,
	              "nodeId": 3,
	              "contractId": "00aa",
	              "templateId": "abc123:Wormhole.Core.State:Emitter",
	              "interfaceId": null,
	              "choice": "PublishMessage",
	              "choiceArgument": {},
	              "actingParties": ["operator::abc"],
	              "consuming": true,
	              "witnessParties": ["operator::abc", "guardianObserver::xyz"],
	              "lastDescendantNodeId": 3,
	              "exerciseResult": {
	                "registrar": "operator::abc",
	                "owner": "admin::def",
	                "emitterId": ` + quote(emitterID) + `,
	                "sequence": ` + quote(sequence) + `,
	                "nonce": ` + quote(nonce) + `,
	                "consistencyLevel": 0,
	                "payload": "9945ff10aabbcc"
	              },
	              "packageName": "wormhole-core",
	              "implementedInterfaces": [],
	              "acsDelta": true
	            }
	          }
	        ]
	      }
	    }
	  }
	}`
	return []byte(raw)
}

func TestDecodeUpdateFrame_ExtractsPublishMessage(t *testing.T) {
	frame := exercisedUpdateFixture(t, "1", "7", "42", false)
	observed, err := DecodeUpdateFrame(frame)
	if err != nil {
		t.Fatalf("DecodeUpdateFrame: %v", err)
	}
	if len(observed) != 1 {
		t.Fatalf("expected 1 observed message, got %d: %+v", len(observed), observed)
	}
	m := observed[0]
	if m.Registrar != "operator::abc" || m.Owner != "admin::def" {
		t.Fatalf("registrar/owner mismatch: %+v", m)
	}
	if m.EmitterID != 7 || m.Sequence != 1 || m.Nonce != 42 || m.ConsistencyLevel != 0 {
		t.Fatalf("numeric field mismatch: %+v", m)
	}
	if m.Payload != "9945ff10aabbcc" {
		t.Fatalf("payload mismatch: %q", m.Payload)
	}
	if m.UpdateID != "upd-1" || m.EffectiveAt != "2026-07-20T12:00:00Z" || m.Offset != 42 {
		t.Fatalf("transaction-level fields mismatch: %+v", m)
	}
}

// TestDecodeUpdateFrame_NumericOrString checks the json.Number tolerance:
// sequence/emitterId/nonce may render as a JSON number or a quoted string depending on the
// participant's int64 rendering, and both must decode identically.
func TestDecodeUpdateFrame_NumericOrString(t *testing.T) {
	asNumber, err := DecodeUpdateFrame(exercisedUpdateFixture(t, "5", "9", "3", false))
	if err != nil {
		t.Fatalf("DecodeUpdateFrame (numeric): %v", err)
	}
	asString, err := DecodeUpdateFrame(exercisedUpdateFixture(t, "5", "9", "3", true))
	if err != nil {
		t.Fatalf("DecodeUpdateFrame (string): %v", err)
	}
	if len(asNumber) != 1 || len(asString) != 1 {
		t.Fatalf("expected exactly one observed message from each fixture")
	}
	if asNumber[0] != asString[0] {
		t.Fatalf("numeric and string renderings decoded differently: %+v vs %+v", asNumber[0], asString[0])
	}
}

// TestDecodeUpdateFrame_DerivedAddressMatchesWire proves the observer's derived
// EmitterAddress uses the SAME domain-separated hash internal/wire.DerivedAddress computes
// for the transceiver Emitter, not a reimplementation that could silently drift.
func TestDecodeUpdateFrame_DerivedAddressMatchesWire(t *testing.T) {
	observed, err := DecodeUpdateFrame(exercisedUpdateFixture(t, "1", "7", "0", false))
	if err != nil {
		t.Fatalf("DecodeUpdateFrame: %v", err)
	}
	want := hex.EncodeToString(func() []byte {
		addr := wire.DerivedAddress(wire.EmitterAddressTag, "operator::abc", "admin::def", 7)
		return addr[:]
	}())
	if observed[0].EmitterAddress != want {
		t.Fatalf("derived emitter address mismatch: got %s want %s", observed[0].EmitterAddress, want)
	}
}

// TestDecodeUpdateFrame_SkipsNonPublishMessage proves the decoder ignores exercises that
// aren't a consuming PublishMessage (e.g. a non-consuming exercise, or a different choice),
// so an update frame incidental to some other contract doesn't get mistaken for an
// observation.
func TestDecodeUpdateFrame_SkipsNonPublishMessage(t *testing.T) {
	raw := `{
	  "update": {
	    "Transaction": {
	      "value": {
	        "updateId": "upd-2",
	        "effectiveAt": "2026-07-20T12:00:00Z",
	        "offset": 43,
	        "events": [
	          {
	            "ExercisedEvent": {
	              "offset": 43,
	              "nodeId": 1,
	              "contractId": "00bb",
	              "templateId": "abc123:Some.Other:Template",
	              "choice": "SomeOtherChoice",
	              "consuming": true,
	              "exerciseResult": {"foo": "bar"}
	            }
	          },
	          {
	            "CreatedEvent": {
	              "offset": 43,
	              "nodeId": 2,
	              "contractId": "00cc",
	              "templateId": "abc123:Wormhole.Core.State:Emitter"
	            }
	          }
	        ]
	      }
	    }
	  }
	}`
	observed, err := DecodeUpdateFrame([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeUpdateFrame: %v", err)
	}
	if len(observed) != 0 {
		t.Fatalf("expected no observed messages, got %+v", observed)
	}
}

// TestDecodeUpdateFrame_SkipsNonTransactionUpdates checks that OffsetCheckpoint (and other
// non-Transaction update kinds such as Reassignment and TopologyTransaction) are skipped
// rather than treated as errors.
func TestDecodeUpdateFrame_SkipsNonTransactionUpdates(t *testing.T) {
	raw := `{"update": {"OffsetCheckpoint": {"value": {"offset": 44}}}}`
	observed, err := DecodeUpdateFrame([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeUpdateFrame: %v", err)
	}
	if len(observed) != 0 {
		t.Fatalf("expected no observed messages from an OffsetCheckpoint frame, got %+v", observed)
	}
}
