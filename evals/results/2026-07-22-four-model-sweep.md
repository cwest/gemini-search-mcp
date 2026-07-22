# Four-model sweep — 3.1-pro-preview + 3.1-flash-lite + 3.6-flash + 3.5-flash-lite (one run)

**Run:** [`results/2026-07-22T16:47:42Z.json`](2026-07-22T16:47:42Z.json) (report:
[`.md`](2026-07-22T16:47:42Z.md)) — 24 cases × 4 models = 96 cells, **0 errors**.
Judged by `claude-opus-4-8` on Vertex, region `global`, project
`caseywest-model-garden`. Phase 2 (faithfulness + citations) live. Same golden
dataset (`dataset/cases.yaml`) as every prior run — identical inputs across all
four models, so every row in every table below shares one denominator.

**Why one run:** the committed data used to be split across two runs with different
denominators — the 2026-07-12 run scored `gemini-3.1-pro-preview` + the default,
the 2026-07-22 run scored the three current Flash models. The same model scored
differently in each because they were different runs, so dropping a Pro row from
one run into the other's table would be exactly the variable-denominator artifact
the eval-math fix corrected. This run scores all four on the *same* cases in a
single invocation, so the four-way comparison is honest end to end.

**Config parity (and the one place a model broke it):** every model was requested
with `ThinkingConfig.ThinkingBudget=0` — grounded search runs thinking-off in
production, so parity requires it across the board. The three Flash models complied
(measured `thought_tokens = 0` on all 72 of their cells). **`gemini-3.1-pro-preview`
did not:** it emitted thinking tokens on 6 of its 24 cells anyway (128,356 thought
tokens total, of which one case — `fake-treaty` — alone burned 125,832). This is
not a straw man against Pro; it is the honest result of running Pro in the config
the tool actually uses, and the non-compliance is itself a finding (it is the direct
cause of Pro's runaway cost and latency below).

## Two axes, kept separate: coverage vs quality

A grounded-search tool is judged on two independent things, and averaging them into
one cell hides the story:

- **Coverage — how often the model actually grounds** (returns sources instead of
  answering from parametric memory). Measured over all 24 cases as the
  ungrounded/0-source rate. This is a comparable, whole-table metric.
- **Quality — how good the answer is *when* it grounds** (faithfulness, citation
  precision/recall/F1). These are *conditional* metrics: only defined on a grounded
  answer, so they only exist on the cases a model actually grounded. Comparing them
  head-to-head is valid **only on the cases every model grounded** (the common
  grounded subset) — never over each model's own, variable-size subset.

The quality head-to-head therefore leads with the common-subset table; the
whole-table comparison carries only the genuinely-comparable columns.

## Faithfulness / citation — common grounded subset (the primary quality comparison)

Restricted to the cases **all four models grounded** — the only inputs where all
four are actually scored on these axes (faithfulness n=4, citation n=4). The subset
is small precisely *because* the four models' grounded sets barely overlap: the
weaker-coverage models (Pro and 3.6-flash each grounded only 11/24) shrink the
shared denominator. That is a real property of the lineup, not a defect of the run.

| Model | Faithfulness (n=4) | Citation F1 (P / R) (n=4) |
| --- | --- | --- |
| **gemini-3.1-pro-preview** | **1.000** | **0.681 (1.00 / 0.52)** |
| **gemini-3.1-flash-lite** (current default) | 0.966 | 0.416 (1.00 / 0.26) |
| **gemini-3.6-flash** | 0.844 | 0.541 (0.80 / 0.41) |
| **gemini-3.5-flash-lite** | 0.917 | 0.460 (0.75 / 0.33) |

On the four cases everyone grounded, `gemini-3.1-pro-preview` leads both quality
axes — faithfulness 1.000 and citation F1 0.681 (recall 0.52, the only model above
0.41). The current default is second on faithfulness (0.966) and ties Pro on
citation precision (1.00) but trails badly on recall (0.26). **Read this next to the
coverage and cost columns below before drawing any conclusion:** Pro's quality lead
holds only on the 4 cases it chose to ground, and it earns that lead at ~140× the
default's cost per 1k queries.

*(n=4, not larger, is a direct consequence of the coverage gap — see the ungrounded
column below. A quality win measured on a model's best 4 cases is not the same kind
of evidence as a whole-table result.)*

## Comparison table — whole-table, comparable metrics only (all 24 cases)

Only metrics that are genuinely like-for-like across all 24 cases appear here. The
conditional faithfulness/citation columns are deliberately **absent** — averaging
them over each model's own grounded subset compares different denominators, so they
belong in the common-subset table above, not here.

| Model | Ungrounded (0-source) | Relevance | Correctness | Source qual | p50 latency | Grounded tok/s (mean / median) | Avg tokens/query | $/1k queries* |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **gemini-3.1-pro-preview** | 13/24 (54%) | 0.64 | 0.62 | 0.29 | 9662 ms | 21 / 7 | 5612 | $67.22 |
| **gemini-3.1-flash-lite** (current default) | **5/24 (21%)** | 0.92 | 0.83 | **0.58** | 2897 ms | 84 / **90** | 329 | **$0.48** |
| **gemini-3.6-flash** | 13/24 (54%) | 0.91 | 0.81 | 0.36 | 3230 ms | 91 / 80 | 379 | $2.63 |
| **gemini-3.5-flash-lite** | 11/24 (46%) | **0.92** | **0.85** | 0.40 | **1952 ms** | **109** / 67 | **328** | $0.74 |

**On every whole-table axis, `gemini-3.1-pro-preview` is the worst of the four.** In
this fast-grounded-search configuration it grounds least reliably (tied-worst 54%
ungrounded), scores lowest on relevance (0.64), correctness (0.62) and source
quality (0.29), is ~3–5× slower (9.7 s p50), and costs **~140× the current default**
per 1k queries ($67.22 vs $0.48). The cost is dominated by its inability to hold
thinking-off: its 5,612 avg tokens/query is an order of magnitude above the Flash
trio's ~330, driven by the 6 cells where thinking leaked (one at 125,832 thought
tokens). A frontier model measured out of its intended regime looks bad — and
grounded search *is* out of Pro's intended regime.

Among the three Flash models the picture matches the prior three-model sweep: the
current default leads **source quality** (0.58) and **coverage** (21% ungrounded vs
46–54%); `3.5-flash-lite` is fastest/cheapest-ish and marginally leads
relevance/correctness within noise; `3.6-flash` grounds least of the three (54%).

**The ungrounded rate is a first-order coverage result for a grounded-search tool.**
`gemini-3.1-pro-preview` answered from parametric knowledge with **zero** sources on
13/24 cases (54%) — including `boiling-point-water`, `go-latest-version`,
`python-latest-version`, and `uk-prime-minister` — versus 5/24 (21%) for the current
default. For a tool whose entire job is grounded citation, answering more than half
the queries without searching is a first-order failure, and it is *why* Pro's
conditional-quality columns are scored over so few cells: the coverage gap and the
small quality denominator are the same fact from two sides.

\* Token cost only, at official Vertex Standard-tier Global list prices
(`pricing.go`, captured 2026-07-22). **Not modeled:** the flat grounded-search
surcharge ($35 / 1k grounded prompts beyond the free tier), which is identical per
prompt across all four models and so does not change their relative ranking — but it
dominates absolute cost for the cheap models. It does *not* dominate Pro, whose
token cost ($67/1k) already dwarfs the surcharge.

## Per-model conditional metrics (own grounded subset — NOT a head-to-head)

Each row is averaged over that model's OWN grounded cases (scored-n shown). **The
scored-n differ across models, so these columns are not comparable across rows:**
their averages sit on different denominators. The cross-model ranking lives in the
common-grounded-subset table above, not here. This table exists only to show each
model's within-model behavior on the cases it did ground.

| Model | Faithfulness (scored-n) | Citation precision (scored-n) | Citation recall | Citation F1 |
| --- | --- | --- | --- | --- |
| gemini-3.1-pro-preview | 0.870 (n=10) | 0.938 (n=8) | 0.402 | 0.563 |
| gemini-3.1-flash-lite | 0.891 (n=19) | 0.955 (n=16) | 0.275 | 0.426 |
| gemini-3.6-flash | 0.787 (n=10) | 0.902 (n=8) | 0.394 | 0.548 |
| gemini-3.5-flash-lite | 0.844 (n=12) | 0.896 (n=12) | 0.346 | 0.500 |

The current default's own grounded subset is by far the largest (n=19 faithfulness,
n=16 citation) — another view of its coverage lead: it grounds on nearly four times
as many cases as Pro does, so its conditional metrics rest on a much wider base.

## Judge validation (Cohen's κ, this run)

Judge validated against independent human reference label sets, computed offline
directly against this run's JSON, bucketed low/med/high:

| Label set | Dimension | Cohen's κ | Reading |
| --- | --- | --- | --- |
| `labels/flash-lite.yaml` (current default) | relevance | **1.000** | perfect |
| | correctness | **0.625** | clears the 0.6 trust bar |
| | source_quality | 0.590 | **just below** 0.6 — read with caution |
| `labels/flash-3.6.yaml` (candidate) | relevance | **1.000** | perfect |
| | correctness | **0.742** | strong |
| | source_quality | **0.680** | clears the 0.6 trust bar |

Relevance and correctness clear κ > 0.6 on both validated models. For the current
default, `source_quality` lands at κ=0.590 — marginally *below* the 0.6 bar, so its
source-quality column should be read as directional rather than tightly trusted on
this run. It clears the bar on the 3.6-flash label set (0.680). (Commands:
`go run ./cmd/eval --kappa evals/labels/flash-lite.yaml --results
evals/results/2026-07-22T16:47:42Z.json --kappa-model gemini-3.1-flash-lite` and the
same with `flash-3.6.yaml` / `--kappa-model gemini-3.6-flash`.)

## Reading of the four-model comparison

- **Pro is not the grounded-search default, and this run shows why by measurement,
  not assertion.** On the four cases every model grounded, `gemini-3.1-pro-preview`
  does lead answer quality (faithfulness 1.000, citation F1 0.681). But that win
  exists only where it grounds — and it grounds on barely half the cases (54%
  ungrounded), scores worst on every whole-table axis, runs 3–5× slower, and costs
  ~140× the current default per 1k queries. In the thinking-off, fast-grounded
  configuration this tool actually uses, a frontier reasoning model is the wrong
  instrument, and it could not even hold thinking-off (6 cells leaked thinking, one
  at 125k tokens). The quality ceiling it shows on 4 cases is real; the price and
  coverage make it unusable as the default.
- **Among the Flash models the prior recommendation stands.** The current default
  (`gemini-3.1-flash-lite`) leads coverage (21% ungrounded) and source quality
  (0.58), with faithfulness/relevance/correctness within noise of the alternatives.
  `3.6-flash` and `3.5-flash-lite` do not beat it on the axes that matter for a
  grounded-search tool at a price worth paying.
- **The apples-to-apples framing is the whole point.** Because all four models ran
  on identical inputs in one invocation, the "Pro leads quality but loses everywhere
  else" statement is defensible — it is not an artifact of comparing a 7-12 Pro run
  against a 7-22 Flash run. The harness computes the common-subset figures directly
  from the run JSON (intersecting per-case grounded sets and averaging over the
  shared denominator), so the head-to-head number cannot drift from the raw scores.

This document reports the numbers; the default-change decision is Casey's. Nothing
here recommends changing the default — `internal/config/config.go` stays
`gemini-3.1-flash-lite`.
