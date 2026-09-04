package ghosttelemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type Rate struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Rate        float64 `json:"rate"`
}

type Metrics struct {
	TriggerRate                    map[string]Rate `json:"trigger_rate"`
	AcceptanceAt1                  Rate            `json:"acceptance_at_1"`
	AcceptanceAt3                  Rate            `json:"acceptance_at_3"`
	AcceptedRunesPer100Output      float64         `json:"accepted_runes_per_100_output"`
	ObservedKeyActionsPer100Output float64         `json:"observed_key_actions_per_100_output"`
	EstimatedSavedKeysPer100Output float64         `json:"estimated_saved_keys_per_100_output"`
	PartialMatchRate               Rate            `json:"partial_match_rate"`
	ReadyBeforeNextKeyRate         Rate            `json:"ready_before_next_key_rate"`
	LatencyP50MS                   int64           `json:"latency_p50_ms"`
	LatencyP90MS                   int64           `json:"latency_p90_ms"`
	InterruptionRate               Rate            `json:"interruption_rate"`
	QuickUndoRate                  Rate            `json:"quick_undo_rate"`
	LateFinishAfterCancelRate      Rate            `json:"late_finish_after_cancel_rate"`
}

type opportunitySummary struct {
	trigger    string
	shown      []uint64
	acceptedID uint64
	closed     OpportunityClosedPayload
	quickUndo  bool
	inputTimes []int64
}

type requestSummary struct {
	opportunityKey string
	started        int64
	ready          *int64
	cancelled      bool
	finished       RequestFinishedPayload
}

func Aggregate(r io.Reader) (Metrics, error) {
	events, err := Decode(r)
	if err != nil {
		return Metrics{}, err
	}
	opportunities := make(map[string]*opportunitySummary)
	requests := make(map[string]*requestSummary)
	var totalOutput, aiRunes, manualRunes, manualKeys, allKeys, acceptActions int
	for _, event := range events {
		opportunityKey := scopedID(event.SessionID, event.OpportunityID)
		requestKey := scopedID(event.SessionID, event.RequestID)
		switch event.Event {
		case EventOpportunityCreated:
			var payload OpportunityCreatedPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			opportunities[opportunityKey] = &opportunitySummary{trigger: payload.Trigger}
		case EventShown:
			var payload ShownPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			opp := ensureOpportunity(opportunities, opportunityKey)
			if len(opp.shown) == 0 {
				opp.shown = append([]uint64(nil), payload.CandidateIDs...)
			}
		case EventAccepted:
			var payload AcceptedPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			opp := ensureOpportunity(opportunities, opportunityKey)
			opp.acceptedID = event.CandidateID
			acceptActions += payload.AcceptActions
		case EventQuickUndo:
			ensureOpportunity(opportunities, opportunityKey).quickUndo = true
		case EventOpportunityClosed:
			var payload OpportunityClosedPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			ensureOpportunity(opportunities, opportunityKey).closed = payload
		case EventInputAdvanced:
			opp := ensureOpportunity(opportunities, opportunityKey)
			opp.inputTimes = append(opp.inputTimes, event.MonotonicTimeMS)
		case EventRequestStarted:
			requests[requestKey] = &requestSummary{opportunityKey: opportunityKey, started: event.MonotonicTimeMS}
		case EventCandidateReady:
			request := ensureRequest(requests, requestKey, opportunityKey)
			if request.ready == nil {
				ready := event.MonotonicTimeMS
				request.ready = &ready
			}
		case EventRequestCancelRequested:
			ensureRequest(requests, requestKey, opportunityKey).cancelled = true
		case EventRequestFinished:
			var payload RequestFinishedPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			ensureRequest(requests, requestKey, opportunityKey).finished = payload
		case EventTextCommitted:
			var payload TextCommittedPayload
			if err := decodePayload(event, &payload); err != nil {
				return Metrics{}, err
			}
			totalOutput += payload.OutputRunes
			allKeys += payload.PhysicalKeyActionsSincePreviousCommit
			if payload.Source == "ai" {
				aiRunes += payload.OutputRunes
			} else if payload.Source == "manual" {
				manualRunes += payload.OutputRunes
				manualKeys += payload.PhysicalKeyActionsSincePreviousCommit
			}
		}
	}

	metrics := Metrics{TriggerRate: make(map[string]Rate)}
	triggerCounts := make(map[string]*Rate)
	var shownCount, accepted1, accepted3, unacceptedShown, partial, interrupted, accepted, quickUndo int
	for _, opp := range opportunities {
		rate := triggerCounts[opp.trigger]
		if rate == nil {
			rate = &Rate{}
			triggerCounts[opp.trigger] = rate
		}
		rate.Denominator++
		if len(opp.shown) > 0 {
			rate.Numerator++
			shownCount++
			if opp.acceptedID != 0 {
				accepted++
				if opp.acceptedID == opp.shown[0] {
					accepted1++
				}
				for _, id := range opp.shown[:min(3, len(opp.shown))] {
					if id == opp.acceptedID {
						accepted3++
						break
					}
				}
				if opp.quickUndo {
					quickUndo++
				}
			} else {
				unacceptedShown++
				if opp.closed.MatchedPrefixRunes > 0 {
					partial++
				}
				if (opp.closed.Reason == "dismissed" || opp.closed.Reason == "new_input") && opp.closed.MatchedPrefixRunes == 0 {
					interrupted++
				}
			}
		}
	}
	for trigger, counts := range triggerCounts {
		metrics.TriggerRate[trigger] = newRate(counts.Numerator, counts.Denominator)
	}
	metrics.AcceptanceAt1 = newRate(accepted1, shownCount)
	metrics.AcceptanceAt3 = newRate(accepted3, shownCount)
	metrics.PartialMatchRate = newRate(partial, unacceptedShown)
	metrics.InterruptionRate = newRate(interrupted, shownCount)
	metrics.QuickUndoRate = newRate(quickUndo, accepted)

	var latencies []int64
	var readyBefore, observableNextKey, cancelled, lateFinish int
	for _, request := range requests {
		if request.ready != nil {
			latencies = append(latencies, *request.ready-request.started)
		}
		if request.cancelled {
			cancelled++
			if request.finished.Status == "completed" && request.finished.FinishedAfterCancel {
				lateFinish++
			}
		}
		opp := opportunities[request.opportunityKey]
		if opp == nil {
			continue
		}
		var nextKey *int64
		for _, inputTime := range opp.inputTimes {
			if inputTime > request.started && (nextKey == nil || inputTime < *nextKey) {
				t := inputTime
				nextKey = &t
			}
		}
		if nextKey != nil {
			observableNextKey++
			if request.ready != nil && *request.ready < *nextKey {
				readyBefore++
			}
		}
	}
	metrics.ReadyBeforeNextKeyRate = newRate(readyBefore, observableNextKey)
	metrics.LateFinishAfterCancelRate = newRate(lateFinish, cancelled)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	metrics.LatencyP50MS = percentile(latencies, 0.50)
	metrics.LatencyP90MS = percentile(latencies, 0.90)
	if totalOutput > 0 {
		metrics.AcceptedRunesPer100Output = float64(aiRunes) * 100 / float64(totalOutput)
		metrics.ObservedKeyActionsPer100Output = float64(allKeys) * 100 / float64(totalOutput)
		if manualRunes > 0 {
			manualKeysPerRune := float64(manualKeys) / float64(manualRunes)
			metrics.EstimatedSavedKeysPer100Output = (float64(aiRunes)*manualKeysPerRune - float64(acceptActions)) * 100 / float64(totalOutput)
		}
	}
	return metrics, nil
}

func decodePayload(event Envelope, dst any) error {
	if err := json.Unmarshal(event.Payload, dst); err != nil {
		return fmt.Errorf("decode %s payload: %w", event.Event, err)
	}
	return nil
}

func scopedID(session string, id uint64) string { return fmt.Sprintf("%s/%d", session, id) }

func ensureOpportunity(items map[string]*opportunitySummary, key string) *opportunitySummary {
	if items[key] == nil {
		items[key] = &opportunitySummary{}
	}
	return items[key]
}

func ensureRequest(items map[string]*requestSummary, key, opportunityKey string) *requestSummary {
	if items[key] == nil {
		items[key] = &requestSummary{opportunityKey: opportunityKey}
	}
	return items[key]
}

func newRate(numerator, denominator int) Rate {
	rate := Rate{Numerator: numerator, Denominator: denominator}
	if denominator > 0 {
		rate.Rate = float64(numerator) / float64(denominator)
	}
	return rate
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1)*p + 0.5)
	return sorted[index]
}
