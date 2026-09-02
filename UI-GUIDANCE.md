# UI Product Semantics Guidance

> An optional extension for stateful application interfaces.
>
> This document supplements `DESIGN.md`. It does not redefine that
> document's themes, tokens, typography, layout, component styling, or decision
> priority.

## 0. When to use this document

Read `DESIGN.md` for every design task. Also apply this document when
the interface includes one or more of the following:

- warnings, status, urgency, or conditions that may demand user attention;
- acknowledgement, progress, completion, freshness, or uncertain state;
- summaries, aggregation, overview/detail relationships, or compressed data;
- independently failing regions, optional capabilities, or partial operation.

Apply only the relevant sections. Before implementation, state which sections
apply and why. The core design system remains authoritative when rules conflict.

---

## 1. Consequential attention

- Allocate attention by consequence, urgency, and required action—not by technical novelty or internal abnormality.
- Expected, reversible, or non-blocking conditions SHOULD remain visually quiet.
- A warning MUST correspond to a meaningful consequence or decision; otherwise present a neutral state.
- Removing false urgency is part of clarity, not merely visual restraint.

---

## 2. Temporal legibility

- Distinguish acknowledgement, progress, completion, and resolution when they lead to different user decisions.
- Match visible precision to the evidence, freshness, and update cadence available.
- Prefer an honest unknown, stale, or indeterminate state to inferred certainty.
- Feedback is complete only when the outcome is observable, and affected representations either agree or explicitly represent any remaining inconsistency as pending or stale.

Illustrative examples:

| Concept | Example |
|---|---|
| Acknowledgement | The request has been received and queued; processing has not necessarily started. |
| Progress | The work is underway, for example, `Processing 62%`. |
| Completion | The operation has finished, for example, `Processing complete`. |
| Resolution | The user's underlying need is satisfied, for example, `The file is now available`. |
| Stale | A last-known value is shown with its timestamp after its freshness window has expired. |
| Unknown | The system lacks reliable evidence for the current value. |
| Indeterminate | Work is known to be underway, but its total amount or remaining time is not knowable. |

---

## 3. Progressive compression

- A summary MUST preserve the distinctions needed for the next decision.
- Remove explanation before removing identity, consequence, or action.
- Use aggregation only when the collection or pattern matters more than the individual item.
- A single actionable item is usually named rather than merely counted.
- Overview and detail MAY differ in density, but MUST NOT disagree in meaning.

---

## 4. Bounded failure

- A failure SHOULD occupy only the scope it actually invalidates.
- Preserve the primary path when partial operation remains safe and truthful.
- Explain what is affected and what remains available before exposing technical cause.
- The loss of an optional capability MUST NOT be presented as total product failure.
- Local failure SHOULD remain locally recoverable whenever the surrounding structure is still valid.

Illustrative example: if export fails but editing and saving are confirmed to
remain operational, preserve those capabilities and present export as the
bounded failure. A path is safe and truthful only when the product has evidence
that the promised operation remains available.

---

## 5. Completion checklist

For every applicable section, verify its corresponding check. Mark a section as
not applicable only when its interface condition is absent.

### Consequential attention

- [ ] Visual prominence matches consequence and required action.

### Temporal legibility

- [ ] Time, freshness, progress, and uncertainty are represented only as precisely as the available evidence permits.

### Progressive compression

- [ ] Summaries preserve the information needed for the next decision and remain semantically aligned with detail views.

### Bounded failure

- [ ] Partial failures remain proportional to their actual scope while safe and truthful primary paths remain available.

---

## 6. Agent implementation brief

```text
Apply only the UI guidance sections relevant to the required views, and state
which sections apply before implementation.

Allocate attention by consequence and required action. Represent time,
freshness, progress, and uncertainty only as precisely as the evidence permits.
Keep compressed summaries useful for the next decision, and keep overview and
detail semantically aligned. Contain partial failures within their actual scope
while preserving any primary path that remains safe and truthful.

After implementation, verify every applicable checklist item with realistic
content and state transitions. Report non-applicable sections explicitly rather
than inventing states or failure modes that the product does not have.
```

For projects using this extension, add the relevant fields to the project
adaptation:

```md
- Applicable UI guidance:
- User-visible consequences:
- State evidence and freshness:
- Partial-failure boundaries:
- Recovery actions and limits:
```
