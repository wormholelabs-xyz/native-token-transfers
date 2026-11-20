package main

import (
	"fmt"
	"strings"
	"unicode"
)

// maxPartyIDBytes is the Canton party identifier length limit.
const maxPartyIDBytes = 255

// readerRole names the party as which the service reads one template.
type readerRole int

const (
	readerGuardianGovernance readerRole = iota + 1
	readerNttCustody
)

// readerPartiesByRole holds the configured party of each reader role.
type readerPartiesByRole struct {
	guardianGovernance partyID
	nttCustody         partyID
}

func (parties readerPartiesByRole) partyFor(role readerRole) partyID {
	switch role {
	case readerGuardianGovernance:
		return parties.guardianGovernance
	case readerNttCustody:
		return parties.nttCustody
	default:
		panic(fmt.Sprintf("disclosure-service: unknown reader role %d", role))
	}
}

// readerRolesByTail names the only reader roles of each flow template.
//
// SECURITY: each party reads only what its role needs. gg signs every core and NTT root,
// including ReplayRootRegistry.
var readerRolesByTail = map[string][]readerRole{
	tailNttManager:            {readerGuardianGovernance},
	tailLockedLedger:          {readerGuardianGovernance},
	tailCoreState:             {readerGuardianGovernance},
	tailEmitter:               {readerGuardianGovernance},
	tailReplayNode:            {readerGuardianGovernance},
	tailNttGovernance:         {readerGuardianGovernance},
	tailEmitterRegistry:       {readerGuardianGovernance},
	tailCoinFactory:           {readerGuardianGovernance},
	tailDepositPreapproval:    {readerGuardianGovernance},
	tailAdminTransferProposal: {readerGuardianGovernance},
	tailTransferPreapproval:   {readerGuardianGovernance, readerNttCustody},
	tailCoin:                  {readerNttCustody},
	tailReplayRootRegistry:    {readerGuardianGovernance},
}

func init() {
	if len(readerRolesByTail) != len(flowTemplateByTail) {
		panic("disclosure-service: readerRolesByTail and flowTemplateByTail have different sizes")
	}
	for tail := range flowTemplateByTail {
		if len(readerRolesByTail[tail]) == 0 {
			panic("disclosure-service: no reader role for flow template " + tail)
		}
	}
}

// readersFor returns the reader parties of tail.
func (s *server) readersFor(tail string) []partyID {
	roles, known := readerRolesByTail[tail]
	if !known {
		panic("disclosure-service: no reader role for " + tail)
	}
	parties := make([]partyID, 0, len(roles))
	for _, role := range roles {
		party := s.opts.readerParties.partyFor(role)
		if party == "" {
			panic(fmt.Sprintf("disclosure-service: reader role %d of %s has no party", role, tail))
		}
		parties = append(parties, party)
	}
	return parties
}

// parseReaderParty checks one party flag value. An empty optional value is "".
func parseReaderParty(flagName, rawParty string, required bool) (partyID, error) {
	if rawParty == "" {
		if required {
			return "", fmt.Errorf("disclosure-service: --%s is required", flagName)
		}
		return "", nil
	}
	if len(rawParty) > maxPartyIDBytes {
		return "", fmt.Errorf("disclosure-service: --%s is %d bytes, the limit is %d", flagName, len(rawParty), maxPartyIDBytes)
	}
	if strings.TrimSpace(rawParty) != rawParty {
		return "", fmt.Errorf("disclosure-service: --%s has surrounding whitespace", flagName)
	}
	if strings.ContainsFunc(rawParty, func(character rune) bool {
		return character == ',' || unicode.IsSpace(character) || unicode.IsControl(character)
	}) {
		return "", fmt.Errorf("disclosure-service: --%s must be one party id", flagName)
	}
	return partyID(rawParty), nil
}

// parseReaderParties builds the two roles and requires them to be distinct parties.
func parseReaderParties(rawGuardianGovernance, rawNttCustody string) (readerPartiesByRole, error) {
	guardianGovernance, err := parseReaderParty("guardian-governance-party", rawGuardianGovernance, true)
	if err != nil {
		return readerPartiesByRole{}, err
	}
	nttCustody, err := parseReaderParty("ntt-custody-party", rawNttCustody, true)
	if err != nil {
		return readerPartiesByRole{}, err
	}
	if guardianGovernance == nttCustody {
		return readerPartiesByRole{}, fmt.Errorf("disclosure-service: --guardian-governance-party and --ntt-custody-party must be distinct parties")
	}
	return readerPartiesByRole{guardianGovernance: guardianGovernance, nttCustody: nttCustody}, nil
}
