package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// registryTemplateByTail is the package-qualified name of each template the registry API reads.
var registryTemplateByTail = map[string]string{
	tailNttManager:              "#ntt:Wormhole.Ntt.Manager:NttManager",
	tailCoinFactory:             "#token-cip0056:Token.CIP0056.CoinFactory:CoinFactory",
	tailCoin:                    "#token-cip0056:Token.CIP0056.Coin:Coin",
	tailTransferPreapproval:     "#token-cip0056:Token.CIP0056.CoinTransfer:TransferPreapproval",
	tailCoinTransferInstruction: "#token-cip0056:Token.CIP0056.CoinTransfer:CoinTransferInstruction",
	tailCoinAllocation:          "#token-cip0056:Token.CIP0056.CoinAllocation:CoinAllocation",
}

// registryReaderParties is the only reader party of every registry read: the registry admin.
//
// SECURITY: gg is a stakeholder of each registry template (NttManager, CoinFactory, Coin,
// CoinTransferInstruction, CoinAllocation as signatory; TransferPreapproval as observer).
func (s *server) registryReaderParties(registry *registryConfig) []partyID {
	guardianGovernance := s.opts.readerParties.guardianGovernance
	if guardianGovernance == "" {
		panic("disclosure-service: the registry API runs without a guardianGovernance party")
	}
	if guardianGovernance != registry.adminParty {
		panic("disclosure-service: the registry admin is not the guardianGovernance party")
	}
	return []partyID{guardianGovernance}
}

func registryQualifiedTemplate(tail string) string {
	template, known := registryTemplateByTail[tail]
	if !known {
		panic("disclosure-service: " + tail + " is not a registry template")
	}
	return template
}

// registryQuery reads tail as the registry admin, filtered by fieldMatches before maxMatches.
func (s *server) registryQuery(registry *registryConfig, tail string, offset int64, maxMatches int, fieldMatches ...createArgumentFieldMatch) activeContractsQuery {
	return activeContractsQuery{
		readerParties:  s.registryReaderParties(registry),
		template:       registryQualifiedTemplate(tail),
		activeAtOffset: offset,
		fieldMatches:   fieldMatches,
		maxMatches:     maxMatches,
	}
}

// registryInstrumentFieldMatches selects the burn/mint NttManagers whose instrument admin and
// guardianGovernance are the registry admin.
func registryInstrumentFieldMatches(registry *registryConfig) []createArgumentFieldMatch {
	return []createArgumentFieldMatch{
		exactFieldMatch(tokenConfigBurnMint, "tokenConfig"),
		partyFieldMatch(registry.adminParty, "instrumentId", "admin"),
		partyFieldMatch(registry.adminParty, "guardianGovernance"),
	}
}

// registryInstrument is one burn/mint instrument of the registry admin.
type registryInstrument struct {
	id         string
	decimals   int
	factoryCid contractID // the canonical CoinFactory the NttManager commits
}

// fetchRegistryInstruments lists the registry instruments, sorted by id. The query is the same
// for each request at one offset, so the cache serves repeats.
func (s *server) fetchRegistryInstruments(ctx context.Context, registry *registryConfig, offset int64) ([]registryInstrument, error) {
	query := s.registryQuery(registry, tailNttManager, offset, s.maxContracts, registryInstrumentFieldMatches(registry)...)
	managers, err := fetchDecodedQuery[daNttManager](ctx, s, query)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]registryInstrument, len(managers))
	for _, manager := range managers {
		value := manager.value
		//SECURITY: second check of the field matches.
		if value.TokenConfig != tokenConfigBurnMint {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s passed the registry filter with tokenConfig %s", manager.entry.ContractID, value.TokenConfig)
		}
		if value.InstrumentId.Admin != registry.adminParty {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s passed the registry filter with instrument admin %s", manager.entry.ContractID, value.InstrumentId.Admin)
		}
		if value.GuardianGovernance != registry.adminParty {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s passed the registry filter with guardianGovernance %s", manager.entry.ContractID, value.GuardianGovernance)
		}
		if err := requireFactoryTag(value.Factory, factoryTagBurnMint); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: %v", manager.entry.ContractID, err)
		}
		if value.Factory.Value == "" {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s commits an empty factory contract id", manager.entry.ContractID)
		}
		decimals, err := value.TokenDecimals.Int64()
		if err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: tokenDecimals %q is not an integer", manager.entry.ContractID, value.TokenDecimals)
		}
		if decimals < 0 {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: tokenDecimals %d is negative", manager.entry.ContractID, decimals)
		}
		if decimals > maxTokenDecimals {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: tokenDecimals %d is above %d", manager.entry.ContractID, decimals, maxTokenDecimals)
		}
		if err := checkRegistryIdentifier("NttManager instrumentId.id", value.InstrumentId.Id); err != nil {
			return nil, newFlowError(http.StatusInternalServerError, "NttManager %s: %v", manager.entry.ContractID, err)
		}
		instrument := registryInstrument{id: value.InstrumentId.Id, decimals: int(decimals), factoryCid: contractID(value.Factory.Value)}
		if previous, seen := byID[instrument.id]; seen && previous != instrument {
			return nil, newFlowError(http.StatusInternalServerError, "NttManagers disagree on instrument %q: %+v and %+v", instrument.id, previous, instrument)
		}
		byID[instrument.id] = instrument
	}
	instruments := make([]registryInstrument, 0, len(byID))
	for _, instrument := range byID {
		instruments = append(instruments, instrument)
	}
	sort.Slice(instruments, func(i, j int) bool { return instruments[i].id < instruments[j].id })
	return instruments, nil
}

// findRegistryInstrument selects instrumentID from the registry instruments.
func (s *server) findRegistryInstrument(ctx context.Context, registry *registryConfig, instrumentID string, offset int64) (registryInstrument, error) {
	instruments, err := s.fetchRegistryInstruments(ctx, registry, offset)
	if err != nil {
		return registryInstrument{}, err
	}
	for _, instrument := range instruments {
		if instrument.id == instrumentID {
			return instrument, nil
		}
	}
	return registryInstrument{}, newFlowError(http.StatusNotFound, "instrument %q is not a registry instrument", truncateForError(instrumentID))
}

// fetchTotalSupplies sums the active Coin amounts of the registry admin's instruments by id. The
// read keeps only the sums and its read time.
func (s *server) fetchTotalSupplies(ctx context.Context, registry *registryConfig, offset int64) (amountTotalsRead, error) {
	query := amountTotalsQuery{
		contracts: activeContractsQuery{
			readerParties:  s.registryReaderParties(registry),
			template:       registryQualifiedTemplate(tailCoin),
			activeAtOffset: offset,
			fieldMatches:   []createArgumentFieldMatch{partyFieldMatch(registry.adminParty, "instrumentId", "admin")},
		},
		groupFieldPath:  []string{"instrumentId", "id"},
		amountFieldPath: []string{"amount"},
		maxGroups:       maxRegistryInstruments,
	}
	totals, err := s.acs.ActiveContractAmountTotals(ctx, query)
	if err != nil {
		return amountTotalsRead{}, &flowError{status: upstreamErrorStatus(err), msg: fmt.Sprintf("disclosure-service: Coin totals: %v", err), cause: err}
	}
	if totals.readAt.IsZero() {
		return amountTotalsRead{}, newFlowError(http.StatusInternalServerError, "Coin totals have no read time")
	}
	return totals, nil
}

// registrySupplyTotals is the Coin totals of one request. available is false when the Coin read
// stopped at a read bound; the metadata routes then omit totalSupply and totalSupplyAsOf.
type registrySupplyTotals struct {
	read      amountTotalsRead
	available bool
}

// isUpstreamReadBoundError reports a read that stopped at the participant list limit (413), the
// scanned-contract bound, or the response byte limit.
func isUpstreamReadBoundError(err error) bool {
	return errors.Is(err, errUpstreamListLimitExceeded) ||
		errors.Is(err, errUpstreamScanBoundExceeded) ||
		errors.Is(err, errUpstreamResponseTooLarge)
}

// isCoinTotalsReadBoundError adds the group cap to isUpstreamReadBoundError.
func isCoinTotalsReadBoundError(err error) bool {
	return isUpstreamReadBoundError(err) || errors.Is(err, errAmountTotalsGroupCapExceeded)
}

// isPreapprovalReadBoundError adds the match cap to isUpstreamReadBoundError.
func isPreapprovalReadBoundError(err error) bool {
	return isUpstreamReadBoundError(err) || errors.Is(err, errMatchCapExceeded)
}

// fetchTotalSuppliesWithinBounds is fetchTotalSupplies. A read that stops at a bound gives
// unavailable totals and one log line. Any other error fails the request.
func (s *server) fetchTotalSuppliesWithinBounds(ctx context.Context, registry *registryConfig, offset int64) (registrySupplyTotals, error) {
	read, err := s.fetchTotalSupplies(ctx, registry, offset)
	if err == nil {
		return registrySupplyTotals{read: read, available: true}, nil
	}
	if !isCoinTotalsReadBoundError(err) {
		return registrySupplyTotals{}, err
	}
	s.logger.Printf("disclosure-service: registry: totalSupply omitted: the Coin read stopped at a bound: %q", err.Error())
	return registrySupplyTotals{available: false}, nil
}

// registryContractNotFound is the one 404 text of a read by contract id. Do not give the cause
// to the client: it shows whether gg sees a contract with that id.
func registryContractNotFound(tail string) error {
	return newFlowError(http.StatusNotFound, "no active %s with this id in this registry", templateTail(tail))
}

// fetchRegistryContractByID reads contractID as the registry admin. A contract the admin cannot
// see, an archived contract, and a contract of another package or template are each a 404 with
// one text.
func fetchRegistryContractByID[T any](ctx context.Context, s *server, registry *registryConfig, tail string, wantContractID contractID) (decodedEntry[T], error) {
	entry, found, err := s.acs.ActiveContractByID(ctx, s.registryReaderParties(registry), wantContractID)
	if err != nil {
		return decodedEntry[T]{}, &flowError{status: upstreamErrorStatus(err), msg: fmt.Sprintf("disclosure-service: read %s %s: %v", tail, wantContractID, err), cause: err}
	}
	if !found {
		return decodedEntry[T]{}, registryContractNotFound(tail)
	}
	if err := checkUpstreamTemplate(registryQualifiedTemplate(tail), entry.PackageName, entry.TemplateID); err != nil {
		s.logger.Printf("disclosure-service: registry: 404 for %s %q: %q", templateTail(tail), string(wantContractID), err.Error())
		return decodedEntry[T]{}, registryContractNotFound(tail)
	}
	if contractID(entry.ContractID) != wantContractID {
		return decodedEntry[T]{}, newFlowError(http.StatusBadGateway, "read %s returned contract %s", wantContractID, entry.ContractID)
	}
	var decodedValue T
	if err := json.Unmarshal(entry.CreateArgument, &decodedValue); err != nil {
		return decodedEntry[T]{}, newFlowError(http.StatusInternalServerError, "decode %s %s createArgument: %v", tail, wantContractID, err)
	}
	return decodedEntry[T]{entry: entry, value: decodedValue}, nil
}

// fetchCanonicalFactory reads the CoinFactory that the instrument's NttManager commits, by
// contract id, and checks that the registry admin administers it.
func (s *server) fetchCanonicalFactory(ctx context.Context, registry *registryConfig, instrument registryInstrument) (decodedEntry[daCoinFactory], error) {
	factory, err := fetchRegistryContractByID[daCoinFactory](ctx, s, registry, tailCoinFactory, instrument.factoryCid)
	if err != nil {
		var flowErr *flowError
		if errors.As(err, &flowErr) && flowErr.status == http.StatusNotFound {
			return decodedEntry[daCoinFactory]{}, newFlowError(http.StatusInternalServerError, "committed CoinFactory %s of instrument %q: %v", instrument.factoryCid, instrument.id, err)
		}
		return decodedEntry[daCoinFactory]{}, err
	}
	//SECURITY: CoinFactory.daml:32-33 serves only instruments whose admin is the factory admin.
	if err := checkCoinFactoryAdminIsGuardianGovernance(factory, registry.adminParty); err != nil {
		return decodedEntry[daCoinFactory]{}, err
	}
	return factory, nil
}

// fetchReceiverPreapproval reads the receiver's TransferPreapproval for instrument. found is
// false when there is none.
func (s *server) fetchReceiverPreapproval(ctx context.Context, registry *registryConfig, receiver partyID, instrument daInstrumentId, offset int64) (decodedEntry[daPreapproval], bool, error) {
	query := s.registryQuery(registry, tailTransferPreapproval, offset, maxMatchingContractsPerQuery, preapprovalFieldMatches(receiver, instrument)...)
	preapprovals, err := fetchDecodedQuery[daPreapproval](ctx, s, query)
	if err != nil {
		return decodedEntry[daPreapproval]{}, false, err
	}
	return findReceiverPreapproval(preapprovals, receiver, instrument)
}

// findReceiverPreapproval returns the receiver's TransferPreapproval for instrument, the lowest
// contract id first when several exist. found is false when there is none.
func findReceiverPreapproval(entries []decodedEntry[daPreapproval], receiver partyID, instrument daInstrumentId) (decodedEntry[daPreapproval], bool, error) {
	var chosen *decodedEntry[daPreapproval]
	for index := range entries {
		candidate := &entries[index]
		if candidate.value.Owner != receiver {
			return decodedEntry[daPreapproval]{}, false, newFlowError(http.StatusInternalServerError, "TransferPreapproval %s passed the receiver filter with owner %s", candidate.entry.ContractID, candidate.value.Owner)
		}
		if candidate.value.InstrumentId != instrument {
			return decodedEntry[daPreapproval]{}, false, newFlowError(http.StatusInternalServerError, "TransferPreapproval %s passed the instrument filter with instrument %+v", candidate.entry.ContractID, candidate.value.InstrumentId)
		}
		if chosen == nil || candidate.entry.ContractID < chosen.entry.ContractID {
			chosen = candidate
		}
	}
	if chosen == nil {
		return decodedEntry[daPreapproval]{}, false, nil
	}
	return *chosen, true, nil
}

// registryDisclosure renders entry as a DisclosedContract.
func registryDisclosure(entry acsEntry, excludeDebugFields bool) registryDisclosedContract {
	disclosed := registryDisclosedContract{
		TemplateID:       entry.TemplateID,
		ContractID:       entry.ContractID,
		CreatedEventBlob: entry.CreatedEventBlob,
		SynchronizerID:   entry.SynchronizerID,
	}
	if excludeDebugFields {
		return disclosed
	}
	disclosed.DebugPackageName = entry.PackageName
	if isJSONObject(entry.CreateArgument) {
		disclosed.DebugPayload = entry.CreateArgument
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.CreatedAt); err == nil {
		disclosed.DebugCreatedAt = entry.CreatedAt
	}
	return disclosed
}
