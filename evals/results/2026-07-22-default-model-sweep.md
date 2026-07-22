# Default-model sweep — gemini-3.6-flash + gemini-3.5-flash-lite vs current default

**Run:** [`results/2026-07-22T03:07:46Z.json`](2026-07-22T03:07:46Z.json) (report:
[`.md`](2026-07-22T03:07:46Z.md)) — 24 cases × 3 models = 72 cells, **0 errors**.
Judged by `claude-opus-4-8` on Vertex, region `global`, project
`caseywest-model-garden`. Phase 2 (faithfulness + citations) live. Same golden
dataset (`dataset/cases.yaml`) as every prior run — identical inputs across models.

**Question:** with `gemini-3.6-flash` and `gemini-3.5-flash-lite` now routable,
should the MCP default change from **`gemini-3.1-flash-lite`**? The decision is
Casey's; below are the numbers behind a recommendation.

## Two axes, kept separate: coverage vs quality

A grounded-search tool is judged on two independent things, and jamming them into
one averaged cell is what made the two 0.90 faithfulness cells look equal in earlier
drafts when they were not comparable at all:

- **Coverage — how often the model actually grounds** (returns sources instead of
  answering from parametric memory). Measured over all 24 cases as the
  ungrounded/0-source rate. This is a comparable, whole-table metric.
- **Quality — how good the answer is *when* it grounds** (faithfulness, citation
  precision/recall/F1). These are *conditional* metrics: they are only defined on a
  grounded answer, so they only exist on the cases a model actually grounded.
  Comparing them head-to-head is valid **only on the cases every model grounded**
  (the common grounded subset) — never over each model's own, variable-size subset.

The quality head-to-head therefore leads with the common-subset table; the
whole-table comparison carries only the genuinely-comparable columns.

## Faithfulness / citation — common grounded subset (the primary quality comparison)

Restricted to the cases **all three models grounded** — the only inputs where all
three are actually scored on these two axes (faithfulness n=10, citation n=9):

| Model | Faithfulness (n=10) | Citation F1 (P / R) (n=9) |
| --- | --- | --- |
| **gemini-3.1-flash-lite** (current default) | **0.934** | 0.476 (0.92 / 0.32) |
| **gemini-3.6-flash** | 0.905 | **0.610 (0.97 / 0.44)** |
| **gemini-3.5-flash-lite** | 0.810 | 0.474 (1.00 / 0.31) |

On identical inputs the current default **leads faithfulness** (0.934 > 0.905 >
0.810). `gemini-3.6-flash`'s one real quality win is citation recall / F1 (F1 0.61
vs 0.48, recall 0.44 vs 0.32): when it grounds, it attaches citations to more of
its source-supported statements. (The citation subset is n=9, not 10: on one common
case a model produced no citation-eligible statements, so its precision/recall is
N/A there.)

These figures are computed by the harness directly from the run JSON — it intersects
the per-case grounded sets automatically and averages each model over that shared
denominator, so the number cannot drift from the raw scores.

## Comparison table — whole-table, comparable metrics only (all 24 cases)

Only the metrics that are genuinely like-for-like across all 24 cases appear here.
The conditional faithfulness/citation columns are deliberately **absent** from this
table — averaging them over each model's own grounded subset compares different
denominators, so they belong in the common-subset table above, not here.

| Model | Ungrounded (0-source) | Relevance | Correctness | Source qual | p50 latency | Grounded tok/s (mean / median) | Avg tokens/query | $/1k queries* |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **gemini-3.1-flash-lite** (current default) | **4/24 (17%)** | 0.91 | 0.82 | **0.59** | 2810 ms | 90 / 89 | **328** | **$0.48** |
| **gemini-3.6-flash** | 13/24 (54%) | 0.92 | **0.84** | 0.35 | 2330 ms | 99 / 81 | 375 | $2.59 |
| **gemini-3.5-flash-lite** | 11/24 (46%) | **0.93** | 0.83 | 0.36 | **2162 ms** | **110** / 79 | 326 | $0.73 |

**The ungrounded rate is a first-order coverage result for a grounded-search tool.**
`gemini-3.6-flash` answered from parametric knowledge with **zero** sources on
13/24 cases (54%) — including `speed-of-light`, `boiling-point-water`,
`largest-ocean`, and `tallest-mountain` — versus 4/24 (17%) for the current default.
For a tool whose entire job is grounded citation, answering half the queries without
searching is the strongest single argument against 3.6-flash. It is also *why* its
conditional quality columns are scored over so few cells — the coverage gap and the
small quality denominator are the same fact seen from two sides, which is exactly
why the two axes are reported separately.

\* Token cost only, at official Vertex Standard-tier Global list prices
(`pricing.go`, captured 2026-07-22). **Not modeled:** the flat grounded-search
surcharge ($35 / 1k grounded prompts beyond the free tier), which is identical per
prompt across all three models and so does not change their relative ranking — but
it dominates absolute cost (it is ~$35/1k vs the ~$0.5–2.6/1k token cost here).

## The two vendor claims — measured, not assumed

**"~350 tok/s for Lite" — NOT reproduced.** Measured end-to-end grounded
throughput for `gemini-3.5-flash-lite` is **~110 tok/s mean, ~79 tok/s median** —
~3× below the 350 claim. Caveat: these are *grounded* search calls, so wall-clock
latency includes Google-Search retrieval and the redirect hop, not pure decode; a
raw-decode benchmark would read higher. Lite *is* the fastest of the three on p50
latency (2162 ms), just not at 350 tok/s in this workload.

**"~17% token efficiency for 3.6-Flash" — NOT reproduced (opposite sign).**
`gemini-3.6-flash` used **more** tokens per query, not fewer: **+14.3%** vs the
current default and **+15.1%** vs `gemini-3.5-flash-lite` (375 vs 328 vs 326 avg
total tokens/query). Combined with 3.6-Flash's higher per-token output rate
($7.50/1M vs $1.50/1M for the current default), its token cost/query is **~5.4×**
the current default. Note: `ThinkingConfig.ThinkingBudget=0` is enforced for all
models (measured thought_tokens = 0 across every cell), so 3.6-Flash's thinking
capability is neutralized in this fast-grounded-search configuration — consistent
methodology, but it means the "efficiency" claim (likely a reasoning-mode figure)
does not transfer to this use case.

## Judge validation (Cohen's κ, this run)

Judge validated against an independent human reference label set for the leading
candidate ([`labels/flash-3.6.yaml`](../labels/flash-3.6.yaml), n=24), bucketed
low/med/high and paired with the judge's bucketed scores:

| Dimension | Cohen's κ | Reading |
| --- | --- | --- |
| relevance | **1.00** | perfect agreement |
| source_quality | **1.00** | perfect agreement |
| correctness | **0.86** | strong — clears the 0.6 trust bar |

All three κ-validated dimensions clear κ > 0.6, so the scores driving the table are
trustworthy. (Command: `go run ./cmd/eval --kappa evals/labels/flash-3.6.yaml
--results evals/results/2026-07-22T03:07:46Z.json --kappa-model gemini-3.6-flash`.)

## Recommendation

**Keep `gemini-3.1-flash-lite` as the default. Consider `gemini-3.6-flash` only
if citation recall is a priority worth ~5× the token cost.**

Reasoning, tied to the numbers:

- **Neither candidate beats the current default on the core quality axes at a
  price worth paying.** On the common grounded subset (identical inputs, n=10) the
  current default **leads faithfulness** — 0.934 vs 3.6-flash 0.905 vs
  3.5-flash-lite 0.810. Relevance/correctness are within noise across all three
  (≤0.03 spread). The current default leads **source quality** outright (0.59 vs
  0.35/0.36) and, on the coverage axis, grounds far more reliably (17% ungrounded
  vs 54%/46%).
- **The one real quality win is 3.6-Flash's citation recall / F1** — on the common
  grounded subset F1 0.61 vs 0.48, recall 0.44 vs 0.32: when it does ground, it
  attaches citations to more of its source-supported statements, narrowing the
  low-recall gap called out in prior runs. But that win applies to the ~46% of
  cases it actually searches; it costs **~5.4× more per query** in tokens (and uses
  ~14% more tokens), and on the common subset it slightly *trails* the default on
  faithfulness (0.905 vs 0.934).
- **`gemini-3.5-flash-lite` is not a compelling swap:** marginally fastest and
  cheapest-ish, but its faithfulness drops to 0.810 (−0.124 vs current on the common
  subset) and it does not lead any quality dimension meaningfully. Faithfulness is
  the metric that matters most for a grounded-search tool, so a regression there is
  disqualifying.
- **Both vendor claims that motivated the eval failed to reproduce in this
  workload** (Lite is ~110 tok/s not 350; 3.6-Flash is ~14% *less* token-efficient
  not 17% more), which removes the speed/cost rationale for switching.

**Bottom line for Casey:** the data does not support changing the default. If the
low-citation-recall behavior becomes a product priority, `gemini-3.6-flash` is the
model that fixes it — at a materially higher token cost and no faithfulness upside.
Decision is yours.
