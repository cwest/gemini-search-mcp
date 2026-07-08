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

- **Small sample** (12 cases per model) — results are directional, not definitive.
- **Single judge, single run** — the κ-validation machinery exists (Phase 2) but
  the shipped human labels are placeholders, so the judge isn't validated yet;
  no averaging across runs.
- **One judge family** — Claude's tastes are baked into the scores.

Treat the numbers as a strong signal, not gospel. The harness is here so you can run
your own cases and form your own view.

## Results (2026-06-14)

12 cases × 3 models, judged by `claude-opus-4-8` on Vertex. Vertex `global` region.

| Model | Relevance | Correctness | Source quality | p50 latency | $/1k queries | Errors |
| --- | --- | --- | --- | --- | --- | --- |
| **gemini-3.1-flash-lite** | **0.92** | **0.88** | **0.60** | **3.0 s** | **$0.13** | 0% |
| gemini-3.5-flash | 0.92 | 0.88 | 0.47 | 3.9 s | $0.98 | 0% |
| gemini-3.1-pro-preview | 0.59 | 0.54 | 0.20 | 20.7 s | $2.81 | 0% |

### What we concluded, and why

**The default is `gemini-3.1-flash-lite`.** It ties Flash on relevance and
correctness, beats it on source quality, runs faster, and costs roughly **7.5×
less**. The reason this isn't paradoxical: for grounded search, answer quality comes
mostly from Google Search results, not from the model's raw reasoning power — so the
biggest model mostly adds latency and cost.

**Pro did worst, and we're not over-reading it.** `gemini-3.1-pro-preview` scored
lowest and took ~20 s/query. The likely cause is our config: we disable "thinking"
(`ThinkingBudget=0`) for speed, and the preview Pro reasoning model seems to
interact badly with that (the 20 s latency suggests thinking wasn't actually
suppressed). The honest takeaway isn't "Pro is bad" — it's "Pro is the wrong tier
for fast grounded search, at least with this configuration."

**A real limitation worth seeing:** source quality is low on `factual` (0.27) and
`how-to` (0.05) cases. Gemini often answers easy questions from its own knowledge
*without* searching, returning zero sources. That's fine for how-to, more concerning
for factual lookups — and a good argument for the planned faithfulness work.

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

An LLM judge is only trustworthy once it agrees with humans. The workflow:

1. Run an eval and keep the results JSON (written to `--out`).
2. Hand-score the same cases on each dimension, bucketed `low`/`med`/`high`, in a
   labels YAML (`evals/labels/example.yaml` documents the format).
3. Compute Cohen's κ per dimension — the judge's `[0,1]` scores are bucketed the
   same way and paired with your labels:

   ```bash
   go run ./cmd/eval --kappa evals/labels/your-labels.yaml \
     --results eval-results/<timestamp>.json --kappa-model gemini-3.1-flash-lite
   ```

   This is offline (no API calls); it prints κ and the paired-sample count per
   dimension.
4. Trust the judge on a dimension only once **κ > 0.6**. Re-check whenever the
   judge model version changes (calibration drift).

`evals/labels/example.yaml` ships with **clearly-marked placeholder rows** so the
format and loader are exercised — those rows are fake and must be replaced with
real human judgments before any κ number means anything.

Labels format:

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
entirely) is also flagged. Per the design, only enable gating once the judge is
κ-validated — otherwise you're gating on noise.

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
