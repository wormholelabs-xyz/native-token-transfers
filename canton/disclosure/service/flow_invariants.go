package main

import (
	"errors"
	"fmt"
	"net/http"
)

// checkOneSynchronizer requires each disclosure to name the same, non-empty synchronizer. A
// command submission runs on one synchronizer.
func checkOneSynchronizer(disclosures []flowDisclosureEntry) error {
	if len(disclosures) == 0 {
		return errors.New("disclosure set has no disclosures")
	}
	synchronizerID := disclosures[0].SynchronizerID
	for _, disclosure := range disclosures {
		if disclosure.SynchronizerID == "" {
			return fmt.Errorf("disclosure %s (%s) has no synchronizer", disclosure.ContractID, disclosure.Role)
		}
		if disclosure.SynchronizerID != synchronizerID {
			return fmt.Errorf("disclosure set spans synchronizers %s and %s", synchronizerID, disclosure.SynchronizerID)
		}
	}
	return nil
}

// checkFlowResponseInvariants is the postcondition of every assembled set.
func checkFlowResponseInvariants(response *flowResponse) error {
	if response.Flow == "" {
		return errors.New("flow response has no flow name")
	}
	if len(response.Disclosures) == 0 {
		return errors.New("flow response has no disclosures")
	}
	if response.Missing == nil {
		return errors.New("flow response missing list is nil")
	}
	seenContractIDs := make(map[string]bool, len(response.Disclosures))
	disclosedRoles := make(map[string]bool, len(response.Disclosures))
	for _, disclosure := range response.Disclosures {
		if disclosure.ContractID == "" {
			return fmt.Errorf("disclosure %s has no contract id", disclosure.Role)
		}
		if disclosure.TemplateID == "" {
			return fmt.Errorf("disclosure %s has no template id", disclosure.Role)
		}
		if disclosure.CreatedEventBlob == "" {
			return fmt.Errorf("disclosure %s has no createdEventBlob", disclosure.Role)
		}
		if seenContractIDs[disclosure.ContractID] {
			return fmt.Errorf("flow response holds contract %s twice", disclosure.ContractID)
		}
		seenContractIDs[disclosure.ContractID] = true
		if disclosedRoles[disclosure.Role] {
			return fmt.Errorf("flow response holds role %s twice", disclosure.Role)
		}
		disclosedRoles[disclosure.Role] = true
	}
	missingRoles := make(map[string]bool, len(response.Missing))
	for _, missing := range response.Missing {
		if disclosedRoles[missing.Role] {
			return fmt.Errorf("role %s is both disclosed and missing", missing.Role)
		}
		if missingRoles[missing.Role] {
			return fmt.Errorf("flow response holds missing role %s twice", missing.Role)
		}
		missingRoles[missing.Role] = true
	}
	return nil
}

// checkAssembledFlowResponse runs both set postconditions. A violation is a 500.
func checkAssembledFlowResponse(response *flowResponse) error {
	if err := checkFlowResponseInvariants(response); err != nil {
		return newFlowError(http.StatusInternalServerError, "flow %s: %v", response.Flow, err)
	}
	if err := checkOneSynchronizer(response.Disclosures); err != nil {
		return newFlowError(http.StatusInternalServerError, "flow %s: %v", response.Flow, err)
	}
	return nil
}
