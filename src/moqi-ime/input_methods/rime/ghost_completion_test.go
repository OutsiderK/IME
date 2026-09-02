package rime

import (
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

func TestGhostCompletionF8ShowsThenAccepts(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostCandidates = []string{"苹果。", "橘子。"}

	f8 := &imecore.Request{KeyCode: vkF8, KeyStates: make(imecore.KeyStates, 256)}
	filterResp := imecore.NewResponse(1, true)
	if !ime.handleGhostKeyDownFilter(f8, filterResp) || filterResp.ReturnValue != 1 {
		t.Fatal("expected first F8 to be consumed")
	}
	showResp := imecore.NewResponse(2, true)
	if !ime.handleGhostKeyDown(f8, showResp) {
		t.Fatal("expected first F8 to show completion")
	}
	if showResp.ShowMessage == nil || showResp.ShowMessage.Message != "苹果。" || showResp.ShowMessage.Duration != ghostMessageDuration {
		t.Fatalf("unexpected ghost response: %#v", showResp.ShowMessage)
	}
	if !ime.ghostVisible {
		t.Fatal("expected ghost to be visible after first F8")
	}

	acceptResp := imecore.NewResponse(3, true)
	if !ime.handleGhostKeyDown(f8, acceptResp) {
		t.Fatal("expected second F8 to accept completion")
	}
	if acceptResp.CommitString != "苹果。" || !acceptResp.HideMessage {
		t.Fatalf("unexpected accept response: %#v", acceptResp)
	}
	if ime.hasGhostWork() {
		t.Fatal("expected ghost state to reset after acceptance")
	}
}

func TestGhostCompletionRegistersAndHandlesF8AsPreservedKey(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostCandidates = []string{"苹果。", "橘子。"}

	activate := ime.HandleRequest(&imecore.Request{Method: "onActivate", SeqNum: 1})
	if len(activate.AddPreservedKey) != 3 {
		t.Fatalf("expected three ghost preserved keys, got %#v", activate.AddPreservedKey)
	}
	key := activate.AddPreservedKey[0]
	if key.KeyCode != uint32(vkF8) || key.Modifiers != 0 || key.GUID != ghostPreservedKeyGUID {
		t.Fatalf("unexpected ghost preserved key: %#v", key)
	}
	if activate.AddPreservedKey[1].KeyCode != uint32(vkF8) || activate.AddPreservedKey[1].Modifiers != tsfModifierShift || activate.AddPreservedKey[1].GUID != ghostLongPreservedKeyGUID {
		t.Fatalf("unexpected long ghost preserved key: %#v", activate.AddPreservedKey[1])
	}
	if activate.AddPreservedKey[2].KeyCode != uint32(vkF9) || activate.AddPreservedKey[2].GUID != ghostNextPreservedKeyGUID {
		t.Fatalf("unexpected next-candidate preserved key: %#v", activate.AddPreservedKey[2])
	}

	show := ime.HandleRequest(&imecore.Request{
		Method: "onPreservedKey", SeqNum: 2, Data: map[string]interface{}{"guid": ghostPreservedKeyGUID},
	})
	if show.ReturnValue != 1 || show.ShowMessage == nil || show.ShowMessage.Message != "苹果。" {
		t.Fatalf("unexpected preserved-key reveal response: %#v", show)
	}

	accept := ime.HandleRequest(&imecore.Request{
		Method: "onPreservedKey", SeqNum: 3, Data: map[string]interface{}{"guid": strings.ToUpper(ghostPreservedKeyGUID)},
	})
	if accept.ReturnValue != 1 || accept.CommitString != "苹果。" || !accept.HideMessage {
		t.Fatalf("unexpected preserved-key accept response: %#v", accept)
	}
}

func TestGhostCompletionPreservedF8GeneratesFromFreshSurroundingText(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.ghostEnabled = true
	ime.ghostConfig = aiCompletionConfig{IdleMS: 450, ContextTokens: 128, CandidateCount: 3}
	generated := make(chan aiCompletionRequest, 1)
	ime.ghostGenerator = func(input aiCompletionRequest, _ aiCompletionConfig) ([]string, error) {
		generated <- input
		return []string{"苹果。"}, nil
	}
	t.Cleanup(ime.resetGhostCompletion)

	payload := ghostContextEnvelope + "我今天吃了一个" + "\x1f" + "，然后去散步。"
	resp := ime.HandleRequest(&imecore.Request{
		Method:             "onPreservedKey",
		SeqNum:             1,
		Data:               map[string]interface{}{"guid": ghostPreservedKeyGUID},
		CloudClipboardText: payload,
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected on-demand F8 to be consumed, got %#v", resp)
	}

	select {
	case input := <-generated:
		if input.Context != "我今天吃了一个" || input.FollowingContext != "，然后去散步。" {
			t.Fatalf("unexpected surrounding context: %#v", input)
		}
	case <-time.After(time.Second):
		t.Fatal("on-demand F8 did not start completion")
	}
}

func TestGhostCompletionAcceptPrimesContinuousCompletion(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostContext = "我今天吃了一个"
	ime.ghostFollowingContext = "，然后去散步。"
	ime.ghostCandidates = []string{"苹果"}
	ime.ghostConfig = aiCompletionConfig{IdleMS: 1, ContextTokens: 128, CandidateCount: 1}
	generated := make(chan aiCompletionRequest, 1)
	ime.ghostGenerator = func(input aiCompletionRequest, _ aiCompletionConfig) ([]string, error) {
		generated <- input
		return []string{"，感觉很甜。"}, nil
	}
	t.Cleanup(ime.resetGhostCompletion)

	resp := ime.HandleRequest(&imecore.Request{
		Method:             "onPreservedKey",
		SeqNum:             1,
		Data:               map[string]interface{}{"guid": ghostPreservedKeyGUID},
		CloudClipboardText: ghostContextEnvelope + "我今天吃了一个" + "\x1f，然后去散步。",
	})
	if resp.CommitString != "苹果" {
		t.Fatalf("expected accepted completion, got %#v", resp)
	}
	select {
	case input := <-generated:
		if input.Context != "我今天吃了一个苹果" {
			t.Fatalf("expected accepted text in next context, got %#v", input)
		}
	case <-time.After(time.Second):
		t.Fatal("acceptance did not prime the next completion")
	}
}

func TestGhostCompletionEditingKeysInvalidateButDoNotSchedulePreEditSnapshots(t *testing.T) {
	for _, key := range []int{vkBack, vkDelete, vkLeft, vkRight, vkUp, vkDown, vkHome, vkEnd, vkPrior, vkNext} {
		req := &imecore.Request{KeyCode: key, KeyStates: make(imecore.KeyStates, 256)}
		if isGhostContextKey(req) {
			t.Fatalf("expected pre-edit key %d not to schedule stale context", key)
		}
		if !shouldInvalidateGhostForKey(req) {
			t.Fatalf("expected editing key %d to invalidate visible completion", key)
		}
	}
}

func TestGhostCompletionShiftF8RequestsLongSentence(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.ghostEnabled = true
	ime.ghostConfig = aiCompletionConfig{ContextTokens: 128, CandidateCount: 3}
	generated := make(chan aiCompletionRequest, 1)
	ime.ghostGenerator = func(input aiCompletionRequest, _ aiCompletionConfig) ([]string, error) {
		generated <- input
		return []string{"，于是我们决定沿着河边慢慢散步，享受难得的悠闲时光。"}, nil
	}
	t.Cleanup(ime.resetGhostCompletion)

	resp := ime.HandleRequest(&imecore.Request{
		Method: "onPreservedKey", SeqNum: 1,
		Data:               map[string]interface{}{"guid": ghostLongPreservedKeyGUID},
		CloudClipboardText: ghostContextEnvelope + "周末天气不错" + "\x1f",
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected Shift+F8 consumed, got %#v", resp)
	}
	select {
	case input := <-generated:
		if !input.Long || input.Context != "周末天气不错" {
			t.Fatalf("unexpected long request: %#v", input)
		}
	case <-time.After(time.Second):
		t.Fatal("long completion did not start")
	}
}

func TestGhostCompletionPlainF8AcceptsVisibleLongCompletion(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostLong = true
	ime.ghostContext = "周末天气不错"
	ime.ghostCandidates = []string{"，我们去公园走走吧。"}

	resp := ime.HandleRequest(&imecore.Request{
		Method:             "onPreservedKey",
		SeqNum:             1,
		Data:               map[string]interface{}{"guid": ghostPreservedKeyGUID},
		CloudClipboardText: ghostContextEnvelope + "周末天气不错" + "\x1f",
	})
	if resp.ReturnValue != 1 || resp.CommitString != "，我们去公园走走吧。" || !resp.HideMessage {
		t.Fatalf("expected plain F8 to accept visible long completion, got %#v", resp)
	}
}

func TestGhostCompletionEscapeCancelsAndIsConsumed(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostCandidates = []string{"，很高兴认识你。"}
	escape := &imecore.Request{KeyCode: vkEscape, KeyStates: make(imecore.KeyStates, 256)}

	filterResp := imecore.NewResponse(1, true)
	if !ime.handleGhostKeyDownFilter(escape, filterResp) || filterResp.ReturnValue != 1 || !filterResp.HideMessage {
		t.Fatalf("expected Escape filter to consume and hide completion, got %#v", filterResp)
	}
	if ime.ghostReady || ime.ghostVisible || len(ime.ghostCandidates) != 0 {
		t.Fatal("expected Escape to clear completion state")
	}
	downResp := imecore.NewResponse(2, true)
	if !ime.handleGhostKeyDown(escape, downResp) || downResp.ReturnValue != 1 || !downResp.HideMessage {
		t.Fatalf("expected Escape keydown to remain consumed, got %#v", downResp)
	}
	upResp := imecore.NewResponse(3, true)
	if !ime.handleGhostKeyUpFilter(escape, upResp) || upResp.ReturnValue != 1 {
		t.Fatalf("expected Escape keyup to be consumed, got %#v", upResp)
	}
}

func TestGhostCompletionF9CyclesVisibleCandidate(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostCandidates = []string{"苹果", "香蕉", "橘子"}
	resp := ime.HandleRequest(&imecore.Request{
		Method: "onPreservedKey", SeqNum: 1,
		Data: map[string]interface{}{"guid": ghostNextPreservedKeyGUID},
	})
	if resp.ReturnValue != 1 || resp.ShowMessage == nil || resp.ShowMessage.Message != "香蕉" {
		t.Fatalf("expected F9 to show second candidate, got %#v", resp)
	}
}

func TestGhostCompletionOrdinaryF9CyclesWhenPreservedRegistrationIsUnavailable(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostCandidates = []string{"苹果", "香蕉", "橘子"}
	f9 := &imecore.Request{KeyCode: vkF9, KeyStates: make(imecore.KeyStates, 256)}
	if !ime.handleGhostKeyDownFilter(f9, imecore.NewResponse(1, true)) {
		t.Fatal("expected ordinary F9 filter to consume visible completion")
	}
	resp := imecore.NewResponse(2, true)
	if !ime.handleGhostKeyDown(f9, resp) || resp.ShowMessage == nil || resp.ShowMessage.Message != "香蕉" {
		t.Fatalf("expected ordinary F9 to show second candidate, got %#v", resp)
	}
}

func TestNormalizeInlineCompletionsRejectsAssistantPersona(t *testing.T) {
	got := normalizeInlineCompletions("你好", []string{
		"，我是你的智能助手。",
		"，有什么可以帮你？",
		"无法确定具体上下文，请补充信息。",
		"，很高兴认识你。",
	}, 3)
	if len(got) != 1 || got[0] != "，很高兴认识你。" {
		t.Fatalf("unexpected filtered candidates: %#v", got)
	}
}

func TestGhostCompletionF8DuringDebounceIsConsumedAndRequestsReveal(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostTimer = time.AfterFunc(time.Hour, func() {})
	defer ime.resetGhostCompletion()

	f8 := &imecore.Request{KeyCode: vkF8, KeyStates: make(imecore.KeyStates, 256)}
	if !ime.handleGhostKeyDownFilter(f8, imecore.NewResponse(1, true)) {
		t.Fatal("expected F8 during debounce to be consumed")
	}
	if !ime.handleGhostKeyDown(f8, imecore.NewResponse(2, true)) {
		t.Fatal("expected F8 during debounce to enter reveal-wait state")
	}
	if !ime.ghostRevealRequested {
		t.Fatal("expected pending suggestion to reveal when generation finishes")
	}
}

func TestGhostCompletionTimerStartsGeneratorWithoutReadingBackendOffThread(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.ghostEnabled = true
	ime.ghostConfig = aiCompletionConfig{IdleMS: 1, CandidateCount: 1}
	generated := make(chan string, 1)
	ime.ghostGenerator = func(input aiCompletionRequest, _ aiCompletionConfig) ([]string, error) {
		generated <- input.Context
		return []string{"今天很好。"}, nil
	}
	t.Cleanup(ime.resetGhostCompletion)

	req := &imecore.Request{
		KeyCode:            'A',
		CharCode:           'a',
		KeyStates:          make(imecore.KeyStates, 256),
		CloudClipboardText: "我",
	}
	ime.scheduleGhostCompletion(req)

	select {
	case context := <-generated:
		if context != "我" {
			t.Fatalf("unexpected completion context %q", context)
		}
	case <-time.After(time.Second):
		t.Fatal("completion generator did not start after idle delay")
	}
}

func TestGhostCompletionTimerRunsAfterHandleRequestUnlocks(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()
	ime.rawInputTracked = "ni"
	ime.ghostEnabled = true
	ime.ghostConfig = aiCompletionConfig{IdleMS: 1, CandidateCount: 1}
	generated := make(chan string, 1)
	ime.ghostGenerator = func(input aiCompletionRequest, _ aiCompletionConfig) ([]string, error) {
		generated <- input.Context
		return []string{"今天很好。"}, nil
	}
	t.Cleanup(ime.resetGhostCompletion)

	ime.HandleRequest(&imecore.Request{
		Method:             "filterKeyDown",
		SeqNum:             1,
		KeyCode:            vkSpace,
		CharCode:           vkSpace,
		KeyStates:          make(imecore.KeyStates, 256),
		CloudClipboardText: "ni",
	})
	resp := ime.HandleRequest(&imecore.Request{
		Method:             "onKeyDown",
		SeqNum:             2,
		KeyCode:            vkSpace,
		CharCode:           vkSpace,
		KeyStates:          make(imecore.KeyStates, 256),
		CloudClipboardText: "ni",
	})
	if resp.CommitString != "你" {
		t.Fatalf("expected normal commit before completion, got %q", resp.CommitString)
	}
	terminated := ime.HandleRequest(&imecore.Request{
		Method: "onCompositionTerminated",
		SeqNum: 3,
		Forced: false,
	})
	if terminated.HideMessage {
		t.Fatal("normal composition termination must not hide or cancel queued completion")
	}

	select {
	case context := <-generated:
		if context != "你" {
			t.Fatalf("unexpected committed context %q", context)
		}
	case <-time.After(time.Second):
		t.Fatal("completion generator remained blocked after HandleRequest returned")
	}
}

func TestGhostCompletionLeftAltCyclesOnlyWhileVisible(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostCandidates = []string{"苹果。", "香蕉。", "橘子。"}
	leftAlt := &imecore.Request{KeyCode: vkMenu, KeyStates: make(imecore.KeyStates, 256)}

	resp := imecore.NewResponse(1, true)
	if !ime.handleGhostKeyDown(leftAlt, resp) {
		t.Fatal("expected Left Alt to cycle visible ghost")
	}
	if resp.ShowMessage == nil || resp.ShowMessage.Message != "香蕉。" {
		t.Fatalf("expected second candidate, got %#v", resp.ShowMessage)
	}

	ime.ghostVisible = false
	if ime.handleGhostKeyDown(leftAlt, imecore.NewResponse(2, true)) {
		t.Fatal("expected Left Alt to pass through while ghost is hidden")
	}
	rightAlt := &imecore.Request{KeyCode: vkMenu, IsExtended: true, KeyStates: make(imecore.KeyStates, 256)}
	ime.ghostVisible = true
	if ime.handleGhostKeyDown(rightAlt, imecore.NewResponse(3, true)) {
		t.Fatal("expected Right Alt to pass through")
	}
}

func TestGhostCompletionTypingRejectsSuggestion(t *testing.T) {
	ime := newTestIME()
	ime.ghostEnabled = true
	ime.ghostReady = true
	ime.ghostVisible = true
	ime.ghostCandidates = []string{"苹果。"}
	req := &imecore.Request{KeyCode: 'A', CharCode: 'a', KeyStates: make(imecore.KeyStates, 256)}
	resp := imecore.NewResponse(1, true)
	if ime.handleGhostKeyDownFilter(req, resp) {
		t.Fatal("typing should continue through the normal input path")
	}
	if !resp.HideMessage || ime.hasGhostWork() {
		t.Fatalf("expected typing to hide and invalidate ghost, resp=%#v", resp)
	}
}

func TestNormalizeCompletionContextKeepsSixRecentSentencesAndBudget(t *testing.T) {
	context := "第一句很早。第二句保留。第三句保留。第四句保留。第五句保留。第六句保留。第七句正在输入"
	got := normalizeCompletionContext(context, 128)
	if strings.Contains(got, "第一句很早") {
		t.Fatalf("expected oldest sentence to be removed, got %q", got)
	}
	for _, want := range []string{"第二句保留", "第三句保留", "第四句保留", "第五句保留", "第六句保留", "第七句正在输入"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}

	long := strings.Repeat("你", 200)
	got = normalizeCompletionContext(long, 128)
	if len([]rune(got)) != 128 {
		t.Fatalf("expected 128 CJK runes for approximate 128-token budget, got %d", len([]rune(got)))
	}
}

func TestNormalizeInlineCompletionsRemovesRepeatedContextAndStopsAtSentence(t *testing.T) {
	got := normalizeInlineCompletions("我今天吃了一个", []string{
		"1. 我今天吃了一个苹果。然后继续解释",
		"- 香蕉。",
		"香蕉。",
	}, 3)
	want := []string{"苹果。", "香蕉。"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestNormalizeInlineCompletionsKeepsAndRepairsLeadingPunctuation(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions("你好", "", []string{
		"，很高兴认识你。",
		"最近过得怎么样？",
	}, 3, 20)
	want := []string{"，很高兴认识你。", "，最近过得怎么样？"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestNormalizeInlineCompletionsClassifiesAttachmentAndNewClause(t *testing.T) {
	tests := []struct {
		name      string
		context   string
		candidate string
		want      string
	}{
		{"question particle attaches", "你好", "吗？", "吗？"},
		{"invalid comma before particle removed", "你好", "，吗？", "吗？"},
		{"greeting starts another clause", "你好", "很高兴认识你。", "，很高兴认识你。"},
		{"repeated subject starts another clause", "你好吗？我是朱泽宇", "我是你的新朋友。", "，我是你的新朋友。"},
		{"incomplete opinion stays attached", "我觉得", "我是对的。", "我是对的。"},
		{"object stays attached", "我今天吃了一个", "苹果", "苹果"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeInlineCompletionsWithOptions(test.context, "", []string{test.candidate}, 1, ghostShortMaxRunes)
			if len(got) != 1 || got[0] != test.want {
				t.Fatalf("got %#v want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeInlineCompletionsSeparatesCompletedPredicates(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions("柑橘调与木质香交织非常和谐", "", []string{
		"令人惊喜。",
		"层次清晰。",
	}, 3, ghostShortMaxRunes)
	if len(got) != 2 || got[0] != "，令人惊喜。" || got[1] != "，层次清晰。" {
		t.Fatalf("unexpected predicate separator: %#v", got)
	}
	got = normalizeInlineCompletionsWithOptions("柑橘调与木质香交织非常和谐，令人惊喜", "", []string{
		"非常棒的选择！",
	}, 3, ghostShortMaxRunes)
	if len(got) != 1 || got[0] != "，非常棒的选择！" {
		t.Fatalf("unexpected chained predicate separator: %#v", got)
	}
	got = normalizeInlineCompletionsWithOptions("口感醇厚带果酸回甘", "", []string{
		"口感层次丰富",
	}, 3, ghostShortMaxRunes)
	if len(got) != 1 || got[0] != "，层次丰富。" {
		t.Fatalf("unexpected repeated topic cleanup: %#v", got)
	}
}

func TestNormalizeInlineCompletionsRemovesOverlapAndPathologicalRepetition(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions(
		"爱乐压是一种咖啡机的用法", "，但也需要一点练习。",
		[]string{"用法推荐推荐推荐推荐推荐，操作起来很方便，但也需要一点练习。"}, 3, 40)
	if len(got) != 1 || got[0] != "推荐，操作起来很方便" {
		t.Fatalf("unexpected repetition cleanup: %#v", got)
	}
}

func TestNormalizeInlineCompletionsRepeatedlyRemovesTypedPrefix(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions(
		"风味很丰富，具体有番茄、", "",
		[]string{"番茄、番茄、柑橘"}, 3, 16)
	if len(got) != 1 || got[0] != "柑橘" {
		t.Fatalf("unexpected repeated prefix cleanup: %#v", got)
	}
	got = normalizeInlineCompletionsWithOptions("这个方案值得", "", []string{"推荐推荐"}, 3, 16)
	if len(got) != 1 || got[0] != "推荐" {
		t.Fatalf("unexpected doubled phrase cleanup: %#v", got)
	}
}

func TestUnsafeInlineBoundaryRejectsProductTokenSplit(t *testing.T) {
	if !hasUnsafeInlineBoundary("我尝试了水洗肯尼亚", "AA，香气浓郁") {
		t.Fatal("expected Chinese/ASCII product token split to be unsafe")
	}
	if hasUnsafeInlineBoundary("我今天吃了一个", "，然后去散步") {
		t.Fatal("expected punctuation boundary to allow insertion")
	}
}

func TestLongCompletionGetsNaturalSentenceEnding(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions("这个方案的问题是", "", []string{
		"需要进一步讨论细节才能找到合适的解决办法",
	}, 3, ghostLongMaxRunes)
	if len(got) != 1 || got[0] != "需要进一步讨论细节才能找到合适的解决办法。" {
		t.Fatalf("unexpected long completion: %#v", got)
	}
}

func TestLongCompletionRejectsMidSentenceHardTruncation(t *testing.T) {
	incomplete := strings.Repeat("这是一段仍然没有结束的内容", 8)
	got := normalizeInlineCompletionsWithOptions("接下来", "", []string{incomplete}, 3, ghostLongMaxRunes)
	if len(got) != 0 {
		t.Fatalf("expected over-limit incomplete sentence to be rejected, got %#v", got)
	}
	complete := "我准备调整研磨度和水温，再重新冲一杯比较风味差异。后面不应保留"
	got = normalizeInlineCompletionsWithOptions("接下来", "", []string{complete}, 3, ghostLongMaxRunes)
	if len(got) != 1 || got[0] != "我准备调整研磨度和水温，再重新冲一杯比较风味差异。" {
		t.Fatalf("expected completion to stop at sentence boundary, got %#v", got)
	}
}

func TestInfillCompletionUsesBothSidesAndStripsFollowingText(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions("今天喝了", "的耶加雪菲，香气浓郁。", []string{
		"一杯",
		"埃塞俄比亚产的",
		"来自埃塞俄比亚的",
		"杯来自埃塞俄比亚的耶加雪菲，香气浓郁顺滑。",
	}, 3, 20)
	want := []string{"来自埃塞俄比亚", "杯来自埃塞俄比亚"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestLongCompletionRejectsExistingAndSemanticRestatements(t *testing.T) {
	context := "今天喝了耶加雪菲，香气浓郁。这种独特的风味搭配完美诠释了精品咖啡的魅力所在。"
	got := normalizeInlineCompletionsWithOptions(context, "", []string{
		"这种独特的风味搭配完美诠释了精品咖啡的魅力所在。",
		"下次准备换一种冲煮方式，看看能否呈现更多层次。",
	}, 3, ghostLongMaxRunes)
	if len(got) != 1 || got[0] != "下次准备换一种冲煮方式，看看能否呈现更多层次。" {
		t.Fatalf("unexpected exact restatement filtering: %#v", got)
	}
	got = normalizeInlineCompletionsWithOptions("我是一个小猪", "", []string{
		"我是一只可爱的小猪，每天在草地上打滚玩耍。",
		"，每天最期待的事情就是晒太阳和吃饱饭。",
	}, 3, ghostLongMaxRunes)
	if len(got) != 1 || got[0] != "，每天最期待的事情就是晒太阳和吃饱饭。" {
		t.Fatalf("unexpected semantic restatement filtering: %#v", got)
	}
}

func TestShortCompletionRejectsGenericCliches(t *testing.T) {
	got := normalizeInlineCompletionsWithOptions("这杯咖啡层次丰富。", "", []string{
		"回味无穷", "令人难忘", "下次想换个水温再试试",
	}, 3, 16)
	if len(got) != 1 || got[0] != "下次想换个水温再试试" {
		t.Fatalf("unexpected generic completion filtering: %#v", got)
	}
}

func TestCommittedGhostContextReplacesInlinePreeditWithCommit(t *testing.T) {
	tests := []struct {
		name        string
		document    string
		composition string
		commit      string
		want        string
	}{
		{"inline pinyin", "我今天吃了一个pingguo", "pingguo", "苹果", "我今天吃了一个苹果"},
		{"plain commit", "我今天吃了一个", "", "苹果", "我今天吃了一个苹果"},
		{"already committed", "我今天吃了一个苹果", "", "苹果", "我今天吃了一个苹果"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := committedGhostContext(test.document, test.composition, test.commit); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}

func TestAIConfigExplicitEmptyActionsEnablesOnlyCompletion(t *testing.T) {
	cfg, err := parseAIConfigJSON([]byte(`{
		"api":{"base_url":"http://127.0.0.1:8080/v1","api_key":"ime-local","model":"qwen3.5-4b-ime"},
		"actions":[],
		"completion":{"enabled":true,"idle_ms":450,"context_tokens":128,"max_output_tokens":32,"candidate_count":3,"temperature":0.35}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Actions) != 0 {
		t.Fatalf("expected legacy actions disabled, got %#v", cfg.Actions)
	}
	if !cfg.Completion.Enabled || cfg.Completion.ContextTokens != 128 || cfg.Completion.CandidateCount != 3 {
		t.Fatalf("unexpected completion config: %#v", cfg.Completion)
	}
}
