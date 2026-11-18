package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemovedRawEndpointServesNothing: each /v1/disclosures request is a 404 with zero upstream calls.
func TestRemovedRawEndpointServesNothing(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"no template param", "/v1/disclosures"},
		{"custody holding template", "/v1/disclosures?template=Token.CIP0056.Coin:Coin"},
		{"manager template", "/v1/disclosures?template=Wormhole.Ntt.Manager:NttManager"},
		{"package-qualified template", "/v1/disclosures?template=%23ntt:Wormhole.Ntt.Manager:NttManager"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &countingACS{offset: 42}
			srv := newTestServerWithClient(t, upstream)

			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Zero(t, upstream.ledgerCalls.Load())
			assert.Zero(t, upstream.acCalls.Load())
		})
	}
}

func TestParseFlags_AllowListFlagRemoved(t *testing.T) {
	_, err := parseFlags(append([]string{"--json-api", "http://127.0.0.1:6975", "--allow-list", "list.json"}, testPartyFlags...))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allow-list")
}

func TestFlowTemplateByTailIsPackageQualified(t *testing.T) {
	require.Len(t, flowTemplateByTail, 13)
	for tail, qualifiedTemplate := range flowTemplateByTail {
		assert.Equal(t, tail, templateTail(qualifiedTemplate))
		assert.Regexp(t, `^#[a-z0-9-]+:[A-Za-z0-9.]+:[A-Za-z0-9]+$`, qualifiedTemplate)
	}
}
