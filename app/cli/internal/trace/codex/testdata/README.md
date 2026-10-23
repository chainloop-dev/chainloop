`native-session.jsonl` contains selected records from a Codex CLI 0.163.0
rollout. Instructions, prompts, tool inputs and outputs, paths, IDs, and times
were removed or replaced. The native usage counters and record shapes remain.
It includes both `token_usage_record` and `event_msg/token_count` records for
the same usage, which must count once.

`session.jsonl` is a small constructed fixture for predictable totals, repeated
cumulative counters, and duplicate conversation events.

`compacted-session.jsonl` retains only the usage events and compaction marker
from the artifact in issue #3706 (digest
`sha256:d729cade5c9f2c7eef4ca720290133db119e291445a70e21175ef4901f819391`).
Its 275 native response records and 275 mirrored legacy events keep their
original token counters. Response IDs are replaced, timestamps are fixed,
and no conversation content is retained. The test supplies a synthetic
session header and model context. The streams diverge after compaction;
the previous parser counted 5,260,816,481 tokens and emitted 147 reset warnings.
Unique responses instead account for 51,971,562 inclusive tokens. See
../TOKEN_ACCOUNTING.md for normalized evidence totals.
