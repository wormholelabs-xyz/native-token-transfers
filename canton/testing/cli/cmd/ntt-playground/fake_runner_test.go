package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// fakeCall records one Run invocation a fakeRunner observed: the script name and the input
// struct as passed by the caller (before JSON marshaling) -- wiring tests inspect both the
// CALL SEQUENCE (which script names ran, in what order) and routing (which party ended up in
// which field), never real Daml/ledger behavior (that's Playground.*.daml's own tests and
// the e2e suite's job).
type fakeCall struct {
	Script string
	Input  any
}

// fakeRunner is a scriptRunner double that never shells out to dpm: it records every call
// and, optionally, fills in a canned output (JSON round-tripped through the same
// input/output marshaling path `dpm script` would use) or returns a canned error, keyed by
// script name. Exists so deploy/transfer/receive's call sequence can be pinned fast and
// without a live sandbox -- these are Go-level wiring tests (call sequence + routing), not
// end-to-end ones; the Daml scripts themselves are exercised for real by the e2e suite
// (canton/testing/cli/e2e) against a live `dpm script` runner.
type fakeRunner struct {
	calls   []fakeCall
	outputs map[string]any
	errs    map[string]error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: map[string]any{}, errs: map[string]error{}}
}

func (f *fakeRunner) Run(_ context.Context, scriptName string, input, output any) error {
	f.calls = append(f.calls, fakeCall{Script: scriptName, Input: input})
	if err, ok := f.errs[scriptName]; ok {
		return err
	}
	out, ok := f.outputs[scriptName]
	if !ok || output == nil {
		return nil
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, output)
}

// scriptNames returns the sequence of script names f.Run was called with, in order.
func (f *fakeRunner) scriptNames() []string {
	names := make([]string, len(f.calls))
	for i, c := range f.calls {
		names[i] = c.Script
	}
	return names
}

// runPlaygroundWithRunner runs the root command in-process against a pre-built app whose
// script runner is overridden to r -- no real dpm/ledger is ever touched. Mirrors
// runPlayground (cmd_wiring_test.go) except it returns the *app used, so callers can inspect
// r after Execute() returns.
func runPlaygroundWithRunner(t *testing.T, r scriptRunner, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	a := &app{runnerOverride: r}
	root := newRootCmdForApp(a)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}
