# Codex token accounting

Codex and Claude Code evidence use disjoint token categories:

- `input_tokens`: base input, excluding cache reads and cache writes.
- `cache_read_input_tokens`: input served from cache.
- `cache_creation_input_tokens`: input written to cache.
- `output_tokens`: output, including reasoning once.
- `total_tokens`: `input_tokens + output_tokens`, excluding both cache fields.

To count all processed tokens, add the two cache fields to `total_tokens`.
Cache hits are still processed input; they have a different price. The field
called `total_tokens` follows the existing Claude evidence convention rather
than representing all processed tokens.

Codex's source `input_tokens` includes both cache categories. Trace subtracts
them once when writing evidence. The original counters remain in the raw
transcript. Cost uses the same disjoint counts as evidence, with separate
prices for base input, cache reads, cache writes, and output. When the pricing
table has no cache-write rate, writes use the base input rate. The estimate
does not establish billed spend.

## Selecting usage records

Trace reads the entire copied rollout before selecting one source:

1. If native `token_usage_record` records exist and every one has a
   `response_id` and per-response `usage`, sum those response usages. Count
   each response ID once; the first valid usage for an ID wins. The model is
   the model in effect at that record. Mirrored events and cumulative
   counters do not contribute.
2. Otherwise, use native `thread_token_usage` counters. If no native usage
   records exist, use `event_msg.token_count.info.total_token_usage` instead.
   Never interleave the two cumulative streams or add their totals together.
   Cumulative snapshots are not deduplicated by response ID: another snapshot
   with the same ID may contain a new increment.
3. For a cumulative source, count only increases from the previous accepted
   snapshot. Repeated snapshots add zero. A decreasing counter leaves the
   previous baseline unchanged and produces a warning; Trace cannot infer a
   genuine reset from that decrease. A missing cumulative snapshot in the
   selected source also produces a warning.

Per-response usage remains valid across compaction, cumulative-counter
resets, and resumed sessions with replayed response IDs. With only cumulative
counters, a real reset may leave usage undercounted until the counters exceed
the previous baseline. The warning makes this uncertainty visible rather than
adding a full session total again. Resuming with the complete transcript and
monotonic counters adds each increment once.

With cumulative counters, each increment belongs to the model at the snapshot;
the first snapshot belongs entirely to that model. Historical model attribution
cannot be recovered if earlier snapshots are absent. Negative counts and cache
categories exceeding source input are rejected with a warning. Reasoning is
already part of source output, so `reasoning_output_tokens` is not added again.

## Regression evidence

The sanitized `testdata/compacted-session.jsonl` fixture retains all 275 native
usage records and 275 mirrored events from issue
[#3706](https://github.com/chainloop-dev/chainloop/issues/3706), including the
compaction boundary. It removes conversation content, replaces response IDs,
and uses a fixed timestamp. Its token counters are unchanged.

The original inclusive response totals are 51,803,136 input and 168,426 output
(51,971,562 combined). Normalized evidence contains:

| Field | Tokens |
| --- | ---: |
| `input_tokens` | 20,425 |
| `cache_read_input_tokens` | 51,113,536 |
| `cache_creation_input_tokens` | 669,175 |
| `output_tokens` | 168,426 |
| `total_tokens` | 188,851 |

The embedded model prices produce an estimated cost of $8.5094. Parsing this
fixture must produce no warnings. The previous parser reported 5,260,816,481
total tokens, $900.8926 estimated cost, and 147 false reset warnings.

## Other provider parsers

This table describes the current code. This fix changes Codex only.

| Provider | Input/cache fields | Total and cost | Duplicate and reasoning handling |
| --- | --- | --- | --- |
| Claude Code | Base input, cache reads, and cache writes are separate. | Total is base input + output. Cost includes cache reads and both 5-minute and 1-hour writes. | Sums each assistant usage record, including subagents, without message-ID deduplication. No separate reasoning count is added. Duplicate usage records can inflate counts and cost. |
| Cursor | The collected transcript supplies no usage. | Usage and cost are unavailable, with a warning. | No token accounting to deduplicate or normalize. |
| Pi parser | Copies input, output, cache reads, and cache writes separately. | Uses the supplied `totalTokens` and `cost.total` without reconciling categories. Its fixture total includes cache. | Rejects duplicate entry IDs and reads only the selected branch. No separate reasoning count is added. |
| OpenCode | Copies input, output, and cache read/write fields separately. | Total is input + output. Prefers session counters/cost over message sums. | Message sums have no ID deduplication. A separate source `reasoning` counter is not represented in the parser's token types. |

Pi's selected-branch scope and source totals differ from a complete session
spend report. OpenCode's separate reasoning counter needs a follow-up decision
and normalization if it is to be included in output. Those behaviors and
Claude's missing deduplication remain outside this Codex fix.

## Existing evidence and display

Codex evidence now sets `data.schema_version` to `v2`. The JSON structure and
schema URL stay the same; the data version identifies the accounting semantics.
Consumers must check both `data.agent.name` and `data.schema_version`:

| Agent and data version | Meaning of input and total | Inclusive processed-token total |
| --- | --- | --- |
| Codex `v1` | Input includes cache; total includes input and output. | Use `total_tokens` without adding the cache fields. Counts may still contain the old overcount. |
| Codex `v2` | Input and total exclude cache, matching the existing Claude convention. | Add both cache fields to `total_tokens` exactly once. |

Other providers retain their existing versions and semantics. An unknown
version requires an explicit compatibility decision; do not infer its meaning
from whether cache fields are present. No new evidence fields are needed.
Consumers must not treat inclusive input as base input for pricing, or aggregate
Codex `v1` and `v2` input/total fields together without normalizing them first.

Previously uploaded attestations are not rewritten. Their inflated counts and
cost estimates remain historical evidence and must not be interpreted as billed
spend. Re-parsing a complete stored rollout with the fixed CLI recomputes counts
and cost; replacing an uploaded record requires the normal re-attestation path.
An old Codex aggregate alone is insufficient to recover the correct counts.
