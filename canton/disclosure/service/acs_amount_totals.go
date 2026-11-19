package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

var errAmountTotalsGroupCapExceeded = errors.New("disclosure-service: amount totals exceed the group cap")

// amountTotalsQuery sums one Daml Decimal field per group over the contracts that pass the field
// matches. The scan bound and maxGroups bound it.
type amountTotalsQuery struct {
	contracts       activeContractsQuery // template, readers, offset and field matches
	groupFieldPath  []string
	amountFieldPath []string
	maxGroups       int
}

// amountTotalsByGroup maps a group value to its total, scaled by 10^10.
type amountTotalsByGroup map[string]*big.Int

// amountTotalsRead is the totals of one upstream read and the time the read started.
type amountTotalsRead struct {
	totalsByGroup amountTotalsByGroup
	readAt        time.Time
}

func (read amountTotalsRead) clone() amountTotalsRead {
	return amountTotalsRead{totalsByGroup: read.totalsByGroup.clone(), readAt: read.readAt}
}

func (totals amountTotalsByGroup) clone() amountTotalsByGroup {
	cloned := make(amountTotalsByGroup, len(totals))
	for group, total := range totals {
		cloned[group] = new(big.Int).Set(total)
	}
	return cloned
}

func (query amountTotalsQuery) check() error {
	if len(query.contracts.readerParties) == 0 {
		return errors.New("disclosure-service: amount totals: at least one reader party is required")
	}
	if query.contracts.template == "" {
		return errors.New("disclosure-service: amount totals: template is empty")
	}
	if query.contracts.activeAtOffset < 1 {
		return fmt.Errorf("%w: activeAtOffset %d", errLedgerEndOffsetNotPositive, query.contracts.activeAtOffset)
	}
	if len(query.groupFieldPath) == 0 {
		return errors.New("disclosure-service: amount totals: group field path is empty")
	}
	if len(query.amountFieldPath) == 0 {
		return errors.New("disclosure-service: amount totals: amount field path is empty")
	}
	if query.maxGroups < 1 {
		return fmt.Errorf("disclosure-service: amount totals: maxGroups %d is not positive", query.maxGroups)
	}
	return nil
}

func (query amountTotalsQuery) cacheKey() string {
	encodedKey, err := json.Marshal(struct {
		Kind      string
		Contracts string
		Group     []string
		Amount    []string
		MaxGroups int
	}{"amount-totals", query.contracts.cacheKey(), query.groupFieldPath, query.amountFieldPath, query.maxGroups})
	if err != nil {
		panic(fmt.Sprintf("disclosure-service: encode cache key: %v", err))
	}
	return string(encodedKey)
}

// ActiveContractAmountTotals reads query.contracts.template without createdEventBlobs and sums
// the amount field per group value. A contract two readers both see is counted once.
func (c *httpACSClient) ActiveContractAmountTotals(ctx context.Context, query amountTotalsQuery) (amountTotalsRead, error) {
	if err := query.check(); err != nil {
		return amountTotalsRead{}, err
	}
	bodyBytes, err := marshalActiveContractsRequest(query.contracts, false)
	if err != nil {
		return amountTotalsRead{}, err
	}
	readAt := c.now()
	resp, err := c.sendUpstreamRequest(ctx, "active-contracts", http.MethodPost, "/v2/state/active-contracts", bodyBytes)
	if err != nil {
		return amountTotalsRead{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return amountTotalsRead{}, fmt.Errorf("%w: template %s: %w", errUpstreamListLimitExceeded, query.contracts.template, newUpstreamHTTPError("active-contracts", resp))
	}
	if resp.StatusCode != http.StatusOK {
		return amountTotalsRead{}, newUpstreamHTTPError("active-contracts", resp)
	}
	body := &boundedBodyReader{source: resp.Body, remainingBytes: c.maxResponseBytes}
	totals, err := decodeActiveContractAmountTotals(body, query)
	if body.limitExceeded {
		return amountTotalsRead{}, fmt.Errorf("disclosure-service: amount totals %s: %w", query.contracts.template, errUpstreamResponseTooLarge)
	}
	if err != nil {
		return amountTotalsRead{}, fmt.Errorf("disclosure-service: amount totals %s: %w", query.contracts.template, err)
	}
	if readAt.IsZero() {
		panic("disclosure-service: amount totals read has no read time")
	}
	return amountTotalsRead{totalsByGroup: totals, readAt: readAt}, nil
}

// decodeActiveContractAmountTotals reads one JSON array element at a time. Each kept contract
// must have a group value and a positive amount.
func decodeActiveContractAmountTotals(body io.Reader, query amountTotalsQuery) (amountTotalsByGroup, error) {
	decoder := json.NewDecoder(body)
	if err := expectJSONDelimiter(decoder, '['); err != nil {
		return nil, err
	}
	totals := make(amountTotalsByGroup)
	seenContractIDs := make(map[string]bool)
	scannedCount := 0
	for decoder.More() {
		scannedCount++
		if scannedCount > maxScannedContractsPerQuery {
			return nil, fmt.Errorf("%w: more than %d", errUpstreamScanBoundExceeded, maxScannedContractsPerQuery)
		}
		var responseEntry acsResponseEntry
		if err := decoder.Decode(&responseEntry); err != nil {
			return nil, wrapStreamError(err)
		}
		activeContract := responseEntry.ContractEntry.JsActiveContract
		if activeContract == nil || activeContract.CreatedEvent == nil {
			continue // other contractEntry variant
		}
		createdEvent := activeContract.CreatedEvent
		if err := checkUpstreamTemplate(query.contracts.template, createdEvent.PackageName, createdEvent.TemplateID); err != nil {
			return nil, err
		}
		if seenContractIDs[createdEvent.ContractID] {
			continue // visible to more than one reader party
		}
		seenContractIDs[createdEvent.ContractID] = true
		if !query.contracts.matchesCreateArgument(createdEvent.CreateArgument) {
			continue
		}
		groupValue, found := lookupCreateArgumentScalar(createdEvent.CreateArgument, query.groupFieldPath)
		if !found {
			return nil, fmt.Errorf("%w: contract %s has no group field", errUpstreamResponseMalformed, createdEvent.ContractID)
		}
		amountText, found := lookupCreateArgumentScalar(createdEvent.CreateArgument, query.amountFieldPath)
		if !found {
			return nil, fmt.Errorf("%w: contract %s has no amount field", errUpstreamResponseMalformed, createdEvent.ContractID)
		}
		amountScaled, err := parseDamlDecimalScaled(amountText)
		if err != nil {
			return nil, fmt.Errorf("%w: contract %s amount: %v", errUpstreamResponseMalformed, createdEvent.ContractID, err)
		}
		if amountScaled.Sign() <= 0 {
			return nil, fmt.Errorf("%w: contract %s amount %s is not positive", errUpstreamResponseMalformed, createdEvent.ContractID, amountText)
		}
		total, known := totals[groupValue]
		if !known {
			if len(totals) == query.maxGroups {
				return nil, fmt.Errorf("%w: more than %d", errAmountTotalsGroupCapExceeded, query.maxGroups)
			}
			total = new(big.Int)
			totals[groupValue] = total
		}
		total.Add(total, amountScaled)
	}
	if err := expectJSONDelimiter(decoder, ']'); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if errors.Is(err, errUpstreamResponseTooLarge) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: data after the array", errUpstreamResponseMalformed)
	}
	if scannedCount > maxScannedContractsPerQuery {
		panic("disclosure-service: decodeActiveContractAmountTotals exceeded the scan bound")
	}
	if len(totals) > query.maxGroups {
		panic("disclosure-service: decodeActiveContractAmountTotals exceeded the group cap")
	}
	return totals, nil
}
