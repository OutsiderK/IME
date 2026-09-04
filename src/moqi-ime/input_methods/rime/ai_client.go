package rime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const aiRequestTimeout = 20 * time.Second

type aiClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type chatCompletionsRequest struct {
	Model            string        `json:"model"`
	Messages         []chatMessage `json:"messages"`
	Temperature      float64       `json:"temperature"`
	TopP             float64       `json:"top_p,omitempty"`
	FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
	MaxTokens        int           `json:"max_tokens,omitempty"`
	RepeatPenalty    float64       `json:"repeat_penalty,omitempty"`
	DryMultiplier    float64       `json:"dry_multiplier,omitempty"`
	DryBase          float64       `json:"dry_base,omitempty"`
	DryAllowedLength int           `json:"dry_allowed_length,omitempty"`
	DryPenaltyLastN  int           `json:"dry_penalty_last_n,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func newConfiguredAIReviewGenerator(cfg *aiRuntimeConfig) func(aiGenerateRequest) ([]string, error) {
	client := newAIClient(cfg)
	if client == nil {
		return nil
	}
	return client.GenerateReviewCandidates
}

func newAIClient(cfg *aiRuntimeConfig) *aiClient {
	if cfg == nil {
		return nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.API.BaseURL), "/")
	apiKey := strings.TrimSpace(cfg.API.APIKey)
	model := strings.TrimSpace(cfg.API.Model)
	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	return &aiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: aiRequestTimeout,
		},
	}
}

func newAIClientFromEnv() *aiClient {
	return newAIClient(envAIConfig())
}

func (c *aiClient) GenerateReviewCandidates(input aiGenerateRequest) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("AI client is not configured")
	}
	input = normalizeAIGenerateRequest(input)
	if input.PreviousCommit == "" && input.Composition == "" && len(input.Candidates) == 0 {
		return nil, fmt.Errorf("AI input is empty")
	}

	payload := chatCompletionsRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "你是一个中文输入法助手。请严格按用户要求输出候选文案，每条单独一行，不要编号，不要项目符号，不要解释，不要输出思考过程。",
			},
			{
				Role:    "user",
				Content: buildAIUserPrompt(input.Prompt, input),
			},
		},
		Temperature: 0.8,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call AI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("AI API returned no choices")
	}

	candidates := parseReviewCandidates(parsed.Choices[0].Message.Content)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("AI API returned empty review content")
	}
	return candidates, nil
}

func (c *aiClient) GenerateInlineCompletions(input aiCompletionRequest, cfg aiCompletionConfig) ([]string, error) {
	return c.GenerateInlineCompletionsContext(context.Background(), input, cfg)
}

func (c *aiClient) GenerateInlineCompletionsContext(ctx context.Context, input aiCompletionRequest, cfg aiCompletionConfig) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("AI client is not configured")
	}
	context := normalizeCompletionContext(input.Context, cfg.ContextTokens)
	if context == "" {
		return nil, fmt.Errorf("completion context is empty")
	}
	following := normalizeFollowingCompletionContext(input.FollowingContext, 96)
	isInfill := following != ""

	systemPrompt := "你是中文输入法的补全器。根据光标前后文，预测用户本人接下来最可能输入的新增文字。候选就是要原样插入光标处的全部字符，必须包含衔接所需的标点。若前文虽未带标点但语义已经完整（例如问候后另起分句），候选应以恰当的逗号、句号、感叹号或问号开头；若前文仍缺宾语、补语或名称，则直接补词，不要乱加标点。只输出候选，每行一个；不要复述前文，不要解释。如果前文末尾是逗号或顿号，续写下一个并列项，禁止重复刚写过的项目。如果有光标后文，候选必须能直接插入且不能重复或改写后文。"
	exampleUser := "光标前：我今天吃了一个\n光标后：（空）\n输出3个短补全候选。"
	exampleAssistant := "苹果\n三明治\n冰淇淋"
	actualTask := "输出 %d 个互不相同的短补全候选，最多14个汉字。每个候选只续写一个词组或写到下一个自然停顿为止；若补全后当前分句语义已完整，末尾必须包含，。！？；之一；若前文当前分句已完整，候选必须以恰当标点开头。严禁把两个本应由标点分隔的成分直接连在一起。一个词足够时不要强行写长。"
	maxTokens := cfg.MaxOutputTokens
	maxRunes := ghostShortMaxRunes
	if completionContextEndsSentence(context) {
		actualTask = "前文已经完整结束一句。输出 %d 个紧扣具体主题的下一句开头或短句，最多18个汉字，必须引入新的具体信息或动作；禁止空泛评价，例如回味无穷、推荐、值得、下次再来。"
		if maxTokens < 64 {
			maxTokens = 64
		}
	}
	if isInfill {
		systemPrompt = "你是中文输入法的句中填空器。光标前文和后文都已存在。输出能同时衔接左右两侧的最短新增片段；长度完全由语义决定，可以只有一两个字。不得复述、改写或包含后文。必须检查拼接后的完整句是否通顺。只输出候选，每行一个，不解释。"
		exampleUser = "光标前：会议安排在\n光标后：下午三点开始。\n输出3个最短填空候选。"
		exampleAssistant = "明天\n本周五\n下周一"
		actualTask = "输出 %d 个最短填空候选。优先利用常识补全产地、时间、名称、属性或动作。"
		if strings.HasPrefix(following, "的") {
			actualTask += " 后文以“的”开头：候选必须是修饰后面名词的产地或属性，禁止数量词，末尾不要再输出“的”。"
			if noun := followingHeadNoun(following); noun != "" {
				actualTask += fmt.Sprintf(" 后文开头实体是“%s”；如果它有公认产地，第一候选优先填写该产地。", noun)
			}
		}
		if maxTokens < 64 {
			maxTokens = 64
		}
		maxRunes = 20
	} else if input.Long {
		systemPrompt = "你是中文输入法的句子接龙器。根据光标前后文，直接续写用户本人最可能输入的一句完整中文。此任务没有标准答案，不需要用户补充信息。只输出候选，每行一个；不要复述前文，不要解释。如果有光标后文，候选必须能直接插入且不能重复或改写后文。"
		exampleUser = "光标前：下班以后我打算\n光标后：（空）\n输出3个完整句补全候选。"
		exampleAssistant = "先去附近的超市买些食材，回家给自己做顿晚饭。\n沿着河边慢慢走一会儿，让忙碌一天的心情放松下来。\n约上朋友找家安静的小店，聊聊最近发生的新鲜事。"
		actualTask = "输出 %d 个互不相同的自然续写候选，长度由语义决定、不设下限，通常不超过50个汉字。每个候选必须是语法完整的一句话并以。！？结束；宁可提前结束也绝不能输出半句话；继续发展内容，不要换一种说法总结前文。"
		if maxTokens < 256 {
			maxTokens = 256
		}
		maxRunes = ghostLongMaxRunes
	}
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: exampleUser},
		{Role: "assistant", Content: exampleAssistant},
	}
	if !input.Long && !isInfill {
		messages = append(messages,
			chatMessage{Role: "user", Content: "光标前：常见的水果有苹果、\n光标后：（空）\n输出3个短补全候选。"},
			chatMessage{Role: "assistant", Content: "香蕉\n橘子\n草莓"},
			chatMessage{Role: "user", Content: "光标前：你好\n光标后：（空）\n输出3个短补全候选。"},
			chatMessage{Role: "assistant", Content: "，很高兴认识你。\n！没想到能在这里见到你。\n，请问怎么称呼？"},
			chatMessage{Role: "user", Content: "光标前：柑橘调与木质香交织非常和谐\n光标后：（空）\n输出3个短补全候选。"},
			chatMessage{Role: "assistant", Content: "，令人惊喜。\n，层次十分清晰。\n，余韵也很干净。"},
		)
	}
	messages = append(messages, chatMessage{Role: "user", Content: fmt.Sprintf("光标前：%s\n光标后：%s\n%s",
		context, printableFollowingContext(following), fmt.Sprintf(actualTask, cfg.CandidateCount))})

	payload := chatCompletionsRequest{
		Model:            c.model,
		Messages:         messages,
		Temperature:      cfg.Temperature,
		TopP:             0.85,
		FrequencyPenalty: 0.12,
		MaxTokens:        maxTokens,
	}
	if isLoopbackAIEndpoint(c.baseURL) {
		payload.RepeatPenalty = 1.12
		payload.DryMultiplier = 0.8
		payload.DryBase = 1.75
		payload.DryAllowedLength = 2
		payload.DryPenaltyLastN = 64
	}

	content, err := c.completeContext(ctx, payload)
	if err != nil {
		return nil, err
	}
	candidates := normalizeInlineCompletionsWithOptions(context, following, parseReviewCandidates(content), cfg.CandidateCount, maxRunes)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("AI API returned empty completion content")
	}
	return candidates, nil
}

func completionContextEndsSentence(context string) bool {
	runes := []rune(strings.TrimSpace(context))
	return len(runes) > 0 && isSentenceTerminator(runes[len(runes)-1])
}

func followingHeadNoun(following string) string {
	runes := []rune(strings.TrimSpace(following))
	if len(runes) == 0 || runes[0] != '的' {
		return ""
	}
	runes = runes[1:]
	end := 0
	for end < len(runes) && end < 12 {
		r := runes[end]
		if unicode.IsSpace(r) || isSentenceTerminator(r) || r == '，' || r == ',' || r == '、' || r == '：' || r == ':' {
			break
		}
		end++
	}
	return strings.TrimSpace(string(runes[:end]))
}

func printableFollowingContext(context string) string {
	context = normalizeFollowingCompletionContext(context, 64)
	if context == "" {
		return "（空）"
	}
	return context
}

func isLoopbackAIEndpoint(baseURL string) bool {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func (c *aiClient) complete(payload chatCompletionsRequest) (string, error) {
	return c.completeContext(context.Background(), payload)
}

func (c *aiClient) completeContext(ctx context.Context, payload chatCompletionsRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal AI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call AI API: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode AI response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("AI API returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func normalizeAIGenerateRequest(input aiGenerateRequest) aiGenerateRequest {
	input.PreviousCommit = strings.TrimSpace(input.PreviousCommit)
	input.Composition = strings.TrimSpace(input.Composition)
	normalized := make([]string, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		normalized = append(normalized, candidate)
		if len(normalized) == 3 {
			break
		}
	}
	input.Candidates = normalized
	return input
}

func buildAIUserPrompt(promptTemplate string, input aiGenerateRequest) string {
	input = normalizeAIGenerateRequest(input)
	promptTemplate = strings.TrimSpace(promptTemplate)
	if promptTemplate == "" {
		promptTemplate = defaultAIUserPromptTemplate()
	}
	prompt, replaced := applyAIPromptPlaceholders(promptTemplate, input)
	if replaced {
		return prompt
	}
	return promptTemplate + "\n\n" + buildAIContextText(input)
}

func applyAIPromptPlaceholders(prompt string, input aiGenerateRequest) (string, bool) {
	candidate1 := aiCandidateAt(input.Candidates, 0)
	candidate2 := aiCandidateAt(input.Candidates, 1)
	candidate3 := aiCandidateAt(input.Candidates, 2)
	top3 := buildAICandidatesTop3Text(input.Candidates)
	previousCommit := input.PreviousCommit
	if previousCommit == "" {
		previousCommit = "无"
	}
	replaced := false

	replacements := []struct {
		old string
		new string
	}{
		{old: "{{previous_commit}}", new: previousCommit},
		{old: "{{composition}}", new: input.Composition},
		{old: "{{raw_input}}", new: input.Composition},
		{old: "{{candidate_1}}", new: candidate1},
		{old: "{{candidate_2}}", new: candidate2},
		{old: "{{candidate_3}}", new: candidate3},
		{old: "{{first_candidate}}", new: candidate1},
		{old: "{{second_candidate}}", new: candidate2},
		{old: "{{third_candidate}}", new: candidate3},
		{old: "{{candidates_top3}}", new: top3},
	}

	for _, item := range replacements {
		if strings.Contains(prompt, item.old) {
			prompt = strings.ReplaceAll(prompt, item.old, item.new)
			replaced = true
		}
	}
	return prompt, replaced
}

func aiCandidateAt(candidates []string, index int) string {
	if index < 0 || index >= len(candidates) {
		return ""
	}
	return candidates[index]
}

func buildAICandidatesTop3Text(candidates []string) string {
	if len(candidates) == 0 {
		return "无"
	}
	lines := make([]string, 0, len(candidates))
	for i, candidate := range candidates {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, candidate))
	}
	return strings.Join(lines, "\n")
}

func buildAIContextText(input aiGenerateRequest) string {
	previousCommit := input.PreviousCommit
	if previousCommit == "" {
		previousCommit = "无"
	}
	composition := input.Composition
	if composition == "" {
		composition = "无"
	}
	return "上一句：" + previousCommit + "\n原始输入：" + composition + "\n前三个候选词：\n" + buildAICandidatesTop3Text(input.Candidates)
}

func parseReviewCandidates(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var jsonArray []string
	if strings.HasPrefix(content, "[") && json.Unmarshal([]byte(content), &jsonArray) == nil {
		return normalizeAICandidates(jsonArray)
	}

	lines := strings.Split(content, "\n")
	candidates := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		candidates = append(candidates, line)
	}
	if len(candidates) == 0 {
		candidates = append(candidates, content)
	}
	return normalizeAICandidates(candidates)
}
