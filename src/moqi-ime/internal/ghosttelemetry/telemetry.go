package ghosttelemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const SchemaVersion = 1

const (
	EventOpportunityCreated     = "opportunity_created"
	EventRequestStarted         = "request_started"
	EventCandidateReady         = "candidate_ready"
	EventRequestCancelRequested = "request_cancel_requested"
	EventRequestFinished        = "request_finished"
	EventShown                  = "shown"
	EventCycled                 = "cycled"
	EventAccepted               = "accepted"
	EventInputAdvanced          = "input_advanced"
	EventTextCommitted          = "text_committed"
	EventQuickUndo              = "quick_undo"
	EventOpportunityClosed      = "opportunity_closed"
)

type IDs struct {
	OpportunityID uint64
	RequestID     uint64
	CandidateID   uint64
}

type Envelope struct {
	SchemaVersion   int             `json:"schema_version"`
	Time            string          `json:"time"`
	MonotonicTimeMS int64           `json:"monotonic_time_ms"`
	SessionID       string          `json:"session_id"`
	OpportunityID   uint64          `json:"opportunity_id,omitempty"`
	RequestID       uint64          `json:"request_id,omitempty"`
	CandidateID     uint64          `json:"candidate_id,omitempty"`
	Event           string          `json:"event"`
	Payload         json.RawMessage `json:"payload"`
}

type OpportunityCreatedPayload struct {
	Mode               string `json:"mode"`
	CompositionVersion uint64 `json:"composition_version"`
	Trigger            string `json:"trigger"`
}

type RequestStartedPayload struct {
	Generator           string `json:"generator"`
	RequestedCandidates int    `json:"requested_candidates"`
}

type CandidateFeature struct {
	CandidateID       uint64   `json:"candidate_id"`
	FeatureVersion    int      `json:"feature_version"`
	Rank              int      `json:"rank"`
	Source            string   `json:"source"`
	Runes             int      `json:"runes"`
	MeanLogprob       *float64 `json:"mean_logprob,omitempty"`
	TerminalEntropy   *float64 `json:"terminal_entropy,omitempty"`
	NaturalEnd        bool     `json:"natural_end"`
	PersonalFrequency *int     `json:"personal_frequency,omitempty"`
}

type CandidateReadyPayload struct {
	Candidates   []CandidateFeature `json:"candidates"`
	GenerationMS int64              `json:"generation_ms"`
}

type RequestCancelRequestedPayload struct {
	Reason string `json:"reason"`
}

type RequestFinishedPayload struct {
	Status              string `json:"status"`
	FinishedAfterCancel bool   `json:"finished_after_cancel"`
	ElapsedMS           int64  `json:"elapsed_ms"`
}

type ShownPayload struct {
	CandidateIDs []uint64 `json:"candidate_ids"`
}

type CycledPayload struct {
	FromCandidateID uint64 `json:"from_candidate_id"`
	ToCandidateID   uint64 `json:"to_candidate_id"`
}

type AcceptedPayload struct {
	AcceptedRunes int `json:"accepted_runes"`
	AcceptActions int `json:"accept_actions"`
}

type InputAdvancedPayload struct {
	CompositionVersion uint64 `json:"composition_version"`
}

type TextCommittedPayload struct {
	Source                                string `json:"source"`
	OutputRunes                           int    `json:"output_runes"`
	PhysicalKeyActionsSincePreviousCommit int    `json:"physical_key_actions_since_previous_commit"`
}

type QuickUndoPayload struct {
	RemovedRunes int   `json:"removed_runes"`
	ElapsedMS    int64 `json:"elapsed_ms"`
}

type OpportunityClosedPayload struct {
	Reason             string `json:"reason"`
	MatchedCandidateID uint64 `json:"matched_candidate_id,omitempty"`
	MatchedPrefixRunes int    `json:"matched_prefix_runes,omitempty"`
	ManualRunes        int    `json:"manual_runes,omitempty"`
}

type Recorder struct {
	mu                sync.Mutex
	w                 io.Writer
	closer            io.Closer
	now               func() time.Time
	started           time.Time
	sessionID         string
	nextOpportunityID uint64
	nextRequestID     uint64
	nextCandidateID   uint64
}

func NewRecorder(w io.Writer, sessionID string, now func() time.Time) *Recorder {
	if now == nil {
		now = time.Now
	}
	started := now()
	if sessionID == "" {
		sessionID = fmt.Sprintf("%d-%d", os.Getpid(), started.UnixMilli())
	}
	return &Recorder{w: w, now: now, started: started, sessionID: sessionID}
}

func NewFileRecorder(logDir string) (*Recorder, string, error) {
	if logDir == "" {
		return nil, "", fmt.Errorf("telemetry log directory is empty")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create telemetry log directory: %w", err)
	}
	path := filepath.Join(logDir, "ai-completion-events-"+time.Now().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open telemetry log: %w", err)
	}
	recorder := NewRecorder(f, "", nil)
	recorder.closer = f
	return recorder, path, nil
}

func (r *Recorder) NextOpportunityID() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextOpportunityID++
	return r.nextOpportunityID
}

func (r *Recorder) NextRequestID() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextRequestID++
	return r.nextRequestID
}

func (r *Recorder) NextCandidateID() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextCandidateID++
	return r.nextCandidateID
}

func (r *Recorder) Record(event string, ids IDs, payload any) error {
	if r == nil || r.w == nil {
		return nil
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telemetry payload: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	envelope := Envelope{
		SchemaVersion:   SchemaVersion,
		Time:            now.Format(time.RFC3339Nano),
		MonotonicTimeMS: now.Sub(r.started).Milliseconds(),
		SessionID:       r.sessionID,
		OpportunityID:   ids.OpportunityID,
		RequestID:       ids.RequestID,
		CandidateID:     ids.CandidateID,
		Event:           event,
		Payload:         payloadJSON,
	}
	line, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal telemetry event: %w", err)
	}
	line = append(line, '\n')
	if _, err := r.w.Write(line); err != nil {
		return fmt.Errorf("write telemetry event: %w", err)
	}
	return nil
}

func (r *Recorder) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closer.Close()
}

func Decode(r io.Reader) ([]Envelope, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var events []Envelope
	line := 0
	for scanner.Scan() {
		line++
		var event Envelope
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode telemetry line %d: %w", line, err)
		}
		if event.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("unsupported telemetry schema %d on line %d", event.SchemaVersion, line)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read telemetry: %w", err)
	}
	return events, nil
}

func ValidateComplete(events []Envelope) error {
	type lifecycleCount struct{ started, finished int }
	opportunities := make(map[string]lifecycleCount)
	requests := make(map[string]lifecycleCount)
	candidates := make(map[string]struct{})
	lastMonotonic := make(map[string]int64)
	for _, event := range events {
		if previous, ok := lastMonotonic[event.SessionID]; ok && event.MonotonicTimeMS < previous {
			return fmt.Errorf("session %s monotonic time moved backwards", event.SessionID)
		}
		lastMonotonic[event.SessionID] = event.MonotonicTimeMS
		opportunityKey := fmt.Sprintf("%s/%d", event.SessionID, event.OpportunityID)
		requestKey := fmt.Sprintf("%s/%d", event.SessionID, event.RequestID)
		switch event.Event {
		case EventOpportunityCreated:
			count := opportunities[opportunityKey]
			count.started++
			opportunities[opportunityKey] = count
		case EventOpportunityClosed:
			count := opportunities[opportunityKey]
			count.finished++
			opportunities[opportunityKey] = count
		case EventRequestStarted:
			count := requests[requestKey]
			count.started++
			requests[requestKey] = count
		case EventRequestFinished:
			count := requests[requestKey]
			count.finished++
			requests[requestKey] = count
		case EventCandidateReady:
			var payload CandidateReadyPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return fmt.Errorf("decode candidate_ready payload: %w", err)
			}
			for _, candidate := range payload.Candidates {
				candidates[fmt.Sprintf("%s/%d", event.SessionID, candidate.CandidateID)] = struct{}{}
			}
		case EventAccepted:
			if _, ok := candidates[fmt.Sprintf("%s/%d", event.SessionID, event.CandidateID)]; !ok {
				return fmt.Errorf("accepted candidate %d was never ready", event.CandidateID)
			}
		case EventShown:
			var payload ShownPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return fmt.Errorf("decode shown payload: %w", err)
			}
			for _, candidateID := range payload.CandidateIDs {
				if _, ok := candidates[fmt.Sprintf("%s/%d", event.SessionID, candidateID)]; !ok {
					return fmt.Errorf("shown candidate %d was never ready", candidateID)
				}
			}
		}
	}
	for key, count := range opportunities {
		if count.started != 1 || count.finished != 1 {
			return fmt.Errorf("opportunity %s lifecycle is %d created/%d closed", key, count.started, count.finished)
		}
	}
	for key, count := range requests {
		if count.started != 1 || count.finished != 1 {
			return fmt.Errorf("request %s lifecycle is %d started/%d finished", key, count.started, count.finished)
		}
	}
	return nil
}
