package disclosure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeScriptRunner records the last Run call so tests can assert what a Service actually asked
// for, without touching `dpm`.
type fakeScriptRunner struct {
	scriptName string
	input      any
	output     json.RawMessage
	err        error
}

func (f *fakeScriptRunner) Run(ctx context.Context, scriptName string, input, output any) error {
	f.scriptName = scriptName
	f.input = input
	if f.err != nil {
		return f.err
	}
	if out, ok := output.(*json.RawMessage); ok {
		*out = f.output
	}
	return nil
}

func testConfig() Config {
	return Config{
		Disclose: []Entry{
			{Template: "Wormhole.Core.State:CoreState", FetchAs: "GuardianGovernance"},
			{Template: "Wormhole.Ntt.Manager:NttManager", FetchAs: "GuardianGovernance"},
		},
	}
}

// ----------------------------------------------------------------------
// GET /v1/disclosures -- allow-list enforcement
// ----------------------------------------------------------------------

func TestHandleDisclosures_UnlistedTemplate_Forbidden(t *testing.T) {
	acsSrv := newACSTestServer(t, 1, acsCannedResponse)
	svc := &Service{
		Cfg: testConfig(),
		ACS: &ActiveContracts{BaseURL: acsSrv.URL, Party: "gg::party"},
	}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/disclosures?template=Evil:Template")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var body bytes.Buffer
	_, _ = body.ReadFrom(resp.Body)
	require.Contains(t, body.String(), "Evil:Template")
}

func TestHandleDisclosures_ListedTemplate_OK(t *testing.T) {
	acsSrv := newACSTestServer(t, 1, acsCannedResponse)
	svc := &Service{
		Cfg: testConfig(),
		ACS: &ActiveContracts{BaseURL: acsSrv.URL, Party: "gg::party"},
	}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/disclosures?template=Wormhole.Core.State:CoreState")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out disclosuresResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Len(t, out.Contracts, 1)
	require.Equal(t, "cid-1", out.Contracts[0].ContractID)
}

func TestHandleDisclosures_MissingTemplateParam_BadRequest(t *testing.T) {
	svc := &Service{Cfg: testConfig()}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/disclosures")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleDisclosures_NilACS_ServiceUnavailable(t *testing.T) {
	svc := &Service{Cfg: testConfig()} // ACS deliberately nil
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/disclosures?template=Wormhole.Core.State:CoreState")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// ----------------------------------------------------------------------
// POST /v1/seam/{name} -- server-side discloseTemplates injection
// ----------------------------------------------------------------------

func TestHandleSeam_InjectsServerTemplates_IgnoresClientSupplied(t *testing.T) {
	runner := &fakeScriptRunner{output: json.RawMessage(`{"mgr":null}`)}
	svc := &Service{
		Cfg: testConfig(),
		NewRunner: func(ctx context.Context) (ScriptRunner, func(), error) {
			return runner, func() {}, nil
		},
	}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	clientBody := `{"operator": "Operator", "discloseTemplates": ["Evil:Template"]}`
	resp, err := http.Post(srv.URL+"/v1/seam/transferOut", "application/json", bytes.NewBufferString(clientBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody bytes.Buffer
	_, _ = respBody.ReadFrom(resp.Body)
	require.JSONEq(t, `{"mgr":null}`, respBody.String())

	require.Equal(t, "Playground.Prepare:prepareTransferOut", runner.scriptName)
	input, ok := runner.input.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Operator", input["operator"])

	gotTemplates, ok := input["discloseTemplates"].([]string)
	require.True(t, ok)
	require.Equal(t, testConfig().Templates(), gotTemplates,
		"the server's own allow-list must win over whatever the client sent")
}

func TestHandleSeam_ReceiveAndPublish_MapToCorrectScripts(t *testing.T) {
	cases := map[string]string{
		"receive":             "Playground.Prepare:prepareReceive",
		"publish":             "Playground.Prepare:preparePublish",
		"acceptAdminTransfer": "Playground.Prepare:prepareAcceptAdmin",
	}
	for name, wantScript := range cases {
		runner := &fakeScriptRunner{output: json.RawMessage(`{}`)}
		svc := &Service{
			Cfg: testConfig(),
			NewRunner: func(ctx context.Context) (ScriptRunner, func(), error) {
				return runner, func() {}, nil
			},
		}
		srv := httptest.NewServer(svc.Handler())

		resp, err := http.Post(srv.URL+"/v1/seam/"+name, "application/json", bytes.NewBufferString(`{}`))
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, wantScript, runner.scriptName)
		srv.Close()
	}
}

func TestHandleSeam_UnknownName_NotFound(t *testing.T) {
	svc := &Service{Cfg: testConfig()}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/v1/seam/bogus", "application/json", bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleSeam_RunnerError_BadGateway(t *testing.T) {
	runner := &fakeScriptRunner{err: errors.New("dpm script failed")}
	svc := &Service{
		Cfg: testConfig(),
		NewRunner: func(ctx context.Context) (ScriptRunner, func(), error) {
			return runner, func() {}, nil
		},
	}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/v1/seam/transferOut", "application/json", bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

// ----------------------------------------------------------------------
// GET /v1/healthz
// ----------------------------------------------------------------------

func TestHandleHealthz(t *testing.T) {
	acsSrv := newACSTestServer(t, 55, "[]")
	svc := &Service{
		Cfg:         testConfig(),
		Participant: "guardian-governance",
		ACS:         &ActiveContracts{BaseURL: acsSrv.URL, Party: "gg::party"},
	}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out healthzResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, "guardian-governance", out.Participant)
	require.Equal(t, testConfig().Templates(), out.Templates)
	require.Equal(t, int64(55), out.LedgerEnd)
}
