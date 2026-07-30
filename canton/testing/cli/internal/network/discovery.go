package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isLocalNetDir reports whether path is a directory containing compose.yaml -- the minimal
// signature of an extracted splice-node/docker-compose/localnet, cheap enough to probe every
// candidate below without shelling out.
func isLocalNetDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "compose.yaml")); err != nil {
		return false
	}
	return true
}

// localNetDirCandidates returns, in priority order, the non-LOCALNET_DIR locations
// ResolveLocalNetDir probes for an extracted Splice LocalNet bundle:
//
//  1. <cantonDir>/testing/.localnet/splice-node/docker-compose/localnet -- a repo-local cache
//     (gitignored) for whoever extracts the bundle straight into the checkout.
//  2. $HOME/.cache/ntt-playground/splice-node/docker-compose/localnet -- a shared per-user cache,
//     independent of any one checkout.
//  3. $HOME/splice-node/docker-compose/localnet -- where the release tarball's own `tar xzf`
//     invocation from $HOME (the README's own example) lands it.
func localNetDirCandidates(cantonDir string) []string {
	var candidates []string
	if cantonDir != "" {
		candidates = append(candidates, filepath.Join(cantonDir, "testing", ".localnet", "splice-node", "docker-compose", "localnet"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".cache", "ntt-playground", "splice-node", "docker-compose", "localnet"),
			filepath.Join(home, "splice-node", "docker-compose", "localnet"),
		)
	}
	return candidates
}

// ResolveLocalNetDir finds the extracted Splice LocalNet bundle's
// splice-node/docker-compose/localnet directory: $LOCALNET_DIR if set (validated, not just
// trusted), else the first of a short list of standard locations that actually contains a
// compose.yaml. cantonDir is the resolved canton/ root (used to check the repo-local cache);
// pass "" to skip that candidate.
//
// An explicit, set LOCALNET_DIR always wins over discovery, even when it's invalid -- a set
// env var is a deliberate choice, so a bad value is reported as an error rather than silently
// falling through to a discovered path the caller didn't ask for.
func ResolveLocalNetDir(cantonDir string) (string, error) {
	if dir := os.Getenv("LOCALNET_DIR"); dir != "" {
		if !isLocalNetDir(dir) {
			return "", fmt.Errorf("network: LOCALNET_DIR=%s is not a Splice LocalNet dir (no compose.yaml)", dir)
		}
		return dir, nil
	}

	candidates := localNetDirCandidates(cantonDir)
	for _, candidate := range candidates {
		if isLocalNetDir(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"network: no Splice LocalNet dir found; checked: %s -- set LOCALNET_DIR, or extract the Splice "+
			"*_splice-node.tar.gz release bundle so splice-node/docker-compose/localnet lands at one of those paths",
		strings.Join(candidates, ", "),
	)
}
