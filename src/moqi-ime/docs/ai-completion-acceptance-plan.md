# AI 续写接受率改进计划（初稿）

> 状态：研究记录 / 初步实现，后续仍可大幅调整
> 版本：0.3
> 记录日期：2026-09-04  
> 最近修订：切片 A/B 已实现，记录代码位置、日志路径与汇总命令
> 适用范围：`src/moqi-ime/input_methods/rime` 的 AI ghost completion

## 1. 文档目的

本文记录如何提高 AI 续写的实际接受率和输入效率，供后续设计、实验和实现时参考。它不是稳定接口规范，也不是一次性实施清单；随着真实数据、交互反馈和模型能力变化，方案可以大幅调整。

本计划遵循三个原则：

1. 先测量，再优化。没有稳定的回放集和指标时，不根据少量主观体验决定模型或提示词优劣。
2. 优先解决端到端瓶颈。候选是否及时、是否值得展示、排序是否正确，通常比单独提升语言模型能力更重要。
3. 保持实现简单。先用可解释的 Trie、n-gram、线性模型或树模型验证价值，不提前引入复杂在线学习、分布式服务或大量完整性校验。

## 2. 问题边界

输入法中的“预测”包含两个不同任务，后续数据、模型和指标不能混用。

### 2.1 拼音候选重排

发生在汉字提交前：给定拼音和 Rime Top N 候选，选择更可能正确的汉字候选。

```text
拼音 + Rime Top N -> reranker -> 重排后的候选栏
```

此任务继续参考 [pinyin-reranker-training.md](./pinyin-reranker-training.md)，不应让 4B 生成模型在每次按键时替代 Rime。

### 2.2 AI ghost completion

发生在已有文本之后或光标中间：预测用户接下来可能输入的短文本，用户通过 `F8` 接受、`F9` 切换。

```text
左右上下文 + 用户历史 -> 多路候选 -> 排序/展示决策 -> ghost suggestion
```

本文以下内容只讨论这一任务。

## 3. 当前基线

截至本文记录时，默认配置为：

```text
模型：Qwen3.5-4B Q4_K_M，本地 llama.cpp
自动触发防抖：450 ms
上下文预算：320 tokens
最大输出：64 tokens
候选数：3
temperature：0.3
llama-server parallel：1
```

当前实现已经具备：

- 短续写、长续写和 infill；
- 输入变化后通过请求序号阻止过期结果上屏；
- 基本的去重、截断、重复上下文过滤；
- 智能、常驻和手动三种 AI 生命周期；
- 本地模型失败时不影响普通 Rime 输入。

当前最明显的改进空间：

- 没有展示、接受、跳过、部分匹配、撤销等结构化事件，无法形成可靠基线；
- 提示词要求一次生成三条“不同”候选，目标更接近多样性而不是 Top 1 接受率；
- 没有独立的接受概率排序器，也没有判断“本次是否应该展示”；
- 固定长度、固定防抖和固定最近上下文不能适配不同场景；
- 输入变化后旧 HTTP 请求仍可能继续推理；在单 slot 服务上，它可能延迟更新后的请求；
- 三个 LLM 候选全部生成后再返回，增加首个可用候选的等待时间。

## 4. 目标与非目标

### 4.1 目标

首要产品指标是“净节省输入操作”，而不是孤立的接受率。没有按键基线时，先使用明确标注的字符代理指标：

```text
saved_actions_proxy = accepted_runes - accept_actions
```

得到真实按键计数后，再估算“如果这些 AI 字符由用户手工输入需要多少按键”；不能直接把一个汉字当成一次按键。

工程上同时约束：

- 候选在下一次用户按键前准备好的比例；
- p50 / p90 预测延迟；
- 打扰率和接受后快速撤销率；
- CPU、内存和模型常驻成本；
- 本地数据的可见、可关和可删除。

### 4.2 非目标

本轮计划不要求：

- 更换成更大的生成模型；
- 建设云端数据或训练服务；
- 完整的联邦学习、差分隐私或跨设备同步；
- 用 RL/GRPO 直接学习全部展示策略；
- 为本地日志加入复杂哈希链、逐条签名或重复校验；
- 在没有真实回放数据前确定最终阈值和模型结构。

## 5. 建议架构

```text
输入上下文
   |
   +-- 用户短语 Trie / 近期历史 --------+
   +-- 轻量 n-gram --------------------+--> 候选池
   +-- Qwen 短续写 / infill -----------+      |
                                                v
                                      去重与安全过滤
                                                |
                                                v
                                      接受概率 ranker
                                                |
                                                v
                                       展示 gate / Top 1
                                                |
                                  +-------------+-------------+
                                  |                           |
                               不展示                  F8 接受 / F9 切换
                                                              |
                                                              v
                                                        本地反馈事件
```

设计要点：

- 历史中见过的固定表达优先走 Trie/n-gram，低延迟且容易个性化；
- Qwen 处理未见前缀、语义续写和 infill 长尾；
- 候选来源只是 ranker 的一个特征，所有候选进入同一排序流程；
- 自动模式默认只展示 Top 1；F9 和手动模式再暴露次级候选；
- 低置信度时不展示，而不是强迫模型每次给出答案。

一个可用于实验的简单效用函数：

```text
utility(candidate) =
    P(accept | context, user, candidate) * (candidate_runes - 1)
    - interruption_weight * P(interrupt)
    - latency_weight * latency_ms
```

只有最高效用超过阈值时才展示。该公式是项目假设，不是已验证结论；第一版可以退化成“接受概率 × 节省字符数”并设置固定阈值。

## 6. 分阶段实施

### Phase 0：建立可观测基线

目标：回答“当前系统在什么场景下被接受、来得是否及时、失败在哪里”。

新增本地事件采用 JSONL，每行一个事件；无需额外数据库。事件使用统一信封，不要求每条事件重复所有业务字段：

```json
{
  "schema_version": 1,
  "time": "2026-09-04T12:00:00+08:00",
  "monotonic_time_ms": 38127,
  "session_id": "local-session-id",
  "opportunity_id": 42,
  "request_id": 17,
  "candidate_id": 2,
  "event": "candidate_ready",
  "payload": {}
}
```

`session_id` 在一次输入法会话内稳定；其余 ID 使用会话内递增整数即可，不需要全局 UUID 或内容哈希。`request_id`、`candidate_id` 只在相关事件中出现。墙上时间用于查阅日志，延迟和先后关系使用单调时间计算。

最小生命周期由四类对象构成：

```text
opportunity_created
  ├─ request_started
  │    ├─ candidate_ready
  │    ├─ request_cancel_requested
  │    └─ request_finished(status=completed|cancelled|timeout|error)
  ├─ shown
  │    ├─ cycled
  │    ├─ accepted
  │    └─ quick_undo
  ├─ input_advanced
  ├─ text_committed(source=manual|ai)
  └─ opportunity_closed(reason)
```

事件的最小载荷：

| 事件 | 必需载荷 | 用途 |
|---|---|---|
| `opportunity_created` | `mode`、`composition_version`、`trigger` | 形成 trigger rate 的完整分母 |
| `request_started` | `generator`、`requested_candidates` | 关联推理请求和机会 |
| `candidate_ready` | `candidates[]`、`generation_ms` | 保存候选集合和生成时特征快照 |
| `request_cancel_requested` | `reason` | 区分主动取消与普通失败 |
| `request_finished` | `status`、`finished_after_cancel`、`elapsed_ms` | 保证每个请求闭环 |
| `shown` | 按展示顺序排列的 `candidate_ids[]` | 确定 Top 1/Top 3 分母和原始排名 |
| `cycled` | `from_candidate_id`、`to_candidate_id` | 形成可靠的候选偏好对 |
| `accepted` | `candidate_id`、`accepted_runes`、`accept_actions` | 计算接受率和节省字符 |
| `input_advanced` | `composition_version` | 确定下一次输入时刻，不记录具体键值 |
| `text_committed` | `source`、`output_runes`、`physical_key_actions_since_previous_commit` | 形成手工/AI 最终输出字符和真实按键分母 |
| `quick_undo` | `candidate_id`、`removed_runes`、`elapsed_ms` | 标记接受后的强负反馈 |
| `opportunity_closed` | `reason`；适用时带 `matched_candidate_id`、`matched_prefix_runes`、`manual_runes` | 闭合未展示、部分匹配和放弃场景 |

每个 opportunity 必须有一次 `opportunity_created` 和恰好一次 `opportunity_closed`。未展示也是正常结果，关闭原因使用一个小型枚举：

```text
accepted
dismissed
new_input
cursor_moved
low_confidence
no_candidate
request_timeout
request_error
ai_disabled
session_closed
```

`candidate_ready` 为每个候选记录生成时可得的派生特征，而不是只记录最终入选项：

```json
{
  "candidate_id": 2,
  "feature_version": 1,
  "rank": 1,
  "source": "llm",
  "runes": 6,
  "mean_logprob": -0.31,
  "terminal_entropy": 0.74,
  "natural_end": true,
  "personal_frequency": 0
}
```

暂时拿不到的特征直接省略，不使用虚构默认值。默认日志不保存候选、左右上下文和最终输入原文；部分匹配在 opportunity 关闭时由运行时计算并保存 `matched_prefix_runes`。如果以后需要重新提取文本特征，可增加用户显式开启的研究模式。密码框和安全输入场景不采集。日志按普通本地文件管理，提供关闭和清除入口即可。

标签解释：

- `accepted`：强正例；
- `cycled` 后接受：被接受候选应排在此前跳过候选之前；
- 用户随后输入与候选前缀一致：部分正例；
- 接受后短时间内连续退格/撤销：强负例；
- 用户继续输入且不匹配：弱负例或未观察标签，不能一律解释为候选语义错误；
- 请求被新输入替代：延迟数据，不作为内容负例；以 `request_cancel_requested(reason=new_input)` 和最终 `request_finished` 表示。

离线汇总至少输出：

| 指标 | 确定性定义 |
|---|---|
| `trigger_rate` | 有 `shown` 的 opportunity / 全部 `opportunity_created`；按 `trigger` 分组报告 |
| `acceptance_at_1` | 接受首次 `shown.candidate_ids[0]` 的 opportunity / 已展示 opportunity |
| `acceptance_at_3` | 接受首次 `shown` 前三个 candidate ID 之一的 opportunity / 已展示 opportunity |
| `accepted_runes_per_100_output` | AI `text_committed.output_runes` / 全部 `text_committed.output_runes` × 100 |
| `observed_key_actions_per_100_output` | 全部 `physical_key_actions_since_previous_commit` / 全部输出字符 × 100；越低越好 |
| `estimated_saved_keys_per_100_output` | `AI 输出字符 × 手工提交平均按键/字符 - accept_actions`，再按每 100 输出字符归一化 |
| `partial_match_rate` | `matched_prefix_runes > 0` 的未接受展示 / 所有未接受展示 |
| `ready_before_next_key_rate` | 首次 `candidate_ready` 早于随后首次 `input_advanced` / 能观察到后续输入的请求；会话结束仍无后续输入的样本视为 censored |
| `latency_p50 / latency_p90` | `candidate_ready.monotonic_time_ms - request_started.monotonic_time_ms` 的分位数 |
| `interruption_rate` | 以 `dismissed` 或 `new_input` 关闭、未接受且 `matched_prefix_runes = 0` 的展示 / 全部展示 |
| `quick_undo_rate` | 发生 `quick_undo` 的接受 / 全部接受 |
| `late_finish_after_cancel_rate` | cancel 请求后仍以 `completed` 结束的客户端可见请求 / 全部收到 cancel 请求的请求 |

`stale_request_compute_rate` 不再作为纯客户端日志承诺的指标：HTTP 客户端返回 `cancelled` 后，外部 llama-server 是否仍占用 slot 不能从客户端事件严格得知。如后续需要该指标，应读取或扩展 llama-server 的 slot 级遥测。当前先报告客户端可观察的 `late_finish_after_cancel_rate` 和取消后的请求结束耗时。

中文输入中一个输出汉字通常对应多个物理按键，因此“输入字符数”不能替代真实按键数。Phase 0 同时保留字符利用率、实际按键率和基于手工提交均值的估算节省量。估算值用于版本间对比，不宣称是每条 AI 接受的真实反事实按键数。

验收条件：在不改变现有候选行为的前提下，opportunity、request、candidate 和 commit 四类生命周期均可闭合；从固定日志可以确定性生成上述客户端可观察指标，且默认日志不依赖保存原文。

### Phase 1：消除无效等待并简化生成

目标：先提升候选“来得及被用到”的比例。

建议改动：

1. 为每个 ghost request 创建可取消的 `context.Context`；新输入、光标移动、禁用 AI 或关闭会话时取消旧 HTTP 请求。
2. 保留请求序号检查，作为 UI 状态一致性判断；它与取消请求职责不同。
3. 设置短续写的硬时间预算。超时后可以采用已就绪的 Trie/n-gram 候选，否则不展示。
4. 固定提示前缀，启用 llama.cpp prompt cache/cache reuse。
5. 自动模式先生成一个低温候选；只在 `F9`、手动长续写或离线采样时生成更多分支。
6. 先用标点、最大字符数和重复规则停止；模型接口提供可靠 token 概率后，再实验基于熵的动态停止。

不建议仅把 `--parallel` 从 1 调大来掩盖旧请求未取消的问题。是否增加 slot 应通过峰值内存和延迟测试决定。

验收条件：新输入能中止旧请求；旧请求不会占用当前会话的唯一推理机会；自动模式的 p90 延迟不劣于基线，结果仍不会越过上下文版本上屏。

### Phase 2：混合召回与个人记忆

目标：让高频、重复和个人表达不必每次依赖 4B 模型重新推理。

第一版保持简单：

- 从用户明确接受的候选和正常提交文本中维护前缀 Trie；
- 为近期短语维护带衰减的频次；
- 使用最长前缀、频次和最近使用时间产生 1–2 个候选；
- 只存短文本及计数，不在第一版加入向量数据库；
- 未命中或置信度不足时再调用 Qwen。

以后确有跨句事实召回需求，再评估 embeddings/HNSW。个人固定表达优先用可审计的明文条目，方便用户理解和删除。

验收条件：固定回放集上，混合方案在同展示率下提高净节省按键；Trie 命中延迟明显低于 LLM；错误学习条目可以被自然衰减或删除。

### Phase 3：训练 ranker 和展示 gate

目标：从“模型最先生成什么”改为“用户最可能接受什么”。

候选特征可从小集合开始：

- 来源、长度、是否自然结束；
- 生成平均 log probability、末端熵、Top 1/Top 2 分差；
- Trie/n-gram 频次和最近使用时间；
- 是否重复左右上下文；
- 请求延迟、short/long/infill 模式；
- 粗粒度应用类别；
- 该用户在相似长度、来源下的历史接受率。

第一版模型优先 Logistic Regression 或 LightGBM，并做概率校准。训练时按会话或时间切分，不能把同一次输入产生的相邻前缀随机拆到训练集和测试集。

展示 gate 与 ranker 可以先共用同一个接受概率：

```text
rank：按 expected_saved_keys 排序
gate：expected_saved_keys >= threshold 时展示
```

阈值必须在相同 `trigger_rate` 或 coverage 下比较。单纯提高阈值会提高条件接受率，但可能降低总节省量。

验收条件：固定测试集上，Top 1 排序和净节省按键超过“LLM 原始顺序”；校准曲线能区分低、中、高接受概率区间；线上/个人试用不增加快速撤销率。

### Phase 4：生成模型后训练

目标：在测量、召回和排序链路稳定后，提升候选池质量。

推荐顺序：

1. 收集或合成 `context -> 最短有用续写` 样本；
2. SFT/QLoRA 训练；
3. 离线回放比较基础模型、提示词版本和 LoRA；
4. 积累可靠偏好对后，再考虑 DPO/ORPO；
5. 如果 4B 延迟仍是主要瓶颈，蒸馏到 0.6B–1.7B 级模型并重新评测。

训练样本应覆盖：

- 以 2–8 个汉字的短续写为主；
- 对话、说明文、代码旁中文、专业词和标点；
- short、long、infill 三种模式；
- 明确的 `<NO_SUGGESTION>` 样本；
- 当前系统产生的困难负例；
- 公开文本经教师模型改写得到的短消息/真实输入风格文本。

偏好数据解释要保守：F9 后接受可以形成可靠 pair；“没有接受”不必然表示候选差，可能只是用户已经继续输入或候选来晚了。

验收条件：在未参与训练、按会话或时间切分的回放集上，LoRA 在相同触发率和延迟预算下改善净节省按键；收益覆盖多个文本类别，而不是只来自训练集高频句。

### Phase 5：高级实验（按需）

只有前述链路显示明确瓶颈时再考虑：

- token 熵驱动的动态截断；
- speculative decoding；
- embeddings/HNSW 长期记忆；
- 根据应用类别或用户阶段动态选择模型；
- 用 contextual bandit 调整展示阈值；
- 联邦学习或跨设备同步。

目前不把 RL/GRPO 作为接受率优化的第一选择：它需要更可信的奖励和更大交互量，而现有研究尚未稳定证明复杂策略能直接改善真实输入速度。

## 7. 提示词实验方向

当前提示词工程只作为低成本实验，不替代评测和排序。

推荐的短提示结构：

```text
任务：补全用户将要输入的中文，只输出可直接插入的文本。
模式：short | long | infill
左文：...
右文：...              # 仅 infill
相关个人短语：...      # 仅命中时提供
约束：不要复述左文；不确定则输出空；在自然边界停止。
```

实验变量一次只改变一个主要维度：

- 单候选与三候选；
- temperature 0 / 0.1 / 0.3；
- 无 few-shot 与少量 few-shot；
- 固定字符上限与动态停止；
- 最近上下文与相关性筛选上下文；
- 自由文本输出与 grammar 约束输出。

模型训练稳定后应尝试删除 few-shot：它可能已被 SFT 吸收，而且会增加提示处理延迟。

## 8. 数据集与回放方式

每个真实输入会产生多个高度相关的前缀。数据切分必须以完整会话、对话或时间段为单位，避免同一句话的前半段进入训练集、后半段进入测试集。

建议目录约定（实现时再创建）：

```text
src/moqi-ime/experiments/ghost-completion/
  README.md
  schemas/
  fixtures/             # 可提交的脱敏小样本
  scripts/              # 日志汇总和离线回放

<本地研究数据目录>/
  raw/                  # 不提交
  normalized/           # 不提交
  splits/
  reports/
```

一次回放实验需要固定：

- 数据集版本和切分；
- 模型文件及量化版本；
- llama.cpp 版本与启动参数；
- 提示词版本；
- 候选生成参数；
- ranker/gate 版本和阈值；
- CPU/GPU 与线程配置；
- 输出汇总及逐样本结果。

初期可直接使用一个简短 JSON manifest 记录这些字段，不需要引入通用实验管理平台。

## 9. 测试计划

### 9.1 单元测试

在现有 Go 测试中补充：

- 事件状态机：每个 opportunity 恰好关闭一次，每个 request 恰好产生一个 `request_finished`；
- `session_id`、`opportunity_id`、`request_id`、`candidate_id` 的关联和会话内递增；
- `accepted`、`cycled`、各种 close reason 和 `quick_undo` 的标签转换；
- `candidate_ready` 保存所有候选的生成时特征和 `feature_version`；
- Unicode 字符长度、实际按键率、字符代理指标和估算节省按键计算；
- 使用固定事件 fixture 确定性计算 trigger、Acceptance@1/@3、partial match、ready-before-next-key 和取消指标；
- 候选去重、前缀合并和来源保留；
- gate 阈值边界；
- 新请求取消旧请求，取消错误不显示为 AI 故障；
- 已取消或过期结果不能更新 ghost UI；
- 默认事件序列化不包含原文；安全输入和日志关闭时不写事件。

沿用现有快速测试入口，并逐步把新测试加入 `TestGhostCompletion` 相关测试组：

```powershell
cd src/moqi-ime
go test ./input_methods/rime -run 'TestGhostCompletion'
```

### 9.2 HTTP 集成测试

用 `httptest.Server` 模拟模型服务，覆盖：

- 正常单候选/多候选；
- 流式或非流式响应（若实现流式）；
- 新输入触发请求取消；
- 取消请求产生 `request_cancel_requested` 和唯一的 `request_finished`；
- 超时、空结果、格式错误和 5xx；
- 旧请求晚于新请求返回；
- 取消后仍返回的响应标记为 `finished_after_cancel`，但不能更新 ghost UI；
- hard deadline 后采用备用候选或保持静默。

测试只验证系统行为，不需要模拟真实模型质量。

### 9.3 离线质量测试

同一回放集比较：

```text
A：当前 prompt + 当前候选顺序
B：精简 prompt + 单候选
C：Trie/n-gram + Qwen
D：C + ranker/gate
E：D + LoRA
```

报告必须同时给出 coverage、Acceptance@1、字符利用率、实际按键率、估算节省按键、部分匹配、延迟和错误案例，不能只汇报表现最好的单项指标。

### 9.4 性能测试

至少测试冷启动、热缓存和连续快速输入三种情况：

- 首次请求 TTFC/总延迟；
- 相同固定前缀下的缓存收益；
- 每 100–200ms 输入一个字符时的取消和排队行为；
- 模型常驻内存、峰值内存和 CPU 占用；
- 1 slot 与候选并发方案的对比。

性能测试使用当前目标电脑的真实 llama.cpp 服务；CI 不要求下载或运行 GGUF 模型。

### 9.5 手工验收

至少覆盖记事本、浏览器文本框和一个富文本编辑器：

1. 普通连续中文输入不被阻塞；
2. `F8` 接受、`F9` 切换、`Esc` 取消符合现有习惯；
3. 移动光标和修改前文后不出现旧候选；
4. AI 服务未启动、超时或崩溃时 Rime 正常工作；
5. infill 不重复右侧文本；
6. 日志关闭和清除入口有效；
7. 密码/安全输入场景无研究日志。

完整后端回归仍使用：

```powershell
cd src/moqi-ime
go test ./input_methods/rime
go test ./...
go build -trimpath -ldflags '-s -w' .
```

## 10. 实验判定规则

为了避免“提高接受率但降低实际价值”，所有实验遵守：

1. 离线比较使用相同测试切分；
2. 接受率在相同 trigger rate/coverage 下比较；
3. 最终排序优先看净节省按键，再看 Acceptance@1；
4. p90 延迟、打扰率和快速撤销率作为约束；
5. 相邻版本只改变少数可解释变量；
6. 保存逐样本结果，抽查成功和失败案例；
7. 论文中的提升值仅用于确定方向，不作为本项目验收阈值。

可用于第一轮个人试用的非承诺目标：

```text
在不降低 trigger rate 且不增加 quick_undo_rate 的条件下：
- ready_before_next_key_rate 明显提高；
- `estimated_saved_keys_per_100_output` 相对基线提高 5% 以上，并同时报告 `observed_key_actions_per_100_output`；
- p90 自动续写延迟不高于当前基线。
```

样本量不足时只报告原始计数和趋势，不给出过度精确的百分比结论。

## 11. 变更与回退策略

各阶段应通过少量独立配置开关进入：

```text
telemetry_enabled
personal_retrieval_enabled
ranker_enabled
show_gate_enabled
generator_profile
```

开关用于实验和快速回退，不需要为每个内部参数都建立永久配置。旧行为保留为一个基线 profile；新方案出现延迟或质量回退时，切回基线而不影响 Rime 主链路。

## 12. 思路来源与可迁移结论

### 12.1 直接相关研究

1. [Gmail Smart Compose: Real-Time Assisted Writing](https://arxiv.org/abs/1906.00080)
   - 采用低延迟、长度归一化打分和置信度触发；轻量个人 n-gram 与全局模型混合后，线上点击率相对提升约 6%，ExactMatch 提升约 10%。
   - 对本项目的启发：个性化不必先训练个人 LLM；先做本地短语/n-gram 和展示阈值。

2. [When Does Text Prediction Benefit from Additional Context?](https://aclanthology.org/2021.naacl-industry.1/)
   - 相关的近期对话上下文能显著改善匹配和字符节省，但上下文连续性很重要。
   - 对本项目的启发：上下文需要筛选，不是越长越好；对话双方内容都可能有用。

3. [ChaI-TeA: A Benchmark for Evaluating Autocomplete in Dialogue](https://aclanthology.org/2025.naacl-short.3/)
   - 将接受动作成本纳入字符节省指标；多个短分支优于少量长分支；困惑度排序仍有较大提升空间；LoRA 在其英文对话测试中带来约 4%–12% 的相对改善。
   - 对本项目的启发：同时优化长度、候选数和排序；LoRA 放在可靠回放评测之后。

4. [Chat-Ghosting: A Comparative Study of Methods for Text Prediction in Conversational AI](https://aclanthology.org/2026.eacl-long.209/)
   - Trie/n-gram 擅长已见前缀，神经模型擅长未见上下文；基于熵的动态停止改善精确率与输入节省；语义相似但不能直接拼接的候选仍会造成认知成本。
   - 对本项目的启发：混合召回、精确前缀指标和动态长度比单纯扩大模型更值得优先验证。

5. [Exploring and Adapting Chinese GPT to Pinyin Input Method](https://aclanthology.org/2022.acl-long.133/)
   - 拼音约束、拼音上下文和针对性训练改善中文拼音候选，尤其是缩写拼音。
   - 对本项目的启发：它主要支持提交前拼音候选重排/生成，不应直接当成提交后续写的证据。

6. [HuoziIME: On-Device LLM-Based Input Method Editor](https://aclanthology.org/2026.acl-demo.32/)
   - 展示了小型 Qwen、本地分层记忆、KV 缓存、量化和检索触发的端侧组合。
   - 对本项目的启发：架构可行，但论文用户评估规模很小，具体接受率仍需本项目自己验证。

7. [Prompt Public Large Language Models to Synthesize Data for Private On-device Applications](https://arxiv.org/abs/2404.04360)
   - 用公开数据和教师模型合成接近移动输入分布的训练数据，在 Gboard 真实数据上改善下一词预测。
   - 对本项目的启发：在真实个人数据较少时，可以先合成中文短输入样本，再由少量真实回放校准。

8. [Sequential Decision-Making for Inline Text Autocomplete](https://arxiv.org/abs/2403.15502)
   - 将展示策略建模为序列决策并考虑阅读/选择成本，但小规模研究没有证明 RL 一定改善输入速度。
   - 对本项目的启发：展示成本必须进入指标；RL/GRPO 暂不作为第一阶段方案。

### 12.2 工程能力来源

- [llama.cpp server 文档](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md)：可用于候选数、token 概率、预测时间预算、prompt cache、grammar、LoRA 等实验。
- [llama.cpp speculative decoding](https://github.com/ggml-org/llama.cpp/blob/master/docs/speculative.md)：仅在分析确认解码是主要延迟后再评估。
- [Federated Learning for Mobile Keyboard Prediction](https://research.google/pubs/federated-learning-for-mobile-keyboard-prediction/)：证明真实键盘交互数据的价值；本项目当前为个人本地应用，先采用本地学习，无需直接复刻联邦基础设施。

### 12.3 使用这些来源时的限制

- 大部分公开 autocomplete 数据是英文邮件、聊天或移动键盘数据；中文、Windows TSF 和 `F8` 接受交互可能有不同分布。
- 论文中的相对提升来自不同基线，不能相加，也不能当成本项目承诺。
- 离线 exact/prefix match 是代理指标，最终仍要用真实个人输入中的净节省按键、撤销和延迟验证。
- 本文记录的是截至 2026-09-04 的研究判断；实施前应复核模型、llama.cpp 接口和相关新研究。

## 13. 建议的首次实现切片

### 13.1 实现状态（2026-09-05）

切片 A 和切片 B 已完成首版实现；当前目标是收集可回放基线，尚未开始 Trie/n-gram、ranker、gate、提示词 A/B 或 LoRA 实验。

- 事件类型、JSONL 写入和生命周期校验：`internal/ghosttelemetry/telemetry.go`
- 确定性指标汇总：`internal/ghosttelemetry/metrics.go`
- 命令行入口：`cmd/ghost-metrics/main.go`
- 输入法事件接入、部分匹配和请求取消：`input_methods/rime/ghost_completion.go`
- HTTP `context.Context` 传递：`input_methods/rime/ai_client.go`
- 固定回放样本：`internal/ghosttelemetry/testdata/session.jsonl`

开启 AI 续写时，事件默认写入：

```text
%LOCALAPPDATA%\MoqiIM\Log\ai-completion-events-YYYY-MM-DD.jsonl
```

文件只保存 ID、时间、字符/按键数和派生特征，不保存上下文、候选文本或最终输入原文。Windows 前端在密码和私密输入域不提供上下文；Go 端对空上下文请求不记录提交事件。

如果需要停止收集，在用户 `ai_config.json` 的 `completion` 下设置 `"telemetry_enabled": false` 并重新加载配置。清理某一天的文件使用精确路径：

```powershell
go run ./cmd/ghost-metrics -clear -input "$env:LOCALAPPDATA\MoqiIM\Log\ai-completion-events-2026-09-05.jsonl"
```

离线汇总示例：

```powershell
cd src/moqi-ime
go run ./cmd/ghost-metrics -input "$env:LOCALAPPDATA\MoqiIM\Log\ai-completion-events-2026-09-05.jsonl"
```

基线运行一段时间后，先保存该命令的输出和对应版本，再开始后续候选策略实验。

### 13.2 原始切片定义

首次工作拆成两个可独立验证的小切片，避免把埋点正确性和请求行为变化混在同一次实验中。

切片 A：Phase 0 可观测基线

1. 定义统一事件信封以及 opportunity、request、candidate、commit 的载荷结构；
2. 记录 `opportunity_created`、请求生命周期、候选特征、展示/选择、输入推进、提交和 `opportunity_closed`；
3. 默认只记录字符数、按键数、时间和派生特征，不记录原文；
4. 提供离线汇总命令，并用固定 fixture 验证本文件第 6 节的客户端可观察指标；
5. 用当前候选行为收集基线，不引入 Trie、ranker、gate 或 LoRA。

切片 B：Phase 1 请求取消

1. 给每个 HTTP 请求加入真实 `context.Context` 取消；
2. 新输入、光标移动、禁用 AI 和关闭会话时调用取消；
3. 记录 `request_cancel_requested`、最终 `request_finished` 和取消结束耗时；
4. 使用 `httptest.Server` 验证取消、晚到响应和 UI 版本隔离；
5. 与切片 A 的基线比较 p50/p90、ready-before-next-key 和客户端可见的取消效果。

两个切片完成后，才进入 Trie/n-gram、ranker、提示词或后训练实验。这样既能验证日志设计和请求生命周期，也能为后续方案提供共同基线。
