package rime

import (
	contextpkg "context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gaboolic/moqi-ime/imecore"
	"github.com/gaboolic/moqi-ime/internal/ghosttelemetry"
)

const (
	ghostMessageDuration      = -1
	ghostActionKeyCode        = vkF8
	ghostNextKeyCode          = vkF9
	ghostShortMaxRunes        = 16
	ghostLongMaxRunes         = 56
	ghostContextMaxSentences  = 6
	defaultGhostLoadingDelay  = time.Second
	ghostPreservedKeyGUID     = "{7b5cfb72-41d8-4c76-a819-65b698f91a34}"
	ghostLongPreservedKeyGUID = "{018e48c0-a24f-4b72-bacc-57a2d53284b9}"
	ghostNextPreservedKeyGUID = "{316649a5-9c0b-4708-a43d-7bedc5bde9bb}"
	ghostContextEnvelope      = "\x1eMOQI_CONTEXT_V1\x1f"
	ghostFeatureVersion       = 1
	ghostQuickUndoWindow      = 3 * time.Second
)

type ghostRequestState struct {
	id                     uint64
	opportunityID          uint64
	cancel                 contextpkg.CancelFunc
	cancelRequested        bool
	started                time.Time
	telemetry              *ghosttelemetry.Recorder
	closeTelemetryOnFinish bool
}

type ghostDeferredClose struct {
	opportunityID uint64
	reason        string
	context       string
	candidates    []string
	candidateIDs  []uint64
}

type ghostPendingCommit struct {
	opportunityID uint64
	candidateID   uint64
}

type ghostAcceptedState struct {
	opportunityID  uint64
	candidateID    uint64
	acceptedAt     time.Time
	remainingRunes int
	undoRecorded   bool
}

func ghostPreservedKeyInfos() []imecore.PreservedKeyInfo {
	// TSF preserved-key callbacks have caused repeatable host-process crashes
	// in Electron/Chromium applications. Keep removing legacy registrations,
	// and rely on the ordinary key sink only when the host delivers F8/F9.
	return nil
}

func (ime *IME) ghostCompletionEnabled() bool {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	return ime.ghostEnabled
}

func (ime *IME) configureGhostTelemetry(enabled bool) {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if !enabled {
		recorder := ime.ghostTelemetry
		ime.ghostTelemetry = nil
		if recorder != nil {
			if ime.ghostRequest != nil && ime.ghostRequest.telemetry == recorder {
				ime.ghostRequest.closeTelemetryOnFinish = true
			} else {
				_ = recorder.Close()
			}
		}
		return
	}
	if ime.ghostTelemetry != nil {
		return
	}
	recorder, path, err := ghosttelemetry.NewFileRecorder(rimeLogDir())
	if err != nil {
		debugLogf("AI completion telemetry disabled: %v", err)
		return
	}
	ime.ghostTelemetry = recorder
	debugLogf("AI completion telemetry path=%q", path)
}

func (ime *IME) configureGhostCompletion(cfg *aiRuntimeConfig) {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	ime.resetGhostCompletionLockedWithReason("ai_disabled")
	ime.ghostEnabled = false
	ime.ghostGenerator = nil
	if cfg == nil || !cfg.Completion.Enabled || !ime.productSettings.AIEnabled {
		return
	}
	client := newAIClient(cfg)
	if client == nil || !isLoopbackAIEndpoint(client.baseURL) {
		debugLogf("本地 AI 已禁用：接口必须是 127.0.0.1、localhost 或 ::1")
		return
	}
	ime.ghostConfig = cfg.Completion
	ime.ghostGenerator = managedCompletionGenerator(
		client,
		ime.productSettings.AIRunMode,
		ime.productSettings.AIIdleExitMinutes,
	)
	ime.ghostEnabled = true
	debugLogf("Ghost completion configured idle_ms=%d context_tokens=%d candidates=%d", ime.ghostConfig.IdleMS, ime.ghostConfig.ContextTokens, ime.ghostConfig.CandidateCount)
	if ime.productSettings.AIRunMode == aiRunModeResident {
		go func() {
			if err := sharedLocalAIRuntime.ensure(client, aiRunModeResident, 0); err != nil {
				debugLogf("Local AI resident warmup skipped: %v", err)
			}
		}()
	}
}

func (ime *IME) handleGhostKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil {
		return false
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if !ime.ghostEnabled {
		return false
	}
	if req.KeyCode == vkEscape && ime.hasGhostWorkLocked() {
		ime.resetGhostCompletionLockedWithReason("dismissed")
		// Some TSF hosts act on Escape immediately after filterKeyDown. Keep the
		// complete keystroke inside the IME so the ghost window reliably closes.
		ime.ghostConsumeKeyUpCode = vkEscape
		ime.ghostHidePending = true
		resp.HideMessage = true
		resp.ReturnValue = 1
		return true
	}
	// Always claim F8 through the ordinary TSF key sink. OnKeyDown runs inside
	// a normal edit session and can safely read fresh surrounding text, unlike
	// the crash-prone preserved-key callback that is disabled above.
	if req.KeyCode == ghostActionKeyCode {
		ime.ghostConsumeKeyUpCode = ghostActionKeyCode
		resp.ReturnValue = 1
		return true
	}
	if req.KeyCode == ghostNextKeyCode && ime.ghostVisible && ime.ghostReady {
		ime.ghostConsumeKeyUpCode = ghostNextKeyCode
		resp.ReturnValue = 1
		return true
	}
	if ime.ghostVisible && isLeftAlt(req) {
		ime.ghostConsumeKeyUpCode = vkMenu
		resp.ReturnValue = 1
		return true
	}
	if shouldInvalidateGhostForKey(req) && ime.hasGhostWorkLocked() {
		wasVisible := ime.ghostVisible || ime.ghostLoadingVisible
		ime.recordGhostInputAdvancedLocked()
		reason := "new_input"
		if isGhostCursorMovement(req.KeyCode) {
			reason = "cursor_moved"
		}
		ime.cancelGhostRequestLocked(reason)
		ime.deferOrCloseGhostOpportunityLocked(reason)
		ime.clearGhostStateLocked()
		if wasVisible {
			ime.ghostHidePending = true
			resp.HideMessage = true
		}
	}
	return false
}

func (ime *IME) handleGhostKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil {
		return false
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if ime.ghostConsumeKeyUpCode == 0 {
		return false
	}
	if req.KeyCode != ime.ghostConsumeKeyUpCode {
		return false
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleGhostKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil {
		return false
	}
	if req.KeyCode == ghostActionKeyCode {
		guid := ghostPreservedKeyGUID
		if req.KeyStates.IsKeyDown(vkShift) {
			guid = ghostLongPreservedKeyGUID
		}
		ordinaryRequest := *req
		ordinaryRequest.Data = map[string]interface{}{"guid": guid}
		return ime.handleGhostPreservedKey(&ordinaryRequest, resp)
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if !ime.ghostEnabled {
		return false
	}
	if req.KeyCode == vkEscape && ime.ghostHidePending {
		resp.HideMessage = true
		resp.ReturnValue = 1
		ime.ghostHidePending = false
		return true
	}
	if req.KeyCode == ghostNextKeyCode && ime.ghostVisible && ime.ghostReady {
		ime.cycleGhostCandidateLocked()
		ime.fillGhostResponseLocked(resp)
		resp.ReturnValue = 1
		return true
	}
	if ime.ghostVisible && isLeftAlt(req) {
		ime.cycleGhostCandidateLocked()
		ime.fillGhostResponseLocked(resp)
		resp.ReturnValue = 1
		return true
	}
	return false
}

// handleGhostPreservedKey handles F8 through TSF's preserved-key path. Some
// hosts (notably rich-text editors) reserve F8 themselves and never deliver it
// through the ordinary key sink, so the regular filterKeyDown path cannot make
// the shortcut reliable there.
func (ime *IME) handleGhostPreservedKey(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil || req.Data == nil {
		return false
	}
	guid, _ := req.Data["guid"].(string)
	isShort := strings.EqualFold(guid, ghostPreservedKeyGUID)
	isLong := strings.EqualFold(guid, ghostLongPreservedKeyGUID)
	isNext := strings.EqualFold(guid, ghostNextPreservedKeyGUID)
	if !isShort && !isLong && !isNext {
		return false
	}
	if isNext {
		ime.ghostMu.Lock()
		defer ime.ghostMu.Unlock()
		if !ime.ghostEnabled || !ime.ghostVisible || !ime.ghostReady || len(ime.ghostCandidates) < 2 {
			resp.ReturnValue = 0
			return true
		}
		ime.cycleGhostCandidateLocked()
		ime.fillGhostResponseLocked(resp)
		resp.ReturnValue = 1
		return true
	}
	longMode := isLong
	before, following := decodeGhostSurroundingText(req.CloudClipboardText)
	normalizedBefore := normalizeCompletionContext(before, ime.ghostConfig.ContextTokens)
	normalizedFollowing := normalizeFollowingCompletionContext(following, 64)

	ime.ghostMu.Lock()
	if !ime.ghostEnabled {
		ime.ghostMu.Unlock()
		resp.ReturnValue = 0
		return true
	}
	surroundingMatches := normalizedBefore != "" && normalizedBefore == ime.ghostContext && normalizedFollowing == ime.ghostFollowingContext
	// F8 accepts the visible candidate regardless of whether F8 or Shift+F8
	// generated it. Shift only controls the requested completion length.
	if ime.ghostVisible && ime.ghostReady && surroundingMatches {
		accepted := ime.currentGhostCandidateLocked()
		acceptedContext := ime.ghostContext + accepted
		acceptedFollowing := ime.ghostFollowingContext
		acceptedLong := ime.ghostLong
		ime.performGhostActionLocked(resp)
		ime.ghostMu.Unlock()
		if accepted != "" {
			ime.scheduleGhostCompletionForSurrounding(acceptedContext, acceptedFollowing, false,
				time.Duration(ime.ghostConfig.IdleMS)*time.Millisecond, acceptedLong)
		}
		return true
	}
	modeMatches := surroundingMatches && longMode == ime.ghostLong
	if !ime.hasGhostWorkLocked() || (normalizedBefore != "" && !modeMatches) {
		hadVisible := ime.ghostVisible || ime.ghostLoadingVisible
		ime.ghostMu.Unlock()
		if normalizedBefore == "" {
			resp.ReturnValue = 0
			return true
		}
		// F8 is also an on-demand trigger. This makes completion available after
		// mouse caret moves and in text that was not entered through Moqi.
		ime.scheduleGhostCompletionForSurrounding(before, following, true, 0, longMode)
		if hadVisible {
			resp.HideMessage = true
		}
		resp.ReturnValue = 1
		return true
	}
	debugLogf("Ghost completion F8 preserved key received ready=%t visible=%t pending=%t", ime.ghostReady, ime.ghostVisible, ime.ghostPending)
	accepted := ""
	acceptedContext := ""
	acceptedFollowing := ""
	acceptedLong := ime.ghostLong
	if ime.ghostVisible && ime.ghostReady {
		accepted = ime.currentGhostCandidateLocked()
		acceptedContext = ime.ghostContext + accepted
		acceptedFollowing = ime.ghostFollowingContext
	}
	ime.performGhostActionLocked(resp)
	ime.ghostMu.Unlock()
	if accepted != "" {
		// Prime another completion after accepting one, so repeated F8 can keep
		// extending the sentence without requiring an intervening keystroke.
		ime.scheduleGhostCompletionForSurrounding(acceptedContext, acceptedFollowing, false,
			time.Duration(ime.ghostConfig.IdleMS)*time.Millisecond, acceptedLong)
	}
	return true
}

func (ime *IME) performGhostActionLocked(resp *imecore.Response) bool {
	if resp == nil || !ime.ghostEnabled || !ime.hasGhostWorkLocked() {
		return false
	}
	if ime.ghostHidePending {
		resp.HideMessage = true
		ime.ghostHidePending = false
	}
	if ime.ghostVisible && ime.ghostReady {
		candidate := ime.currentGhostCandidateLocked()
		candidateID := ime.currentGhostCandidateIDLocked()
		opportunityID := ime.ghostOpportunityID
		resp.CommitString = candidate
		resp.HideMessage = true
		if opportunityID != 0 && candidateID != 0 {
			ime.recordGhostEventLocked(ghosttelemetry.EventAccepted, ghosttelemetry.IDs{
				OpportunityID: opportunityID,
				CandidateID:   candidateID,
			}, ghosttelemetry.AcceptedPayload{AcceptedRunes: len([]rune(candidate)), AcceptActions: 1})
			ime.closeGhostOpportunityLocked("accepted", 0, 0, 0)
			ime.ghostPendingCommit = &ghostPendingCommit{opportunityID: opportunityID, candidateID: candidateID}
			ime.ghostLastAccepted = &ghostAcceptedState{
				opportunityID:  opportunityID,
				candidateID:    candidateID,
				acceptedAt:     time.Now(),
				remainingRunes: len([]rune(candidate)),
			}
		}
		ime.clearGhostStateLocked()
		resp.ReturnValue = 1
		return true
	}
	if ime.ghostReady {
		ime.ghostVisible = true
		ime.fillGhostResponseLocked(resp)
	} else {
		ime.requestGhostRevealLocked()
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleGhostKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil {
		return false
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if ime.ghostConsumeKeyUpCode == 0 {
		return false
	}
	if req.KeyCode != ime.ghostConsumeKeyUpCode {
		return false
	}
	ime.ghostConsumeKeyUpCode = 0
	if ime.ghostVisible {
		ime.fillGhostResponseLocked(resp)
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) scheduleGhostCompletion(req *imecore.Request) {
	if req == nil {
		return
	}
	before, following := decodeGhostSurroundingText(req.CloudClipboardText)
	ime.scheduleGhostCompletionWithContext(req, before, following)
}

func (ime *IME) scheduleGhostCompletionAfterCommit(req *imecore.Request, resp *imecore.Response, compositionBeforeCommit string) {
	if req == nil || resp == nil {
		return
	}
	if resp.CommitString != "" {
		// The Windows frontend deliberately sends an empty surrounding-text
		// payload for password/private fields. Treat unavailable context the same
		// way and collect no commit telemetry from that request.
		if req.CloudClipboardText != "" {
			ime.recordGhostTextCommit(resp.CommitString)
		} else {
			ime.resetGhostPhysicalKeyActions()
		}
	}
	if strings.TrimSpace(resp.CommitString) == "" {
		return
	}
	documentBefore, following := decodeGhostSurroundingText(req.CloudClipboardText)
	context := committedGhostContext(documentBefore, compositionBeforeCommit, resp.CommitString)
	debugLogf("Ghost commit trigger context_runes=%d preedit_runes=%d commit_runes=%d", len([]rune(context)), len([]rune(compositionBeforeCommit)), len([]rune(resp.CommitString)))
	ime.scheduleGhostCompletionForSurroundingWithTrigger(context, following, false,
		time.Duration(ime.ghostConfig.IdleMS)*time.Millisecond, false, "after_commit")
}

func (ime *IME) scheduleGhostCompletionWithContext(req *imecore.Request, rawContext, rawFollowing string) {
	if req == nil || !isGhostContextKey(req) {
		return
	}
	if ime.productSettings.AIRunMode == aiRunModeManual {
		return
	}
	ime.scheduleGhostCompletionForSurroundingWithTrigger(rawContext, rawFollowing, false,
		time.Duration(ime.ghostConfig.IdleMS)*time.Millisecond, false, "typing_idle")
}

func (ime *IME) scheduleGhostCompletionForSurrounding(rawContext, rawFollowing string, revealRequested bool, delay time.Duration, longMode bool) {
	trigger := "automatic"
	if revealRequested {
		trigger = "manual"
	}
	ime.scheduleGhostCompletionForSurroundingWithTrigger(rawContext, rawFollowing, revealRequested, delay, longMode, trigger)
}

func (ime *IME) scheduleGhostCompletionForSurroundingWithTrigger(rawContext, rawFollowing string, revealRequested bool, delay time.Duration, longMode bool, trigger string) {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if !ime.ghostEnabled || ime.ghostGenerator == nil {
		return
	}
	ime.resolveDeferredGhostOpportunityLocked(rawContext, "new_input")
	context := normalizeCompletionContext(rawContext, ime.ghostConfig.ContextTokens)
	following := normalizeFollowingCompletionContext(rawFollowing, 64)
	if context == "" {
		return
	}
	if context == ime.ghostContext && following == ime.ghostFollowingContext && longMode == ime.ghostLong && ime.hasGhostWorkLocked() {
		if revealRequested {
			ime.requestGhostRevealLocked()
		}
		return
	}
	if ime.backend == nil || !ime.backendReady() || ime.backend.State().Composition != "" {
		ime.resetGhostCompletionLockedWithReason("new_input")
		return
	}

	ime.resetGhostCompletionLockedWithReason("new_input")
	ime.openGhostOpportunityLocked(trigger)
	if hasUnsafeInlineBoundary(context, following) {
		ime.closeGhostOpportunityLocked("low_confidence", 0, 0, 0)
		return
	}
	ime.ghostContext = context
	ime.ghostFollowingContext = following
	ime.ghostLong = longMode
	ime.ghostRevealRequested = revealRequested
	ime.ghostRequestSeq++
	requestSeq := ime.ghostRequestSeq
	ime.ghostTimer = time.AfterFunc(delay, func() {
		ime.startGhostCompletion(requestSeq, context, following, longMode)
	})
	if revealRequested {
		ime.scheduleGhostLoadingMessageLocked()
	}
	debugLogf("Ghost completion scheduled seq=%d before_runes=%d after_runes=%d delay_ms=%d reveal=%t long=%t", requestSeq, len([]rune(context)), len([]rune(following)), delay.Milliseconds(), revealRequested, longMode)
}

func (ime *IME) requestGhostRevealLocked() {
	ime.ghostRevealRequested = true
	ime.scheduleGhostLoadingMessageLocked()
}

func (ime *IME) scheduleGhostLoadingMessageLocked() {
	if !ime.ghostRevealRequested || ime.ghostReady || ime.ghostLoadingVisible ||
		ime.ghostLoadingTimer != nil || (ime.ghostTimer == nil && !ime.ghostPending) {
		return
	}
	delay := ime.ghostLoadingDelay
	if delay <= 0 {
		delay = defaultGhostLoadingDelay
	}
	requestSeq := ime.ghostRequestSeq
	context := ime.ghostContext
	following := ime.ghostFollowingContext
	longMode := ime.ghostLong
	ime.ghostLoadingTimer = time.AfterFunc(delay, func() {
		ime.showGhostLoadingMessage(requestSeq, context, following, longMode)
	})
}

func (ime *IME) showGhostLoadingMessage(requestSeq uint64, context, following string, longMode bool) {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if !ime.ghostEnabled || requestSeq != ime.ghostRequestSeq ||
		context != ime.ghostContext || following != ime.ghostFollowingContext ||
		longMode != ime.ghostLong || !ime.ghostRevealRequested || ime.ghostReady ||
		(ime.ghostTimer == nil && !ime.ghostPending) {
		return
	}
	ime.ghostLoadingTimer = nil
	if ime.asyncResponseSender == nil {
		return
	}
	ime.ghostLoadingVisible = true
	resp := imecore.NewResponse(0, true)
	resp.ShowMessage = &imecore.MessageWindow{
		Message:  "本地 AI 正在准备…",
		Duration: ghostMessageDuration,
	}
	// Serialize the visibility transition with cancellation. If typing resets
	// this request next, its synchronous response will always hide this window.
	ime.asyncResponseSender(resp)
}

func (ime *IME) startGhostCompletion(requestSeq uint64, context, following string, longMode bool) {
	debugLogf("Ghost completion timer fired seq=%d", requestSeq)
	ime.ghostMu.Lock()
	if !ime.ghostEnabled || requestSeq != ime.ghostRequestSeq || context != ime.ghostContext || following != ime.ghostFollowingContext || longMode != ime.ghostLong || ime.ghostGenerator == nil {
		ime.ghostMu.Unlock()
		debugLogf("Ghost completion timer discarded seq=%d", requestSeq)
		return
	}
	// The request path invalidates this sequence as soon as the user resumes
	// typing. Do not call the Rime backend from this timer goroutine: librime is
	// session-thread-bound and its State/GetCommit read can be destructive.
	ime.ghostTimer = nil
	ime.ghostPending = true
	generator := ime.ghostGenerator
	cfg := ime.ghostConfig
	sender := ime.asyncResponseSender
	ctx, cancel := contextpkg.WithCancel(contextpkg.Background())
	request := &ghostRequestState{
		id:            ime.nextGhostRequestIDLocked(),
		opportunityID: ime.ghostOpportunityID,
		cancel:        cancel,
		started:       time.Now(),
		telemetry:     ime.ghostTelemetry,
	}
	ime.ghostRequest = request
	ime.recordGhostEventLocked(ghosttelemetry.EventRequestStarted, ghosttelemetry.IDs{
		OpportunityID: request.opportunityID,
		RequestID:     request.id,
	}, ghosttelemetry.RequestStartedPayload{Generator: "llm", RequestedCandidates: cfg.CandidateCount})
	ime.ghostMu.Unlock()

	started := request.started
	debugLogf("Ghost completion started seq=%d context_runes=%d", requestSeq, len([]rune(context)))
	candidates, err := generator(ctx, aiCompletionRequest{Context: context, FollowingContext: following, Long: longMode}, cfg)
	cancel()
	maxRunes := ghostShortMaxRunes
	if following != "" {
		maxRunes = 20
	} else if longMode {
		maxRunes = ghostLongMaxRunes
	}
	normalizedCandidates := normalizeInlineCompletionsWithOptions(context, following, candidates, cfg.CandidateCount, maxRunes)

	var updateResp *imecore.Response
	ime.ghostMu.Lock()
	elapsed := time.Since(started)
	candidateIDs := ime.recordGhostCandidatesReadyLocked(request, normalizedCandidates, elapsed)
	status := "completed"
	if errors.Is(err, contextpkg.Canceled) {
		status = "cancelled"
	} else if errors.Is(err, contextpkg.DeadlineExceeded) {
		status = "timeout"
	} else if err != nil {
		status = "error"
	}
	ime.recordGhostEventWithRecorderLocked(request.telemetry, ghosttelemetry.EventRequestFinished, ghosttelemetry.IDs{
		OpportunityID: request.opportunityID,
		RequestID:     request.id,
	}, ghosttelemetry.RequestFinishedPayload{
		Status:              status,
		FinishedAfterCancel: request.cancelRequested && status == "completed",
		ElapsedMS:           elapsed.Milliseconds(),
	})
	if ime.ghostRequest == request {
		ime.ghostRequest = nil
	}
	if request.closeTelemetryOnFinish && request.telemetry != nil {
		_ = request.telemetry.Close()
	}
	if requestSeq == ime.ghostRequestSeq && context == ime.ghostContext && following == ime.ghostFollowingContext && longMode == ime.ghostLong {
		ime.ghostPending = false
		if ime.ghostLoadingTimer != nil {
			ime.ghostLoadingTimer.Stop()
			ime.ghostLoadingTimer = nil
		}
		loadingWasVisible := ime.ghostLoadingVisible
		ime.ghostLoadingVisible = false
		if err == nil {
			ime.ghostCandidates = normalizedCandidates
			ime.ghostCandidateIDs = candidateIDs
			ime.ghostCandidateIndex = 0
			ime.ghostReady = len(ime.ghostCandidates) > 0
			if ime.ghostReady && ime.ghostRevealRequested {
				ime.ghostVisible = true
				updateResp = imecore.NewResponse(0, true)
				ime.fillGhostResponseLocked(updateResp)
			} else if loadingWasVisible {
				updateResp = imecore.NewResponse(0, true)
				updateResp.HideMessage = true
			}
			if !ime.ghostReady {
				ime.ghostRevealRequested = false
				ime.closeGhostOpportunityLocked("no_candidate", 0, 0, 0)
			}
		} else {
			if ime.ghostRevealRequested {
				updateResp = imecore.NewResponse(0, true)
				updateResp.ShowMessage = &imecore.MessageWindow{
					Message:  friendlyGhostError(err),
					Duration: 3,
				}
			}
			reason := "request_error"
			if status == "timeout" {
				reason = "request_timeout"
			}
			ime.resetGhostCompletionLockedWithReason(reason)
		}
	}
	ime.ghostMu.Unlock()
	debugLogf("Ghost completion finished seq=%d elapsed=%s candidates=%d err=%v", requestSeq, elapsed, len(candidates), err)
	if updateResp != nil && sender != nil {
		sender(updateResp)
	}
}

func friendlyGhostError(err error) string {
	if err == nil {
		return "本地 AI 暂不可用"
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "Model file was not found"),
		strings.Contains(message, "模型文件"):
		return "本地 AI 暂不可用：未找到模型文件"
	case strings.Contains(message, "llama-server.exe was not found"):
		return "本地 AI 暂不可用：未安装模型运行程序"
	case strings.Contains(message, "timed out"),
		strings.Contains(message, "deadline exceeded"):
		return "本地 AI 启动超时，请稍后重试"
	default:
		return "本地 AI 暂不可用"
	}
}

func committedGhostContext(documentBefore, compositionBeforeCommit, commit string) string {
	documentBefore = strings.TrimSuffix(documentBefore, compositionBeforeCommit)
	if commit == "" || strings.HasSuffix(documentBefore, commit) {
		return documentBefore
	}
	return documentBefore + commit
}

func decodeGhostSurroundingText(payload string) (before, following string) {
	if !strings.HasPrefix(payload, ghostContextEnvelope) {
		return payload, ""
	}
	remainder := strings.TrimPrefix(payload, ghostContextEnvelope)
	before, following, _ = strings.Cut(remainder, "\x1f")
	return before, following
}

func (ime *IME) fillGhostResponseLocked(resp *imecore.Response) {
	if resp == nil || !ime.ghostVisible || !ime.ghostReady {
		return
	}
	if candidate := ime.currentGhostCandidateLocked(); candidate != "" {
		if !ime.ghostShown {
			ime.ghostShown = true
			ime.recordGhostEventLocked(ghosttelemetry.EventShown, ghosttelemetry.IDs{OpportunityID: ime.ghostOpportunityID},
				ghosttelemetry.ShownPayload{CandidateIDs: append([]uint64(nil), ime.ghostCandidateIDs...)})
		}
		resp.ShowMessage = &imecore.MessageWindow{Message: candidate, Duration: ghostMessageDuration}
	}
}

func (ime *IME) currentGhostCandidateLocked() string {
	if len(ime.ghostCandidates) == 0 {
		return ""
	}
	if ime.ghostCandidateIndex < 0 || ime.ghostCandidateIndex >= len(ime.ghostCandidates) {
		ime.ghostCandidateIndex = 0
	}
	return ime.ghostCandidates[ime.ghostCandidateIndex]
}

func (ime *IME) currentGhostCandidateIDLocked() uint64 {
	if ime.ghostCandidateIndex < 0 || ime.ghostCandidateIndex >= len(ime.ghostCandidateIDs) {
		return 0
	}
	return ime.ghostCandidateIDs[ime.ghostCandidateIndex]
}

func (ime *IME) cycleGhostCandidateLocked() {
	if len(ime.ghostCandidates) < 2 {
		return
	}
	fromID := ime.currentGhostCandidateIDLocked()
	ime.ghostCandidateIndex = (ime.ghostCandidateIndex + 1) % len(ime.ghostCandidates)
	toID := ime.currentGhostCandidateIDLocked()
	if fromID != 0 && toID != 0 {
		ime.recordGhostEventLocked(ghosttelemetry.EventCycled, ghosttelemetry.IDs{OpportunityID: ime.ghostOpportunityID},
			ghosttelemetry.CycledPayload{FromCandidateID: fromID, ToCandidateID: toID})
	}
}

func (ime *IME) hasGhostWork() bool {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	return ime.hasGhostWorkLocked()
}

func (ime *IME) hasGhostWorkLocked() bool {
	return ime.ghostTimer != nil || ime.ghostLoadingTimer != nil || ime.ghostPending ||
		ime.ghostReady || ime.ghostVisible || ime.ghostLoadingVisible
}

func (ime *IME) resetGhostCompletion() {
	ime.resetGhostCompletionWithReason("dismissed")
}

func (ime *IME) resetGhostCompletionWithReason(reason string) {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	ime.resetGhostCompletionLockedWithReason(reason)
}

func (ime *IME) resetGhostCompletionLocked() {
	ime.resetGhostCompletionLockedWithReason("dismissed")
}

func (ime *IME) resetGhostCompletionLockedWithReason(reason string) {
	ime.cancelGhostRequestLocked(reason)
	ime.closeGhostOpportunityLocked(reason, 0, 0, 0)
	ime.closeDeferredGhostOpportunityLocked(reason, "")
	ime.clearGhostStateLocked()
}

func (ime *IME) clearGhostStateLocked() {
	ime.ghostRequestSeq++
	if ime.ghostTimer != nil {
		ime.ghostTimer.Stop()
		ime.ghostTimer = nil
	}
	if ime.ghostLoadingTimer != nil {
		ime.ghostLoadingTimer.Stop()
		ime.ghostLoadingTimer = nil
	}
	ime.ghostLoadingVisible = false
	ime.ghostPending = false
	ime.ghostReady = false
	ime.ghostVisible = false
	ime.ghostRevealRequested = false
	ime.ghostCandidates = nil
	ime.ghostCandidateIDs = nil
	ime.ghostCandidateIndex = 0
	ime.ghostShown = false
	ime.ghostConsumeKeyUpCode = 0
	ime.ghostContext = ""
	ime.ghostFollowingContext = ""
	ime.ghostLong = false
	ime.ghostHidePending = false
	ime.ghostOpportunityID = 0
	ime.ghostOpportunityOpen = false
}

func (ime *IME) recordGhostEventLocked(event string, ids ghosttelemetry.IDs, payload any) {
	ime.recordGhostEventWithRecorderLocked(ime.ghostTelemetry, event, ids, payload)
}

func (ime *IME) recordGhostEventWithRecorderLocked(recorder *ghosttelemetry.Recorder, event string, ids ghosttelemetry.IDs, payload any) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(event, ids, payload); err != nil {
		debugLogf("AI completion telemetry write failed: %v", err)
	}
}

func (ime *IME) nextGhostRequestIDLocked() uint64 {
	if ime.ghostTelemetry == nil {
		return 0
	}
	return ime.ghostTelemetry.NextRequestID()
}

func (ime *IME) openGhostOpportunityLocked(trigger string) {
	if ime.ghostTelemetry == nil {
		return
	}
	ime.ghostOpportunityID = ime.ghostTelemetry.NextOpportunityID()
	ime.ghostOpportunityOpen = true
	ime.recordGhostEventLocked(ghosttelemetry.EventOpportunityCreated, ghosttelemetry.IDs{OpportunityID: ime.ghostOpportunityID},
		ghosttelemetry.OpportunityCreatedPayload{
			Mode:               string(ime.productSettings.AIRunMode),
			CompositionVersion: ime.ghostCompositionVersion,
			Trigger:            trigger,
		})
}

func (ime *IME) closeGhostOpportunityLocked(reason string, matchedCandidateID uint64, matchedPrefixRunes, manualRunes int) {
	if !ime.ghostOpportunityOpen || ime.ghostOpportunityID == 0 {
		return
	}
	ime.recordGhostEventLocked(ghosttelemetry.EventOpportunityClosed, ghosttelemetry.IDs{OpportunityID: ime.ghostOpportunityID},
		ghosttelemetry.OpportunityClosedPayload{
			Reason:             reason,
			MatchedCandidateID: matchedCandidateID,
			MatchedPrefixRunes: matchedPrefixRunes,
			ManualRunes:        manualRunes,
		})
	ime.ghostOpportunityOpen = false
}

func (ime *IME) cancelGhostRequestLocked(reason string) {
	request := ime.ghostRequest
	if request == nil || request.cancelRequested {
		return
	}
	request.cancelRequested = true
	ime.recordGhostEventLocked(ghosttelemetry.EventRequestCancelRequested, ghosttelemetry.IDs{
		OpportunityID: request.opportunityID,
		RequestID:     request.id,
	}, ghosttelemetry.RequestCancelRequestedPayload{Reason: reason})
	request.cancel()
}

func (ime *IME) recordGhostCandidatesReadyLocked(request *ghostRequestState, candidates []string, elapsed time.Duration) []uint64 {
	if request.telemetry == nil || len(candidates) == 0 {
		return nil
	}
	features := make([]ghosttelemetry.CandidateFeature, 0, len(candidates))
	ids := make([]uint64, 0, len(candidates))
	for index, candidate := range candidates {
		candidateID := request.telemetry.NextCandidateID()
		ids = append(ids, candidateID)
		features = append(features, ghosttelemetry.CandidateFeature{
			CandidateID:    candidateID,
			FeatureVersion: ghostFeatureVersion,
			Rank:           index + 1,
			Source:         "llm",
			Runes:          len([]rune(candidate)),
			NaturalEnd:     completionContextEndsSentence(candidate),
		})
	}
	ime.recordGhostEventWithRecorderLocked(request.telemetry, ghosttelemetry.EventCandidateReady, ghosttelemetry.IDs{
		OpportunityID: request.opportunityID,
		RequestID:     request.id,
	}, ghosttelemetry.CandidateReadyPayload{Candidates: features, GenerationMS: elapsed.Milliseconds()})
	return ids
}

func (ime *IME) recordGhostInputAdvancedLocked() {
	if !ime.ghostOpportunityOpen || ime.ghostOpportunityID == 0 {
		return
	}
	ime.ghostCompositionVersion++
	ime.recordGhostEventLocked(ghosttelemetry.EventInputAdvanced, ghosttelemetry.IDs{OpportunityID: ime.ghostOpportunityID},
		ghosttelemetry.InputAdvancedPayload{CompositionVersion: ime.ghostCompositionVersion})
}

func (ime *IME) deferOrCloseGhostOpportunityLocked(reason string) {
	if !ime.ghostOpportunityOpen {
		return
	}
	if !ime.ghostShown || len(ime.ghostCandidates) == 0 {
		ime.closeGhostOpportunityLocked(reason, 0, 0, 0)
		return
	}
	if ime.ghostDeferredClose != nil {
		ime.closeDeferredGhostOpportunityLocked(ime.ghostDeferredClose.reason, "")
	}
	ime.ghostDeferredClose = &ghostDeferredClose{
		opportunityID: ime.ghostOpportunityID,
		reason:        reason,
		context:       ime.ghostContext,
		candidates:    append([]string(nil), ime.ghostCandidates...),
		candidateIDs:  append([]uint64(nil), ime.ghostCandidateIDs...),
	}
	ime.ghostOpportunityOpen = false
}

func (ime *IME) resolveDeferredGhostOpportunityLocked(rawContext, reason string) {
	deferred := ime.ghostDeferredClose
	if deferred == nil || rawContext == "" || rawContext == deferred.context {
		return
	}
	if strings.HasPrefix(rawContext, deferred.context) {
		ime.closeDeferredGhostOpportunityLocked(reason, strings.TrimPrefix(rawContext, deferred.context))
	}
}

func (ime *IME) closeDeferredGhostOpportunityLocked(reason, manual string) {
	deferred := ime.ghostDeferredClose
	if deferred == nil {
		return
	}
	matchedID, matchedRunes := bestGhostPrefixMatch(manual, deferred.candidates, deferred.candidateIDs)
	if reason == "" {
		reason = deferred.reason
	}
	ime.recordGhostEventLocked(ghosttelemetry.EventOpportunityClosed, ghosttelemetry.IDs{OpportunityID: deferred.opportunityID},
		ghosttelemetry.OpportunityClosedPayload{
			Reason:             reason,
			MatchedCandidateID: matchedID,
			MatchedPrefixRunes: matchedRunes,
			ManualRunes:        len([]rune(manual)),
		})
	ime.ghostDeferredClose = nil
}

func bestGhostPrefixMatch(manual string, candidates []string, candidateIDs []uint64) (uint64, int) {
	manualRunes := []rune(manual)
	bestIndex, bestRunes := -1, 0
	for index, candidate := range candidates {
		candidateRunes := []rune(candidate)
		matched := 0
		for matched < len(manualRunes) && matched < len(candidateRunes) && manualRunes[matched] == candidateRunes[matched] {
			matched++
		}
		if matched > bestRunes {
			bestIndex, bestRunes = index, matched
		}
	}
	if bestIndex < 0 || bestIndex >= len(candidateIDs) {
		return 0, bestRunes
	}
	return candidateIDs[bestIndex], bestRunes
}

func (ime *IME) recordGhostPhysicalKeyAction(req *imecore.Request) {
	if req == nil || isModifierKey(req.KeyCode) {
		return
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if ime.ghostTelemetry == nil {
		return
	}
	ime.ghostPhysicalKeyActions++
	accepted := ime.ghostLastAccepted
	if accepted == nil {
		return
	}
	elapsed := time.Since(accepted.acceptedAt)
	if elapsed > ghostQuickUndoWindow || req.KeyCode != vkBack {
		ime.ghostLastAccepted = nil
		return
	}
	if !accepted.undoRecorded && accepted.remainingRunes > 0 {
		accepted.undoRecorded = true
		accepted.remainingRunes--
		ime.recordGhostEventLocked(ghosttelemetry.EventQuickUndo, ghosttelemetry.IDs{
			OpportunityID: accepted.opportunityID,
			CandidateID:   accepted.candidateID,
		}, ghosttelemetry.QuickUndoPayload{RemovedRunes: 1, ElapsedMS: elapsed.Milliseconds()})
	}
}

func (ime *IME) recordGhostTextCommit(text string) {
	if text == "" {
		return
	}
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	if ime.ghostTelemetry == nil {
		return
	}
	if ime.ghostDeferredClose != nil {
		ime.closeDeferredGhostOpportunityLocked(ime.ghostDeferredClose.reason, text)
	}
	ids := ghosttelemetry.IDs{}
	source := "manual"
	if pending := ime.ghostPendingCommit; pending != nil {
		source = "ai"
		ids.OpportunityID = pending.opportunityID
		ids.CandidateID = pending.candidateID
		ime.ghostPendingCommit = nil
	}
	ime.recordGhostEventLocked(ghosttelemetry.EventTextCommitted, ids, ghosttelemetry.TextCommittedPayload{
		Source:                                source,
		OutputRunes:                           len([]rune(text)),
		PhysicalKeyActionsSincePreviousCommit: ime.ghostPhysicalKeyActions,
	})
	ime.ghostPhysicalKeyActions = 0
}

func (ime *IME) resetGhostPhysicalKeyActions() {
	ime.ghostMu.Lock()
	defer ime.ghostMu.Unlock()
	ime.ghostPhysicalKeyActions = 0
}

func isModifierKey(keyCode int) bool {
	switch keyCode {
	case vkShift, vkControl, vkMenu, vkLShift, vkRShift, vkLControl, vkRControl, vkLMenu, vkRMenu:
		return true
	default:
		return false
	}
}

func isGhostCursorMovement(keyCode int) bool {
	return isCaretMovementKey(keyCode)
}

func isLeftAlt(req *imecore.Request) bool {
	if req == nil || req.KeyCode != vkMenu || req.IsExtended {
		return false
	}
	return !req.KeyStates.IsKeyDown(vkControl)
}

func shouldInvalidateGhostForKey(req *imecore.Request) bool {
	if req == nil || req.KeyCode == ghostActionKeyCode || isLeftAlt(req) {
		return false
	}
	return req.CharCode >= 0x20 || req.KeyCode == vkBack || req.KeyCode == vkDelete || req.KeyCode == vkReturn || req.KeyCode == vkSpace || req.KeyCode == vkEscape || isCaretMovementKey(req.KeyCode)
}

func isGhostContextKey(req *imecore.Request) bool {
	if req == nil || req.KeyCode == ghostActionKeyCode || isLeftAlt(req) {
		return false
	}
	if req.KeyStates.IsKeyDown(vkControl) || req.KeyStates.IsKeyDown(vkMenu) {
		return false
	}
	// Arrow/navigation key context is captured before the host moves the caret.
	// It must invalidate an old completion but must not schedule from that stale
	// snapshot. F8/Shift+F8 always fetch fresh context after the move.
	return req.CharCode >= 0x20 || req.KeyCode == vkReturn || req.KeyCode == vkSpace
}

func isCaretMovementKey(keyCode int) bool {
	switch keyCode {
	case vkLeft, vkRight, vkUp, vkDown, vkHome, vkEnd, vkPrior, vkNext:
		return true
	default:
		return false
	}
}

func normalizeCompletionContext(context string, tokenBudget int) string {
	context = strings.ReplaceAll(context, "\x00", "")
	context = strings.TrimSpace(context)
	if context == "" {
		return ""
	}
	runes := []rune(context)
	start := tailSentenceStart(runes, ghostContextMaxSentences)
	runes = runes[start:]
	if tokenBudget <= 0 {
		tokenBudget = 128
	}
	// Four units approximate one token: CJK consumes a token, while common
	// ASCII text averages about four characters per token.
	budgetUnits := tokenBudget * 4
	used := 0
	start = len(runes)
	for start > 0 {
		cost := completionRuneCost(runes[start-1])
		if used+cost > budgetUnits {
			break
		}
		used += cost
		start--
	}
	return strings.TrimSpace(string(runes[start:]))
}

func normalizeFollowingCompletionContext(context string, tokenBudget int) string {
	context = strings.TrimSpace(strings.ReplaceAll(context, "\x00", ""))
	if context == "" {
		return ""
	}
	if tokenBudget <= 0 {
		tokenBudget = 64
	}
	runes := []rune(context)
	used := 0
	end := 0
	sentences := 0
	for end < len(runes) {
		cost := completionRuneCost(runes[end])
		if used+cost > tokenBudget*4 {
			break
		}
		used += cost
		if isSentenceTerminator(runes[end]) {
			sentences++
			if sentences == 2 {
				end++
				break
			}
		}
		end++
	}
	return strings.TrimSpace(string(runes[:end]))
}

func tailSentenceStart(runes []rune, maxSentences int) int {
	if maxSentences <= 0 {
		return 0
	}
	boundaries := 0
	for i := len(runes) - 1; i >= 0; i-- {
		if !isSentenceTerminator(runes[i]) {
			continue
		}
		if i == len(runes)-1 {
			continue
		}
		boundaries++
		if boundaries == maxSentences {
			return i + 1
		}
	}
	return 0
}

func completionRuneCost(r rune) int {
	if r <= unicode.MaxASCII {
		return 1
	}
	return 4
}

func normalizeInlineCompletions(context string, candidates []string, limit int) []string {
	return normalizeInlineCompletionsWithOptions(context, "", candidates, limit, 40)
}

func normalizeInlineCompletionsWithOptions(context, following string, candidates []string, limit, maxRunes int) []string {
	if limit <= 0 {
		limit = 3
	}
	context = strings.TrimSpace(context)
	following = strings.TrimSpace(following)
	result := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate)
		value = strings.TrimLeft(value, "-*0123456789.、)） \t")
		value = strings.TrimSpace(strings.Trim(value, `"'`))
		for _, prefix := range []string{"续写：", "续写:", "候选：", "候选:"} {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
		if looksLikeAssistantReply(value) {
			continue
		}
		if strings.HasPrefix(following, "的") && (looksLikeQuantityPhrase(value) || strings.HasSuffix(value, "产的") || strings.HasSuffix(value, "产地的")) {
			continue
		}
		if value == "（无）" || value == "(无)" || value == "无" {
			continue
		}
		if context != "" && strings.HasPrefix(value, context) {
			value = strings.TrimSpace(strings.TrimPrefix(value, context))
		}
		value = trimLeadingContextOverlap(context, value)
		value = trimEmbeddedFollowingOverlap(value, following)
		value = trimTrailingFollowingOverlap(value, following)
		value = trimRepeatedFollowingConnector(value, following)
		value = collapsePathologicalRepetition(value)
		if following == "" && maxRunes <= 20 {
			value = trimRepeatedShortClauseTopic(context, value)
			value = ensureShortCompletionSeparator(context, value)
		}
		longCompletion := following == "" && maxRunes == ghostLongMaxRunes
		if longCompletion {
			value = normalizeLongCompletionBoundary(value, maxRunes)
		} else {
			value = truncateCompletionAtSentence(value, maxRunes)
		}
		if len([]rune(value)) >= 4 && strings.Contains(context, value) {
			continue
		}
		if longCompletion && looksLikeContextRestatement(context, value) {
			continue
		}
		if following == "" && maxRunes <= 20 && looksLikeGenericCompletion(value) {
			continue
		}
		if longCompletion {
			value = ensureLongCompletionEnding(value, following, maxRunes)
		}
		if value == "" || strings.Trim(value, "。！？!?；;，,、 \t") == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		if len(result) == limit {
			break
		}
	}
	return result
}

func ensureShortCompletionSeparator(context, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	value = trimInvalidSeparatorBeforeAttachedParticle(value)
	if isAttachedShortContinuation(value) {
		return value
	}
	first := []rune(value)[0]
	switch first {
	case '，', ',', '。', '！', '!', '？', '?', '；', ';', '：', ':', '、':
		return value
	}
	tail := strings.TrimSpace(string(lastClauseRunes(context)))
	for _, greeting := range []string{"你好", "您好", "大家好", "早上好", "上午好", "下午好", "晚上好", "晚安", "谢谢", "多谢", "抱歉", "对不起", "没关系", "再见", "辛苦了"} {
		if tail == greeting {
			return finishSeparatedShortClause("，" + value)
		}
	}
	if hasCompletedShortPredicate(tail) {
		plainValue := strings.Trim(value, "。！？!?；;，,、 ")
		if len([]rune(plainValue)) >= 4 {
			for _, r := range value {
				if isSentenceTerminator(r) {
					return finishSeparatedShortClause("，" + value)
				}
			}
		}
		for _, prefix := range []string{"令人", "让人", "整体", "同时", "而且", "并且", "不过", "但是", "但也", "此外", "另外", "因此", "所以", "非常", "十分", "格外", "特别", "确实", "口感", "香气", "风味", "酸度", "甜感", "余韵", "回甘", "层次"} {
			if strings.HasPrefix(value, prefix) {
				return finishSeparatedShortClause("，" + value)
			}
		}
	}
	if likelyCompleteShortClause(tail) && looksLikeIndependentShortClause(value) {
		return finishSeparatedShortClause("，" + value)
	}
	return value
}

func trimInvalidSeparatorBeforeAttachedParticle(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) < 2 {
		return string(runes)
	}
	switch runes[0] {
	case '，', ',', '；', ';', '：', ':', '、':
		withoutSeparator := strings.TrimSpace(string(runes[1:]))
		if isAttachedShortContinuation(withoutSeparator) {
			return withoutSeparator
		}
	}
	return string(runes)
}

func isAttachedShortContinuation(value string) bool {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"吗", "呢", "吧", "啊", "呀", "嘛", "么", "呗", "啦", "哦", "哟", "的", "地", "得", "了", "着", "过"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func looksLikeIndependentShortClause(value string) bool {
	value = strings.TrimLeft(strings.TrimSpace(value), "，,；;：:、 ")
	for _, prefix := range []string{
		"我是", "我有", "我会", "我想", "我觉得", "我认为", "我准备", "我打算", "我希望", "我需要", "我们",
		"你是", "你有", "你会", "你想", "你觉得", "你可以", "你们",
		"他是", "他有", "他会", "她是", "她有", "她会", "它是", "它有", "它会", "他们", "她们",
		"这是", "那是", "今天", "明天", "现在", "接下来", "随后", "然后",
		"令人", "让人", "整体", "同时", "而且", "并且", "不过", "但是", "但也", "此外", "另外", "因此", "所以",
		"口感", "香气", "风味", "酸度", "甜感", "余韵", "回甘", "层次",
	} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func likelyCompleteShortClause(value string) bool {
	value = strings.Trim(value, "。！？!?；;，,、：: ")
	if len([]rune(value)) < 3 {
		return false
	}
	for _, suffix := range []string{
		"的", "地", "得", "和", "与", "或", "及", "以及", "把", "被", "向", "给", "从", "对", "为", "在",
		"是", "有", "想", "要", "会", "能", "可以", "应该", "需要", "准备", "打算", "计划", "希望", "觉得", "认为", "发现", "看到", "听到",
		"一个", "一种", "一杯", "一份", "一位", "一名", "这个", "那个", "因为", "如果", "虽然", "但是", "但", "而",
	} {
		if strings.HasSuffix(value, suffix) {
			return false
		}
	}
	for _, predicate := range []string{
		"是", "有", "在", "叫", "姓", "觉得", "认为", "喜欢", "希望", "需要", "想", "会", "能", "可以", "应该",
		"尝试", "喝", "吃", "用", "做", "去", "来", "看", "听", "说", "写", "发", "给", "让", "使", "带", "呈现", "表现", "包含", "具有", "显得", "变得",
	} {
		if strings.Contains(value, predicate) {
			return true
		}
	}
	return hasCompletedShortPredicate(value)
}

func trimRepeatedShortClauseTopic(context, value string) string {
	tail := strings.TrimSpace(string(lastClauseRunes(context)))
	for _, topic := range []string{"口感", "香气", "风味", "酸度", "甜感", "余韵", "回甘", "层次"} {
		if strings.Contains(tail, topic) && strings.HasPrefix(value, topic) {
			if rest := strings.TrimSpace(strings.TrimPrefix(value, topic)); rest != "" {
				return rest
			}
		}
	}
	return value
}

func hasCompletedShortPredicate(value string) bool {
	plain := strings.Trim(value, "。！？!?；;，,、 ")
	for _, ending := range []string{"和谐", "浓郁", "丰富", "清晰", "明亮", "清爽", "顺滑", "干净", "独特", "自然", "流畅", "稳定", "成熟", "完整", "明显", "突出", "开心", "高兴", "满意", "惊喜", "舒服", "漂亮", "方便", "简单", "困难", "重要", "合适", "回甘", "柔和", "饱满", "持久", "悠长", "平衡", "扎实", "细腻", "醇厚", "清甜", "鲜明", "舒适", "愉悦"} {
		if strings.HasSuffix(plain, ending) {
			return true
		}
	}
	return false
}

func finishSeparatedShortClause(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 || isSentenceTerminator(runes[len(runes)-1]) {
		return string(runes)
	}
	plain := strings.TrimLeft(string(runes), "，,；;：:、 ")
	if !hasCompletedShortPredicate(plain) && !likelyCompleteShortClause(plain) {
		return string(runes)
	}
	return string(runes) + "。"
}

func normalizeLongCompletionBoundary(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	for i, r := range runes {
		if !isSentenceTerminator(r) {
			continue
		}
		if maxRunes > 0 && i+1 > maxRunes {
			return ""
		}
		return strings.TrimSpace(string(runes[:i+1]))
	}
	// Never manufacture a full stop after cutting through a sentence. A short
	// punctuation-less line may merely have omitted its final full stop, but an
	// over-limit line is an incomplete model response and should not be shown.
	if maxRunes > 0 && len(runes) > maxRunes {
		return ""
	}
	return string(runes)
}

func looksLikeQuantityPhrase(value string) bool {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"一杯", "两杯", "三杯", "半杯", "一壶", "两壶", "一款", "一种", "一份", "一袋", "一盒", "一瓶"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func trimEmbeddedFollowingOverlap(value, following string) string {
	if value == "" || following == "" {
		return value
	}
	for size := len([]rune(following)); size >= 3; size-- {
		prefix := string([]rune(following)[:size])
		if index := strings.Index(value, prefix); index >= 0 {
			return strings.TrimSpace(value[:index])
		}
	}
	return value
}

func trimRepeatedFollowingConnector(value, following string) string {
	valueRunes, followingRunes := []rune(strings.TrimSpace(value)), []rune(strings.TrimSpace(following))
	if len(valueRunes) == 0 || len(followingRunes) == 0 || valueRunes[len(valueRunes)-1] != followingRunes[0] {
		return value
	}
	switch followingRunes[0] {
	case '的', '地', '得', '了', '着', '过', '和', '与', '或', '，', ',', '、', '：', ':':
		return strings.TrimSpace(string(valueRunes[:len(valueRunes)-1]))
	default:
		return value
	}
}

func looksLikeGenericCompletion(value string) bool {
	plain := strings.Trim(value, "。！？!?；;，,、 ")
	for _, phrase := range []string{"回味无穷", "令人难忘", "非常不错", "很有意思", "值得一试", "值得推荐", "推荐入手", "下次再来", "口感不错", "风味不错", "独具魅力", "韵味十足", "别有一番风味", "让人印象深刻", "完美诠释"} {
		if strings.Contains(plain, phrase) {
			return true
		}
	}
	return false
}

func looksLikeContextRestatement(context, value string) bool {
	left := lastClauseRunes(context)
	right := firstClauseRunes(value)
	if len(left) < 4 || len(right) < len(left) {
		return false
	}
	return longestCommonSubsequenceLength(left, right)*100 >= len(left)*75
}

func lastClauseRunes(value string) []rune {
	runes := []rune(strings.TrimSpace(value))
	start := 0
	for i, r := range runes {
		if isSentenceTerminator(r) || r == '，' || r == ',' || r == '：' || r == ':' {
			start = i + 1
		}
	}
	return []rune(strings.TrimSpace(string(runes[start:])))
}

func firstClauseRunes(value string) []rune {
	runes := []rune(strings.TrimSpace(value))
	for i, r := range runes {
		if isSentenceTerminator(r) || r == '，' || r == ',' || r == '：' || r == ':' {
			return []rune(strings.TrimSpace(string(runes[:i])))
		}
	}
	return runes
}

func longestCommonSubsequenceLength(a, b []rune) int {
	previous := make([]int, len(b)+1)
	for _, left := range a {
		current := make([]int, len(b)+1)
		for j, right := range b {
			if left == right {
				current[j+1] = previous[j] + 1
			} else if current[j] > previous[j+1] {
				current[j+1] = current[j]
			} else {
				current[j+1] = previous[j+1]
			}
		}
		previous = current
	}
	return previous[len(b)]
}

func ensureLongCompletionEnding(value, following string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 || isSentenceTerminator(runes[len(runes)-1]) {
		return string(runes)
	}
	after := []rune(strings.TrimSpace(following))
	if len(after) > 0 && isSentenceTerminator(after[0]) {
		return string(runes)
	}
	if maxRunes > 1 && len(runes) >= maxRunes {
		runes = runes[:maxRunes-1]
	}
	return string(runes) + "。"
}

func hasUnsafeInlineBoundary(context, following string) bool {
	left := []rune(strings.TrimSpace(context))
	right := []rune(strings.TrimSpace(following))
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	a, b := left[len(left)-1], right[0]
	// A caret inside or immediately beside an ASCII word/model/product token
	// is usually an editing position, not a natural insertion boundary. This
	// prevents cases such as “肯尼亚|AA” from receiving a Chinese phrase.
	return isASCIIWordRune(a) || isASCIIWordRune(b)
}

func isASCIIWordRune(r rune) bool {
	return r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
}

func trimLeadingContextOverlap(context, value string) string {
	left := []rune(context)
	for {
		right := []rune(value)
		max := len(left)
		if len(right) < max {
			max = len(right)
		}
		removed := false
		for size := max; size >= 2; size-- {
			if string(left[len(left)-size:]) == string(right[:size]) {
				value = strings.TrimSpace(string(right[size:]))
				removed = true
				break
			}
		}
		if !removed || value == "" {
			return value
		}
	}
}

func trimTrailingFollowingOverlap(value, following string) string {
	left, right := []rune(value), []rune(following)
	max := len(left)
	if len(right) < max {
		max = len(right)
	}
	for size := max; size >= 2; size-- {
		if string(left[len(left)-size:]) == string(right[:size]) {
			return strings.TrimSpace(string(left[:len(left)-size]))
		}
	}
	return value
}

func collapsePathologicalRepetition(value string) string {
	runes := []rune(value)
	if len(runes) < 4 {
		return value
	}
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); {
		collapsed := false
		maxUnit := 8
		if remaining := (len(runes) - i) / 2; remaining < maxUnit {
			maxUnit = remaining
		}
		for unit := 2; unit <= maxUnit; unit++ {
			count := 1
			for i+(count+1)*unit <= len(runes) && string(runes[i:i+unit]) == string(runes[i+count*unit:i+(count+1)*unit]) {
				count++
			}
			if count >= 2 {
				out = append(out, runes[i:i+unit]...)
				i += count * unit
				collapsed = true
				break
			}
		}
		if collapsed {
			continue
		}
		count := 1
		for i+count < len(runes) && runes[i+count] == runes[i] {
			count++
		}
		if count >= 5 {
			out = append(out, runes[i], runes[i])
			i += count
			continue
		}
		out = append(out, runes[i])
		i++
	}
	return string(out)
}

func looksLikeAssistantReply(value string) bool {
	lower := strings.ToLower(value)
	for _, phrase := range []string{"我是你的智能助手", "我是您的智能助手", "我是你的助手", "我是您的助手", "有什么可以帮", "无法确定", "无法生成", "需要更多上下文", "请提供完整", "请补充", "how can i help", "as an ai"} {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func truncateCompletionAtSentence(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	end := len(runes)
	for i, r := range runes {
		if isSentenceTerminator(r) {
			end = i + 1
			break
		}
	}
	if maxRunes > 0 && end > maxRunes {
		end = maxRunes
	}
	value = strings.TrimSpace(string(runes[:end]))
	if !utf8.ValidString(value) {
		return ""
	}
	return value
}

func isSentenceTerminator(r rune) bool {
	switch r {
	case '。', '！', '？', '!', '?', '；', ';', '\n', '\r':
		return true
	default:
		return false
	}
}
