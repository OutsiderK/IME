package ghosttelemetry

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRecorderUsesStableEnvelopeWithoutText(t *testing.T) {
	var output bytes.Buffer
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	recorder := NewRecorder(&output, "session-test", func() time.Time { return now })
	opportunityID := recorder.NextOpportunityID()
	requestID := recorder.NextRequestID()
	candidateID := recorder.NextCandidateID()
	err := recorder.Record(EventCandidateReady, IDs{OpportunityID: opportunityID, RequestID: requestID}, CandidateReadyPayload{
		Candidates:   []CandidateFeature{{CandidateID: candidateID, FeatureVersion: 1, Rank: 1, Source: "llm", Runes: 4, NaturalEnd: true}},
		GenerationMS: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "secret context") || strings.Contains(output.String(), "candidate text") {
		t.Fatal("telemetry must not include raw context or candidate text")
	}
	events, err := Decode(&output)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].SessionID != "session-test" || events[0].OpportunityID != 1 || events[0].RequestID != 1 {
		t.Fatalf("unexpected envelope: %#v", events)
	}
}

func TestAggregateFixedFixture(t *testing.T) {
	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	metrics, err := Aggregate(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := metrics.TriggerRate["automatic"]; got.Numerator != 2 || got.Denominator != 3 {
		t.Fatalf("unexpected automatic trigger rate: %#v", got)
	}
	if metrics.AcceptanceAt1.Numerator != 0 || metrics.AcceptanceAt1.Denominator != 2 || metrics.AcceptanceAt3.Numerator != 1 {
		t.Fatalf("unexpected acceptance metrics: at1=%#v at3=%#v", metrics.AcceptanceAt1, metrics.AcceptanceAt3)
	}
	if metrics.PartialMatchRate.Numerator != 1 || metrics.PartialMatchRate.Denominator != 1 {
		t.Fatalf("unexpected partial match rate: %#v", metrics.PartialMatchRate)
	}
	if metrics.ReadyBeforeNextKeyRate.Numerator != 2 || metrics.ReadyBeforeNextKeyRate.Denominator != 3 {
		t.Fatalf("unexpected ready-before-next-key rate: %#v", metrics.ReadyBeforeNextKeyRate)
	}
	if metrics.LatencyP50MS != 30 || metrics.LatencyP90MS != 40 {
		t.Fatalf("unexpected latency percentiles: p50=%d p90=%d", metrics.LatencyP50MS, metrics.LatencyP90MS)
	}
	if metrics.QuickUndoRate.Numerator != 1 || metrics.LateFinishAfterCancelRate.Numerator != 1 {
		t.Fatalf("unexpected feedback/cancellation rates: quick=%#v late=%#v", metrics.QuickUndoRate, metrics.LateFinishAfterCancelRate)
	}
	if metrics.AcceptedRunesPer100Output < 66.6 || metrics.AcceptedRunesPer100Output > 66.7 {
		t.Fatalf("unexpected accepted rune utilization: %f", metrics.AcceptedRunesPer100Output)
	}
	if metrics.EstimatedSavedKeysPer100Output < 116.6 || metrics.EstimatedSavedKeysPer100Output > 116.7 {
		t.Fatalf("unexpected estimated saved keys: %f", metrics.EstimatedSavedKeysPer100Output)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	events, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateComplete(events); err != nil {
		t.Fatalf("fixture must contain closed lifecycles: %v", err)
	}
}
