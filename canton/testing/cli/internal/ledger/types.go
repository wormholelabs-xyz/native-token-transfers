package ledger

// PubKeyHint mirrors a Daml `(Int, Bytes)` tuple's JSON encoding -- the pubKeys hints
// ParseAndVerifyVAA/VerifyAndConsumeVAA/Receive take. Daml's built-in tuple types are
// records with fields `_1`/`_2` under the Daml-LF JSON mapping `dpm script` uses for its
// --input-file/--output-file, so this must be an object, not a 2-element JSON array.
type PubKeyHint struct {
	Index int    `json:"_1"`
	Key   string `json:"_2"`
}
