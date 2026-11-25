package main

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// specDisclosureV1 is this service's own specification of the /v1 routes. It is not vendored.
const specDisclosureV1 = "disclosure-v1.yaml"

func TestParseYAMLSubset_ReadsTheDisclosureV1Spec(t *testing.T) {
	want := []string{
		"GET /v1",
		"GET /v1/flows/{flow}",
		"GET /v1/healthz",
		"GET /v1/managers",
		"GET /v1/readyz",
	}
	sort.Strings(want)
	if got := loadOpenAPISpec(t, specDisclosureV1).operations(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("operations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestV1API_EveryOperationConformsToSpec exercises every operation of disclosure-v1.yaml, success
// and error, and requires that each response conforms to it.
func TestV1API_EveryOperationConformsToSpec(t *testing.T) {
	b := newBaseFixtures(tokenConfigLockUnlock)
	potCid := "pot-cid-1"
	fixtures := registerFlowFixtures(b.gg, testRegisterNextID, b.replayNodeJSON(""), claimedRootsTreeRootJSON(b.gg))
	peers := `[["2",` + peerJSON(testPeerManagerAddress, testPeerTransceiverAddress, `"18"`) + `]]`
	managerArg := managerArgWithSummaryFields(
		nttManagerArg(b.admin, b.gg, b.nttCustody, b.reserveCustodian(), b.namespace, b.managerAddr, b.transceiverEmitterID, b.instrumentAdmin, b.instrumentID, b.tokenConfig, b.factoryTag, b.factoryCid),
		`"6"`, peers)
	fixtures[tailNttManager] = jsonArray(acsContractJSONWithArg("pkg:"+tailNttManager, "mgr-cid-1", "bWdy", "sync1", managerArg))
	fixtures[tailLockedLedger] = jsonArray(b.ledgerJSON(&potCid))
	fixtures[tailCoinFactory] = jsonArray(b.factoryJSON())
	fixtures[tailCoin] = jsonArray(acsContractJSON("pkg:"+tailCoin, potCid, "Y29pbg==", "sync1"))
	srv := newFlowTestServer(t, fixtures)
	captureServerLog(srv)

	missingCoreFixtures := maps.Clone(fixtures)
	delete(missingCoreFixtures, tailCoreState)
	missingCoreServer := newFlowTestServer(t, missingCoreFixtures)
	captureServerLog(missingCoreServer)
	releasePath := "/v1/flows/release?manager=" + b.managerAddr + "&digest=" + b.digest

	type v1Request struct {
		srv          *server
		pathTemplate string
		target       string
		wantStatus   int
	}
	requests := []v1Request{
		{srv, "/v1", "/v1", 200},
		{srv, "/v1/healthz", "/v1/healthz", 200},
		{srv, "/v1/readyz", "/v1/readyz", 200},
		{srv, "/v1/managers", "/v1/managers", 200},
		{srv, "/v1/flows/{flow}", releasePath, 200},
		{srv, "/v1/flows/{flow}", "/v1/flows/register?" + testRegisterClaimQuery() + "&gg=" + b.gg, 200},
		{srv, "/v1/flows/{flow}", "/v1/flows/release?manager=zz", 400},
		{srv, "/v1/flows/{flow}", releasePath + "&gg=" + b.gg, 400},
		{srv, "/v1/flows/{flow}", "/v1/flows/nope", 404},
		{missingCoreServer, "/v1/flows/{flow}", releasePath, 404},
		{srv, "/v1/flows/{flow}", "/v1/flows/mint?manager=" + b.managerAddr + "&digest=" + b.digest + "&recipient=" + testRecipient, 409},
		{srv, "/v1/flows/{flow}", "/v1/flows/release?x=" + strings.Repeat("a", maxFlowQueryBytes), 414},
	}
	covered := make(map[string]bool)
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		request.srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, request.target, nil))
		requireStatus(t, recorder, request.wantStatus)
		requireConformsToSpec(t, recorder, specDisclosureV1, "GET", request.pathTemplate)
		covered["GET "+request.pathTemplate] = true
	}
	for _, operation := range loadOpenAPISpec(t, specDisclosureV1).operations() {
		if !covered[operation] {
			t.Errorf("%s %s has no conformance request", specDisclosureV1, operation)
		}
	}
}

func TestV1Spec_RejectsAnUndeclaredEntryField(t *testing.T) {
	spec := loadOpenAPISpec(t, specDisclosureV1)
	entry, err := spec.resolveLocalRef("#/components/schemas/DisclosureEntry")
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"role": "CoreState", "templateId": "p:M:E", "contractId": "00", "createdEventBlob": "AA==", "synchronizerId": "s::1", "createArgument": map[string]any{"a": []any{}}}
	if problems := spec.validateValue(entry, valid, "$", 0); len(problems) > 0 {
		t.Fatalf("valid entry rejected: %v", problems)
	}
	valid["payload"] = "x"
	if problems := spec.validateValue(entry, valid, "$", 0); len(problems) == 0 {
		t.Fatal("an undeclared entry field passed")
	}
	delete(valid, "payload")
	valid["createArgument"] = []any{}
	if problems := spec.validateValue(entry, valid, "$", 0); len(problems) == 0 {
		t.Fatal("a createArgument that is not an object passed")
	}
}
