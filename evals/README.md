# Evaluations

This directory holds an evaluation harness for `gemini-search-mcp` and the golden
dataset it runs on. The point of evals here is simple: the value of a search tool
isn't whether the code runs, it's whether the **answers are good and the sources
are real and relevant**. Unit tests can't tell you that; evals can.

We also use it to answer a concrete product question — **which Gemini model should
be the default?** — with data instead of a guess. The result surprised us (see
below), which is exactly why it was worth measuring.

## What it measures

This tool isn't a classic RAG pipeline: **Google Search is the retriever** (inside
Gemini's grounding), so retriever-tuning metrics like context precision don't
apply. We score the generator and its citations:

- **Relevance** — does the answer address the query, without padding?
- **Correctness** — does it match the facts the case expects?
- **Source quality** — are the returned sources present, on-topic, and reputable?
- plus **operational** cost: p50 latency, token cost, error rate.

Phase 2 adds **faithfulness** and **citation precision/recall** — checking each
claim against the actual fetched source text — plus **judge validation** (Cohen's
κ against human labels) and **regression gating**. See the Phase 2 section below.

## How it's scored: a cross-family judge

Scoring uses an **LLM-as-judge**, with one deliberate choice: the judge is
**Claude (Opus) on Vertex AI**, not Gemini. Letting a model grade its own family's
output invites *self-preference bias*, a well-documented failure mode of
LLM-as-judge setups. Using a different family is the standard mitigation. The judge
prompt forces chain-of-thought before the verdict, penalizes verbosity, and returns
a strict JSON score per dimension.

This is a pragmatic v1, not a published benchmark. Known limitations, stated plainly
so you can weigh the numbers yourself:

- **Modest sample** (24 cases per model, balanced across six categories) — a strong
  signal, not a published-benchmark-scale study.
- **Single judge, single run** — the judge is validated against human labels
  (Cohen's κ, below), but scores are from one judge family and one run, not
  averaged across runs.
- **One judge family** — Claude's tastes are baked into the scores.
- **Correctness is judge-validated** (κ=0.63) — it clears the 0.6 trust bar,
  alongside relevance (κ=1.00) and source quality (κ=0.87) (see κ section below).

## Default-model sweep (2026-07-22)

Latest run re-evaluates the default against the two models that GA'd 2026-07-21
(`gemini-3.6-flash`, `gemini-3.5-flash-lite`) on the same 24-case dataset. Full
comparison table, the two vendor claims measured (not assumed), judge κ, and the
recommendation: **[`results/2026-07-22-default-model-sweep.md`](results/2026-07-22-default-model-sweep.md)**.
Committed run: [`results/2026-07-22T03-07-46Z.json`](results/2026-07-22T03-07-46Z.json).
Headline: the data does not support changing the default — faithfulness ties or
regresses, neither vendor claim reproduced, and the only quality win (3.6-Flash
citation recall) costs ~5.4× the token spend.

## Results (2026-07-12)

24 cases × 3 models, judged by `claude-opus-4-8` on Vertex. Vertex `global` region.
Committed run: [`results/2026-07-12T23-22-55Z.json`](results/2026-07-12T23-22-55Z.json)
(the `.md` alongside it is the rendered report).

| Model | Relevance | Correctness | Source quality | p50 latency | $/1k queries | Errors |
| --- | --- | --- | --- | --- | --- | --- |
| **gemini-3.1-flash-lite** | **0.92** | **0.87** | **0.53** | **2.5 s** | **$0.69** | 0% |
| gemini-3.5-flash | 0.88 | 0.82 | 0.33 | 3.2 s | $2.98 | 0% |
| gemini-3.1-pro-preview | 0.71 | 0.69 | 0.34 | 11.0 s | $49.76 | 0% |

### Faithfulness & citations (Phase 2, live)

Every cell fetched its cited source pages and scored faithfulness (claims checked
against the fetched text) and ALCE-style citation precision/recall.

| Model | Faithfulness | Citation precision | Citation recall | Citation F1 |
| --- | --- | --- | --- | --- |
| **gemini-3.1-flash-lite** | **0.92** | **0.97** | 0.31 | 0.47 |
| gemini-3.5-flash | 0.84 | 0.87 | 0.35 | 0.50 |
| gemini-3.1-pro-preview | 0.78 | 0.86 | 0.38 | 0.53 |

Faithfulness is high across the board (answers' claims are well grounded in the
fetched sources) and citation precision is high, but citation **recall** is low for
all three models — they state many source-supported facts without attaching a
citation to each. That's a real, consistent finding, not noise.

### What we concluded, and why

**The default is `gemini-3.1-flash-lite`.** It leads on relevance and source
quality, ties or beats Flash on correctness and faithfulness, runs fastest, and
costs the least by a wide margin (Pro-preview is ~70× its cost per 1k queries at
these settings). The reason this isn't paradoxical: for grounded search, answer
quality comes mostly from Google Search results, not from the model's raw reasoning
power — so the biggest model mostly adds latency and cost.

**Pro did worst, and we're not over-reading it.** `gemini-3.1-pro-preview` scored
lowest on the quality dimensions and took ~11 s/query. The likely cause is our
config: we disable "thinking" (`ThinkingBudget=0`) for speed, and the preview Pro
reasoning model seems to interact badly with that. The honest takeaway isn't "Pro is
bad" — it's "Pro is the wrong tier for fast grounded search, at least with this
configuration."

**Where the sample is honest about its limits:** source quality is weakest on
`how-to` (0.18) and `ambiguous` (0.26) cases. Gemini often answers easy or opinion
questions from its own knowledge *without* searching, returning zero sources — fine
for a how-to, more of a gap for anything that should be grounded. The ambiguous
cases also expose a real behavior: on bare terms like "python" or "jaguar" the model
sometimes commits to one meaning instead of flagging the ambiguity.

## Run it yourself

The eval makes live, paid API calls, so it's gated behind `RUN_EVALS=1` and never
runs in CI. With Vertex configured (ADC / service-account credentials):

```bash
RUN_EVALS=1 \
GOOGLE_GENAI_USE_VERTEXAI=true \
GOOGLE_CLOUD_PROJECT=your-project \
GOOGLE_CLOUD_LOCATION=global \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa-key.json \
go run ./cmd/eval
```

Flags: `--dataset` (default `evals/dataset/cases.yaml`), `--models` (comma-separated),
`--out` (results dir, default `./eval-results`, gitignored). Results are written as
timestamped `.md` + `.json`.

The judge model defaults to `claude-opus-4-8` (override with `EVAL_JUDGE_MODEL`);
it needs Claude available on your Vertex project. Pricing in `pricing.go` is
approximate (as of 2026-06) — update it for accurate cost figures.

## Phase 2: faithfulness, citations, judge validation, regression gating

Phase 1 asks "does the answer look good?" Phase 2 asks the harder question:
"**are the answer's claims actually in the cited sources?**" Answering it means
leaving the model's word for it and fetching the real publisher pages.

### Faithfulness (groundedness)

Two judge calls per answer:

1. **Claim decomposition** — break the answer into atomic, self-contained
   factual claims.
2. **Claim classification** — for each claim, check it against the *fetched
   source text* (not the judge's own knowledge) and label it `supported`,
   `partial`, or `unsupported`.

Score = (supported + 0.5·partial) / total claims. A low faithfulness score with a
high correctness score is the interesting signal: the answer is right, but it's
right from the model's memory rather than from what it cited.

### Citation precision / recall (ALCE-style)

- **Precision** = fraction of the answer's *cited* sentences that their cited
  sources actually entail. Penalizes citations that don't support the sentence.
- **Recall** = fraction of the source-supported statements that the answer
  actually states. Penalizes leaving supported facts uncited.
- **F1** is the harmonic mean, reported alongside.

Both are judge-based, one call each, scored against the fetched source text.

### Source fetching

The Vertex grounding URIs are redirect shims, not publisher URLs. The fetcher
(`internal/eval/fetch.go`) follows redirects to the real page, strips HTML to
readable text, and caps the size (default 60 KB) to bound prompt cost. Fetches
are best-effort: a source that 403s or times out is skipped, not fatal.

### How to run Phase 2

Add `--phase2` to the command above. It fetches every cited source and makes the
extra judge calls, so it's slower and costs more than Phase 1:

```bash
RUN_EVALS=1 \
GOOGLE_GENAI_USE_VERTEXAI=true \
GOOGLE_CLOUD_PROJECT=your-project \
GOOGLE_CLOUD_LOCATION=global \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa-key.json \
go run ./cmd/eval --phase2
```

The report gains a **Faithfulness & citations** table; the JSON carries the
per-claim verdicts and per-sentence labels for inspection.

### Judge validation with Cohen's κ

An LLM judge is only trustworthy once it agrees with humans. We validate the judge
against a human reference label set for one model's answers
([`labels/flash-lite.yaml`](labels/flash-lite.yaml)), which rates every
`gemini-3.1-flash-lite` cell in the committed run on `relevance` / `correctness` /
`source_quality`, bucketed `low`/`med`/`high`. The file header documents exactly how
the labels were produced (the rubric, and the human sign-off that makes them the
reference). The judge's `[0,1]` scores are bucketed the same way and paired with the
human labels to compute Cohen's κ per dimension:

```bash
go run ./cmd/eval --kappa evals/labels/flash-lite.yaml \
  --results evals/results/2026-07-12T23-22-55Z.json --kappa-model gemini-3.1-flash-lite
```

This is offline (no API calls). For the committed run (n=24 per dimension):

| Dimension | Cohen's κ | Reading |
| --- | --- | --- |
| relevance | **1.00** | perfect agreement |
| source_quality | **0.87** | strong agreement |
| correctness | **0.63** | moderate-to-strong — clears our 0.6 trust bar |

All three dimensions clear the **κ > 0.6** bar and can be trusted. The residual
correctness disagreements are substantive, not random: the judge is more lenient on
a possibly-stale version string, and stricter on an answer that padded a correct fact
with extra claims. The practical
consequence: **gate regressions on all three κ-validated dimensions**. Re-check κ
whenever the judge model version changes (calibration drift).

To validate another model or grow the label set, add a `labels/<model>.yaml` in the
same format:

```yaml
- case_id: go-latest-version   # must match an id in dataset/cases.yaml
  scores:
    relevance: high            # low | med | high
    correctness: high
    source_quality: med
```

### Regression gating

Commit a **baseline summary** (per-model dimension averages) and diff future runs
against it. Generate one with `--write-baseline`, then gate later runs with
`--baseline`:

```bash
# refresh the committed baseline from a known-good run
go run ./cmd/eval --phase2 --write-baseline evals/baseline.json

# later: fail the run (exit non-zero) if any dimension drops > threshold
go run ./cmd/eval --phase2 --baseline evals/baseline.json --threshold 0.05
```

A model present in the baseline but missing from the run (e.g. it errored out
entirely) is also flagged. Per the design, gate only on κ-validated dimensions —
here that means **all three dimensions** clear **κ > 0.6** (relevance 1.00,
source_quality 0.87, correctness 0.63), so all are trustworthy enough to gate on.
A committed baseline generated from the current run lives at
[`baseline.json`](baseline.json).

## Add your own cases

Edit `dataset/cases.yaml`. Each case:

```yaml
- id: short-unique-id
  query: "the question to search"
  category: factual          # factual | temporal | how-to | multi-hop | ambiguous | no-good-answer
  expect_assertions:         # facts the answer should contain (judged, not string-matched)
    - "what a good answer must say"
  expect_domains: ["example.com"]   # optional: sources you'd expect
  notes: "anything the judge should know"
```

Negative cases matter: include `no-good-answer` queries (the tool should say it
doesn't know, not hallucinate) and `ambiguous` ones. They catch the failures that
"happy path" cases miss.
