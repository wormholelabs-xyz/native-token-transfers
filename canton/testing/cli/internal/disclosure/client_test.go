package disclosure

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClient_Disclosures_RoundTrip runs Client against a real Service.Handler backed by a
// canned ACS server -- proves the two independently-tested halves (Client's HTTP encoding,
// Service's HTTP decoding) agree on the wire.
func TestClient_Disclosures_RoundTrip(t *testing.T) {
	acsSrv := newACSTestServer(t, 12, acsCannedResponse)
	svc := &Service{
		Cfg: testConfig(),
		ACS: &ActiveContracts{BaseURL: acsSrv.URL, Party: "gg::party"},
	}
	svcSrv := httptest.NewServer(svc.Handler())
	t.Cleanup(svcSrv.Close)

	cl := &Client{BaseURL: svcSrv.URL}
	contracts, offset, err := cl.Disclosures(context.Background(), []string{"Wormhole.Core.State:CoreState"})
	require.NoError(t, err)
	require.Equal(t, int64(12), offset)
	require.Len(t, contracts, 1)
	require.Equal(t, "cid-1", contracts[0].ContractID)
	require.Equal(t, knownBlobHex, contracts[0].Blob)
}

// TestClient_Disclosures_SurfacesForbiddenOffender proves the 403 offender text (the plan's
// §5.2) survives the client round-trip, not just the raw HTTP handler test.
func TestClient_Disclosures_SurfacesForbiddenOffender(t *testing.T) {
	svc := &Service{Cfg: testConfig()}
	svcSrv := httptest.NewServer(svc.Handler())
	t.Cleanup(svcSrv.Close)

	cl := &Client{BaseURL: svcSrv.URL}
	_, _, err := cl.Disclosures(context.Background(), []string{"Evil:Template"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Evil:Template")
	require.Contains(t, err.Error(), "403")
}

// TestClient_Seam_RoundTrip proves a client-supplied discloseTemplates is overridden
// server-side and the fake runner's RemoteSeam output comes back verbatim.
func TestClient_Seam_RoundTrip(t *testing.T) {
	runner := &fakeScriptRunner{output: json.RawMessage(`{"mgr": {"managerId": 3}}`)}
	svc := &Service{
		Cfg: testConfig(),
		NewRunner: func(ctx context.Context) (ScriptRunner, func(), error) {
			return runner, func() {}, nil
		},
	}
	svcSrv := httptest.NewServer(svc.Handler())
	t.Cleanup(svcSrv.Close)

	cl := &Client{BaseURL: svcSrv.URL}
	input := map[string]any{
		"operator":          "Operator",
		"discloseTemplates": []string{"Evil:Template"},
	}
	out, err := cl.Seam(context.Background(), "transferOut", input)
	require.NoError(t, err)
	require.JSONEq(t, `{"mgr": {"managerId": 3}}`, string(out))

	require.Equal(t, "Playground.Prepare:prepareTransferOut", runner.scriptName)
	gotInput, ok := runner.input.(map[string]any)
	require.True(t, ok)
	gotTemplates, ok := gotInput["discloseTemplates"].([]string)
	require.True(t, ok)
	require.Equal(t, testConfig().Templates(), gotTemplates)
}

func TestClient_Seam_UnknownName_SurfacesNotFound(t *testing.T) {
	svc := &Service{Cfg: testConfig()}
	svcSrv := httptest.NewServer(svc.Handler())
	t.Cleanup(svcSrv.Close)

	cl := &Client{BaseURL: svcSrv.URL}
	_, err := cl.Seam(context.Background(), "bogus", map[string]any{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}
