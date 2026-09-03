# 静潮 Still Current

> A framework-agnostic design mother system for websites and software interfaces.
>
> 核心印象：**清晰得毫不费力，却又意外地优雅。**

## 0. Operating contract

Every derived project MUST choose:

1. one theme: `mist-shore` or `starry-night`;
2. one expression level: `functional`, `editorial`, or `showcase`;
3. only the components and tokens it needs.

State the choice before implementation:

```text
Theme: mist-shore
Expression: functional
Reason: clarity and speed dominate this task-oriented product
```

This is a mother system, not a page template. Preserve its hierarchy, interaction grammar, and semantic roles while adapting composition and content.

### Expression levels

| Level | Best for | Art direction | Motion |
|---|---|---|---|
| `functional` | tools, forms, dashboards | Color, proportion, and nearly invisible texture | Fast, local, restrained |
| `editorial` | documentation, blogs, portfolios | Atmospheric fields, selective serif type, controlled asymmetry | Gentle spatial continuity |
| `showcase` | launches, exhibitions, personal homepages | Art MAY shape major composition and transitions | Cinematic only at key boundaries |

Expression changes atmosphere, never usability.

### Decision priority

When goals conflict, resolve them in this order:

1. **Clarity and accessibility**
2. **Semantic correctness**
3. **Structural hierarchy**
4. **Interaction feedback**
5. **Atmosphere and brand expression**

A lower priority MUST yield to a higher one. Elegance never compensates for ambiguity.

### Normative language

- **MUST / MUST NOT** — non-negotiable.
- **SHOULD / SHOULD NOT** — strong default; deviation requires observed evidence or a concrete product reason.
- **MAY** — optional technique.
- Direct rules beginning with “Never”, “Do not”, or “Avoid” carry **MUST NOT** force unless explicitly qualified.
- Direct rules beginning with “Use” carry **MUST** force when stated as a requirement.
- Rules beginning with “Prefer” carry **SHOULD** force.
- Examples explain intent; they do not create requirements.

### Evidence rule

- Judge decisions in realistic content and density, across supported environments.
- Preserve a rule's intent when its literal implementation creates noise or weakens hierarchy.
- Evaluate repeated components together; repetition amplifies decoration.
- Prefer the smallest intervention that solves the observed problem.

---

## 1. Design thesis

### Name and emotional structure

**静潮 / Still Current**

“静” is apparent calm: breathing room, quiet surfaces, and an interface that recedes behind content.

“潮” is hidden response: focus, dimming, restrained light, and spatial continuity make meaningful change unmistakable.

- **川端康成 — intervals:** silence separates meaning.
- **黑塞 — inward warmth:** warm paper, dusk orange, water, mist, and wood-like neutrals keep precision humane.
- **莫奈 — atmosphere:** translucent fields and softened edges suggest light without noisy spectacle.
- **歌川广重 — composition:** clear planes, asymmetric framing, and deliberate foreground/background relationships.
- **梵高 — night energy:** sparse luminous strokes MAY appear in `starry-night`, but MUST NOT become constant texture.

These references contribute methods, never literal motifs.

External references follow the same boundary: absorb Linear's technical clarity, Apple's receding chrome, Notion's modular calm, and Mastercard's institutional warmth without reproducing their branding. The result MUST NOT copy recognizable compositions, logos, proprietary type, fashionable glass effects, neon AI gradients, or decorative cultural symbols.

### Product promise

At any moment, a user SHOULD be able to answer:

1. Where am I?
2. What matters now?
3. What can I do next?
4. What changed after I acted?

### Signature gesture: quiet current

For a meaningful interaction:

- a precise persimmon edge marks focus;
- nearby light subtly gathers;
- irrelevant surroundings MAY recede;
- confirmation travels once as a restrained ripple, line, or brush-light, then disappears.

The gesture SHOULD feel discovered, not advertised.

---

## 2. Core principles

### Effortless clarity

- One region has one obvious purpose.
- One visual group has at most one dominant action.
- Prefer progressive disclosure over showing every option.
- Put state and feedback close to the affected object.
- Use labels before relying on unfamiliar icons.

### Restrained warmth

- Deep-sea blue carries persistent structure, trust, and continuity.
- Persimmon marks a current moment: focus, active state, confirmation, or one critical action.
- Warm neutrals prevent clinical sterility.
- Orange is a signal, not wallpaper.

### Breathing efficiency

- Related controls use compact internal spacing; unrelated regions use larger external spacing.
- Empty space MUST clarify grouping, sequence, or importance.
- Do not turn every section into a card.
- Reduce inter-item spacing before compressing content or line height.

### Structural restraint

- Derive form from function and hierarchy before adding atmosphere.
- Use one primary organizing device per group: spacing, alignment, divider, tint, or enclosure.
- Add a second only when it communicates different information.
- A component SHOULD remain clear in flat color before gradients, shadows, or motion.
- Every visible border, ornament, and container needs a structural or semantic reason.

### Motion with consequence

- Animate changes of state, position, or attention—not mere presence.
- Keep routine feedback local and fast.
- Reserve cinematic transitions for `showcase` or major `editorial` boundaries.
- Every motion MUST have a reduced-motion equivalent.

### Bilingual calm

- Chinese is primary; English supports rather than duplicates every label.
- Preserve hierarchy and component width when language changes.
- Never compress Chinese merely to match English width.

---

## 3. Color system

Colors are semantic. Bind components to roles such as `focus-ring`, not palette names such as `orange-500`.

### Mist Shore / 雾岸

```yaml
theme: mist-shore
color:
  canvas:             "#F6F7F3"
  canvas-warm:        "#F2F0E9"
  surface:            "#FCFDFB"
  surface-raised:     "#FFFFFF"
  surface-sunken:     "#EBF0EF"
  surface-selected:   "#E3EDF0"

  ink:                "#142A38"
  ink-secondary:      "#425D68"
  ink-muted:          "#60747B"
  ink-inverse:        "#F8FAF7"

  sea:                "#17465F"
  sea-hover:          "#0F3A51"
  sea-pressed:        "#0A2E41"
  sea-soft:           "#DCE9EC"

  persimmon:          "#C2552C"
  persimmon-hover:    "#AD4823"
  persimmon-pressed:  "#8E381B"
  persimmon-soft:     "#F3D8CA"
  persimmon-glow:     "rgba(194, 85, 44, 0.20)"

  border:             "#D5DEDD"
  border-strong:      "#AABBBB"
  divider:            "#E2E8E6"
  overlay:            "rgba(10, 30, 42, 0.44)"

  success:            "#2F7258"
  warning:            "#A96518"
  danger:             "#A9433D"
  info:               "#2D6885"
```

### Starry Night / 星夜

```yaml
theme: starry-night
color:
  canvas:             "#071722"
  canvas-warm:        "#101D24"
  surface:            "#0D2431"
  surface-raised:     "#143142"
  surface-sunken:     "#06121A"
  surface-selected:   "#1A3B4D"

  ink:                "#EDF3F0"
  ink-secondary:      "#B7C8C9"
  ink-muted:          "#81999E"
  ink-inverse:        "#102532"

  sea:                "#86B7C8"
  sea-hover:          "#A5C9D5"
  sea-pressed:        "#6FA4B8"
  sea-soft:           "#193B4B"

  persimmon:          "#E37A46"
  persimmon-hover:    "#F08C58"
  persimmon-pressed:  "#C86132"
  persimmon-soft:     "#4A2A20"
  persimmon-glow:     "rgba(227, 122, 70, 0.24)"

  border:             "#294654"
  border-strong:      "#456675"
  divider:            "#1D3946"
  overlay:            "rgba(2, 9, 14, 0.64)"

  success:            "#69AE8B"
  warning:            "#D9A05B"
  danger:             "#D87970"
  info:               "#77ACC4"
```

### Usage

- Use `canvas` for the page; introduce `surface` only when separation is meaningful.
- Use `sea` for persistent structure: navigation, links, progress, and primary controls.
- Use `persimmon` for transient or singular emphasis.
- A viewport SHOULD contain no more than one solid persimmon CTA.
- Never use orange for large body text or give sea and persimmon equal page-wide weight.
- Semantic states begin with language, position, and structure; color reinforces them.
- `info`, `success`, `warning`, and `danger` MUST remain distinct from brand accents and survive grayscale or color-vision variation.

### Gradient decision

A gradient represents a change in **atmosphere, depth, attention, or state**. It is not default polish.

Use one only when it:

1. softens a meaningful boundary between large regions;
2. gives depth to a broad atmospheric field;
3. gathers attention without becoming a focal object;
4. expresses a directional transition that matters.

Do not use one when flat color already communicates the state, repetition would create a page-wide pattern, the surface is dense or precise, or legibility depends on it.

Expression threshold:

- `functional`: large environmental transitions or orientation cues only;
- `editorial`: major reading or narrative boundaries;
- `showcase`: one coherent light logic per viewport, never several competing effects.

Implementation:

- Default to one atmospheric gradient system per view.
- Keep stops close in hue and luminance; resolve at least one end into the canvas.
- Prefer low-opacity radial or long linear washes.
- Never stack gradient, strong border, rounded container, and shadow unless each solves a separate problem.
- Remove gradients first when reducing decorative intensity.

Reference, not recipe:

```css
radial-gradient(
  70% 55% at 72% 12%,
  rgba(92, 151, 170, 0.14) 0%,
  rgba(246, 247, 243, 0) 72%
);
```

---

## 4. Typography and content rhythm

### Font roles

```yaml
font:
  ui: '"IBM Plex Sans", "Noto Sans SC", "Source Han Sans SC", sans-serif'
  editorial: '"Newsreader", "Noto Serif SC", "Source Han Serif SC", serif'
  mono: '"IBM Plex Mono", "Noto Sans Mono CJK SC", monospace'
```

- `ui` is the default.
- `editorial` MAY appear in display headings, quotations, dates, and reflective passages; it MUST NOT appear in routine controls or dense navigation.
- Use no more than two expressive families per view.
- Specialized content can require its own typeface or renderer; verify real output and visual compatibility.

### Type scale

```yaml
type:
  display: { size: "clamp(2.75rem, 6vw, 5.5rem)", line-height: 0.98, weight: 500, tracking: "-0.035em" }
  h1:      { size: "clamp(2.25rem, 4vw, 4rem)", line-height: 1.08, weight: 550, tracking: "-0.025em" }
  h2:      { size: "clamp(1.75rem, 2.8vw, 2.75rem)", line-height: 1.15, weight: 550, tracking: "-0.018em" }
  h3:      { size: "clamp(1.25rem, 1.7vw, 1.625rem)", line-height: 1.25, weight: 550, tracking: "-0.01em" }
  body-lg: { size: "1.125rem", line-height: 1.72, weight: 400 }
  body:    { size: "1rem", line-height: 1.65, weight: 400 }
  small:   { size: "0.875rem", line-height: 1.55, weight: 400 }
  label:   { size: "0.875rem", line-height: 1.35, weight: 550 }
  caption: { size: "0.75rem", line-height: 1.45, weight: 450 }
```

Chinese headings usually need less negative tracking than English.

### Hierarchy and rhythm

- Hierarchy MUST follow a consistent trend across size, weight, color, spacing, position, or density.
- Adjacent levels MUST remain distinguishable, including the least prominent level and ordinary content.
- Do not assign a different ornament to every level.
- Internal gaps bind a group; external gaps separate groups.
- Major transitions MAY open space; local transitions SHOULD preserve continuity.
- Handle consecutive structural elements explicitly so margins do not accumulate into accidental voids.
- Supporting objects SHOULD remain closer to their introducing content than to the next independent region.
- Validate with realistic length, density, Chinese, and English—not short placeholders alone.

Copy:

- Keep UI labels short and literal.
- Use sentence case in English; avoid all caps except brief metadata.
- Prose SHOULD measure roughly `30–38` Chinese or `55–72` Latin characters per line.
- Poetry belongs in atmosphere and editorial content, not critical operations or error recovery.

---

## 5. Spacing, layout, and shape

```yaml
space:
  0: 0
  1: 4px
  2: 8px
  3: 12px
  4: 16px
  5: 24px
  6: 32px
  7: 48px
  8: 64px
  9: 96px
  10: 144px

layout:
  content-max: 1200px
  reading-max: 720px
  compact-max: 560px
  gutter-mobile: 20px
  gutter-tablet: 32px
  gutter-desktop: 48px
  grid-columns: 12
  grid-gap: "clamp(16px, 2vw, 32px)"

radius:
  xs: 4px
  sm: 8px
  md: 12px
  lg: 18px
  xl: 28px
  pill: 999px

elevation:
  1: "0 1px 2px rgba(14, 39, 52, 0.06)"
  2: "0 8px 28px rgba(14, 39, 52, 0.09)"
  3: "0 24px 64px rgba(7, 23, 34, 0.16)"
```

- Inside controls: usually `8–16px`; related elements: `8–24px`; distinct regions: `48–96px`.
- Showcase regions MAY use `96–144px` separation, but MUST retain enough adjacent context to preserve orientation.
- Functional layouts SHOULD align strongly to the grid.
- Editorial layouts MAY offset one major element; showcase layouts MAY break the grid once per viewport.
- Controls default to `sm`, cards to `md`; pills are reserved for compact status and segmented controls.
- Borders organize more often than shadows. Never stack visible shadows.
- Use elevation 1 for small floating controls, 2 for popovers, and 3 only for dialogs or major showcase media.

---

## 6. Interaction and motion

### Focus and state

```css
:focus-visible {
  outline: 2px solid var(--color-persimmon);
  outline-offset: 3px;
  box-shadow: 0 0 0 5px var(--color-persimmon-glow);
}
```

- Keyboard focus MUST remain visible on both themes and over imagery.
- Hover increases contrast or reveals a local edge; travel stays within `1–2px`.
- Pressed MAY use `scale(0.985)` or a deeper surface; never bounce.
- Selected state uses color plus structure, icon, or weight—not color alone.
- Disabled state preserves legibility and removes decorative motion.
- Loading preserves the original control width.

### Attention and feedback

```yaml
attention:
  backdrop-light: "rgba(10, 30, 42, 0.44)"
  backdrop-dark: "rgba(2, 9, 14, 0.64)"
  background-saturation: 0.82
  background-scale: 0.995
  transition: 180ms
```

- Use attention dimming for dialogs, command palettes, focused previews, and immersive navigation—not ordinary tooltips.
- The focused layer MUST provide an obvious exit.
- Background blur is optional and MUST remain below `6px`.
- Important completed actions MAY emit one quiet-current response for `240–420ms`, normally below `0.22` opacity.
- Never combine ripple, scale, glow, and particles on one control.

### Motion tokens

```yaml
motion:
  instant: 90ms
  fast: 140ms
  base: 200ms
  slow: 320ms
  cinematic: 720ms
  ease-standard: "cubic-bezier(0.2, 0.8, 0.2, 1)"
  ease-enter: "cubic-bezier(0.16, 1, 0.3, 1)"
  ease-exit: "cubic-bezier(0.7, 0, 0.84, 0)"
  ease-cinematic: "cubic-bezier(0.22, 1, 0.36, 1)"
```

- Controls: `90–140ms`; menus and disclosures: `140–200ms`; dialogs and drawers: `200–320ms`.
- Prefer opacity and transform over layout dimensions.
- Avoid spring motion in serious workflows.
- Cinematic motion MAY use `560–800ms` only at a major narrative boundary.
- Keep navigation landmarks stable and never hijack scrolling.

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    scroll-behavior: auto !important;
    animation-duration: 1ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 1ms !important;
  }
}
```

Reduced motion MUST preserve all information.

---

## 7. Component grammar

Shared rule: introduce a persistent container only when an element is independently actionable, repeated as a collection, owns state, or MUST detach from the canvas.

### Actions

- **Primary:** sea fill, inverse text, persimmon focus ring; once per action group.
- **Signal:** persimmon fill; only for one consequential positive action, never destructive.
- **Secondary:** transparent or raised surface with border; MUST NOT compete with Primary.
- **Ghost:** no persistent container; use for navigation and low-priority utilities.
- **Destructive:** danger color and explicit object-specific language.

Controls SHOULD be at least `40px` high with `8px` radius. Standard actions use `10px 16px` padding; adapt it only when density or target size requires a concrete exception. Preserve visible labels and focus.

### Navigation and links

- Current location MUST be persistent and unmistakable.
- Active navigation uses sea plus one structural cue.
- Prose links use sea and a visible underline; navigation links MAY omit it when selection is explicit.
- Indicate external links when leaving the product is consequential.
- Reserve persimmon for focus or one singular current moment.

### Inputs and dialogs

- Keep labels visible above inputs; placeholders are examples, not labels.
- Put validation beside the field and state the recovery action.
- Dialog width follows content; primary action appears last in reading order.
- Destructive confirmation is never initially focused.
- Return focus to the invoking control on close.

### Cards and data

- Prefer spacing and dividers over boxed sections.
- Cards default to surface, `1px` border, `12px` radius, and no shadow.
- Hover elevation is allowed only when the entire card is clickable.
- Tables favor alignment and subtle dividers over boxed cells.
- Keep headers visible for long datasets.
- Use tabular numerals for changing values.
- Charts MUST remain understandable without relying on sea versus persimmon alone.

### Empty and error states

- State what happened plainly and offer one useful next step.
- Keep decorative imagery subordinate.
- Avoid mascots, confetti, and literary metaphors during failure.

---

## 8. Imagery and atmosphere

| Expression | Atmosphere |
|---|---|
| `functional` | Mostly solid surfaces; at most one low-opacity wash; no animated background |
| `editorial` | Soft edge diffusion, calm image fields, controlled asymmetry, clean text zones |
| `showcase` | One dominant scene per viewport; atmosphere MAY shape composition |

- Borrow art through framing, depth, light, and balance—not literal waves, bridges, stars, or “Eastern” symbols.
- Keep active texture away from body copy and precision surfaces.
- Avoid replicas of known paintings and generic AI watercolor blobs.

```yaml
texture:
  paper-grain-opacity: 0.018
  mist-opacity: 0.10
  brush-light-opacity: 0.08
  star-stroke-density: sparse
```

Texture disappears inside forms, tables, dialogs, and other precision surfaces.

---

## 9. Adaptation

```yaml
breakpoint:
  compact: 480px
  mobile: 768px
  tablet: 1024px
  wide: 1440px
```

- Collapse by task priority, not screen position.
- Mobile navigation SHOULD express the same hierarchy without mechanically reproducing the desktop arrangement.
- Touch targets SHOULD normally be at least `40×40px`.
- Reduce decorative layers before content spacing.
- Convert asymmetric compositions into a clear vertical sequence on mobile.
- Preserve focus and hierarchy across keyboard, pointer, and touch.
- Never scale desktop text mechanically; use fluid type tokens.

Language:

- Prefer one active language per content block.
- If Chinese and English MUST appear together, Chinese leads and English receives lower emphasis.
- Controls MUST allow roughly `1.5×` label expansion.
- Avoid fixed widths that depend on one language.
- Keep dates, numerals, and units culturally consistent within a view.

Rendering:

- Verify every supported theme, viewport, input mode, and rendering environment.
- Match perceived hierarchy rather than assuming identical values render identically everywhere.
- Meaning MUST NOT depend on effects an environment can omit or alter.

---

## 10. Intensity recipes

### Functional product

```yaml
theme: mist-shore
expression: functional
typography: ui-only
orange-role: focus-and-single-critical-action
texture: nearly-none
motion: local
layout: strict-grid
```

### Knowledge or content product

```yaml
theme: mist-shore
expression: editorial
typography: ui-plus-selective-editorial
orange-role: focus-and-reading-position
texture: mist-and-paper
motion: gentle-spatial-continuity
layout: grid-with-one-offset
```

### Showcase

```yaml
theme: mist-shore-default + starry-night-alternate
expression: showcase
typography: editorial-display + ui-controls
orange-role: focal-light
texture: atmospheric
motion: cinematic-at-major-boundaries
layout: asymmetric-framing
```

---

## 11. Completion checklist

### Purpose and hierarchy

- [ ] The view has one clear primary purpose.
- [ ] Each action group has at most one dominant action.
- [ ] Current location, state, and next action are obvious.
- [ ] Spacing binds related elements more strongly than unrelated regions.
- [ ] Hierarchy survives realistic content length and density.

### Semantics and interaction

- [ ] Hover, focus, pressed, disabled, loading, success, and error states exist where relevant.
- [ ] Keyboard focus is visible and follows a logical path.
- [ ] Feedback appears near the action that caused it.
- [ ] Semantic states remain understandable without color alone.
- [ ] Destructive actions name the affected object and recovery limits.

### Visual restraint

- [ ] Semantic tokens replace scattered values.
- [ ] Persimmon remains scarce.
- [ ] Every container, border, shadow, ornament, and gradient has a reason.
- [ ] Repeated treatments remain quiet at realistic density.
- [ ] Chinese and English typography both look deliberate.
- [ ] Mist Shore and Starry Night preserve the same hierarchy.

### Context and motion

- [ ] Layout is checked near `390px`, `768px`, `1024px`, and `1440px`.
- [ ] Supported themes, inputs, and rendering environments are verified.
- [ ] Decorative layers reduce before essential content.
- [ ] Functional motion is brief; cinematic motion is exceptional.
- [ ] Reduced motion preserves all information.

---

## 12. Agent implementation brief

```text
Read DESIGN.md before changing the interface.

State one theme and expression level. Resolve conflicts using the documented
priority order. Reuse existing components and semantic tokens before creating
new ones. Preserve clarity, accessibility, hierarchy, keyboard focus, and the
scarcity of persimmon.

Treat gradients as atmospheric infrastructure, not component polish. Keep
functional surfaces quieter than expressive surfaces. Artistic references
shape composition and atmosphere; never turn them into literal decoration.

Implement real interaction states. Then verify realistic content density,
supported viewports and themes, keyboard and reduced-motion behavior, repeated
components together, and every supported rendering environment.
```

For each project append:

```md
## Project adaptation

- Product:
- Audience:
- Theme:
- Expression level:
- Primary user goal:
- Existing component source:
- Required views:
- Art intensity exceptions:
- Verification commands:
```

Explicit project decisions MAY override defaults. They MUST NOT violate the decision priority or the central promise: **effortless clarity with unexpected elegance**.

## Project adaptation

- Product: Windows 个人智能拼音输入法。
- Audience: 需要长期、高频、低干扰中文输入的单用户。
- Theme: `mist-shore`，随系统高对比度保留语义可读性。
- Expression level: `functional`；柿橙仅用于当前候选、光标和必要状态。
- Primary user goal: 在不被设置和增值功能打断的情况下，快速、稳定地完成中文输入。
- Existing component source: Win32 TSF、GDI 候选窗、Rime 后端和现有 AI ghost completion。
- Required views: 横排候选窗、AI 提示窗、五项顶层菜单、系统托盘通知。
- Art intensity exceptions: 无；不使用渐变、卡片叠层、装饰动画或用户皮肤。
- Product semantics: `progressive compression` 用于菜单与诊断入口；`bounded failure` 用于本地 AI 和词库操作。
- Typography: `Noto Sans SC` 16 逻辑像素候选（约 14.5 px 可见字面高度）、10 逻辑像素注释；不打包额外字体，避免增大安装包。
- Verification commands: 见 `ARCHITECTURE.md` 的“验证”。
