package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerEndOffset(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantOffset int64
		wantErr    error
	}{
		{"positive offset", `{"offset":42}`, 42, nil},
		{"offset one", `{"offset":1}`, 1, nil},
		{"offset missing", `{}`, 0, errLedgerEndOffsetMissing},
		{"offset null", `{"offset":null}`, 0, errLedgerEndOffsetMissing},
		{"offset zero", `{"offset":0}`, 0, errLedgerEndOffsetNotPositive},
		{"offset negative", `{"offset":-1}`, 0, errLedgerEndOffsetNotPositive},
		{"offset a string", `{"offset":"x"}`, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(upstream.Close)

			offset, err := newTestHTTPACSClient(upstream.URL, nil).LedgerEnd(context.Background())
			if tc.wantOffset == 0 {
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.ErrorIs(t, err, tc.wantErr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOffset, offset)
		})
	}
}

func TestCachingACSRejectsNonPositiveOffset(t *testing.T) {
	cases := []struct {
		name    string
		offset  int64
		wantErr bool
	}{
		{"zero", 0, true},
		{"negative", -5, true},
		{"one", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := newCachingACS(&countingACS{offset: tc.offset}, time.Second)
			offset, err := cache.LedgerEnd(context.Background())
			if tc.wantErr {
				assert.ErrorIs(t, err, errLedgerEndOffsetNotPositive)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.offset, offset)
		})
	}
}
