package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// daPeer decodes Wormhole.Ntt.Manager's Peer record.
type daPeer struct {
	ManagerAddress     nttManagerAddressValue `json:"managerAddress"`
	TransceiverAddress emitterAddressValue    `json:"transceiverAddress"`
	Decimals           json.Number            `json:"decimals"`
}

// managerPeer is one peer deployment of an NttManager, over the wire. Both addresses are 64
// lowercase hex characters.
type managerPeer struct {
	ManagerAddress     string      `json:"managerAddress"`
	TransceiverAddress string      `json:"transceiverAddress"`
	Decimals           json.Number `json:"decimals"`
}

// nttPeerMap is the NttManager peers field, a Daml "Map Int Peer", keyed by the chain id in
// decimal. The JSON Ledger API encodes a Daml Map as an array of [key, value] pairs (the LocalNet
// scripts read CoreState.guardianSets, a "Map Int GuardianSet", that way). The decoder also
// accepts an object keyed by the chain id. A key may be a JSON string or a JSON number.
type nttPeerMap map[string]managerPeer

func (peers *nttPeerMap) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	decoded := make(nttPeerMap)
	switch {
	case len(trimmed) > 0 && trimmed[0] == '[':
		var pairs []json.RawMessage
		if err := json.Unmarshal(trimmed, &pairs); err != nil {
			return fmt.Errorf("peers: %w", err)
		}
		for index, rawPair := range pairs {
			var pair []json.RawMessage
			if err := json.Unmarshal(rawPair, &pair); err != nil || len(pair) != 2 {
				return fmt.Errorf("peers: entry %d is not a [chainId, peer] pair", index)
			}
			if err := decoded.add(pair[0], pair[1]); err != nil {
				return err
			}
		}
	case len(trimmed) > 0 && trimmed[0] == '{':
		var peersByKey map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &peersByKey); err != nil {
			return fmt.Errorf("peers: %w", err)
		}
		for key, rawPeer := range peersByKey {
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return fmt.Errorf("peers: encode key: %w", err)
			}
			if err := decoded.add(encodedKey, rawPeer); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("peers: not an array of [chainId, peer] pairs or an object")
	}
	*peers = decoded
	return nil
}

// add decodes one chain id and peer. Two entries for one chain id are an error.
func (peers nttPeerMap) add(rawChainID, rawPeer json.RawMessage) error {
	var chainIDNumber json.Number
	if err := json.Unmarshal(rawChainID, &chainIDNumber); err != nil {
		return fmt.Errorf("peers: chain id %s: %w", truncateForError(string(rawChainID)), err)
	}
	chainID, err := strconv.ParseInt(chainIDNumber.String(), 10, 64)
	if err != nil {
		return fmt.Errorf("peers: chain id %q is not an integer", truncateForError(chainIDNumber.String()))
	}
	key := strconv.FormatInt(chainID, 10)
	if _, duplicate := peers[key]; duplicate {
		return fmt.Errorf("peers: chain id %s appears twice", key)
	}
	var peer daPeer
	if err := json.Unmarshal(rawPeer, &peer); err != nil {
		return fmt.Errorf("peers: chain id %s: %w", key, err)
	}
	if peer.ManagerAddress.nttManagerAddressHexText == "" || peer.TransceiverAddress.hex == "" {
		return fmt.Errorf("peers: chain id %s: peer has no managerAddress or no transceiverAddress", key)
	}
	decimals, err := strconv.ParseInt(peer.Decimals.String(), 10, 64)
	if err != nil {
		return fmt.Errorf("peers: chain id %s: decimals %q is not an integer", key, truncateForError(peer.Decimals.String()))
	}
	peers[key] = managerPeer{
		ManagerAddress:     peer.ManagerAddress.nttManagerAddressHexText,
		TransceiverAddress: peer.TransceiverAddress.hex,
		Decimals:           json.Number(strconv.FormatInt(decimals, 10)),
	}
	return nil
}

// daNttManagerPeers decodes the peers field of an NttManager. The /v1/managers summary decodes it
// apart from daNttManager, so a peers value this service cannot read fails that route only, not
// the flows.
type daNttManagerPeers struct {
	Peers *nttPeerMap `json:"peers"`
}
