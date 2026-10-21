`native-session.jsonl` contains selected records from a Codex CLI 0.163.0
rollout. Instructions, prompts, tool inputs and outputs, paths, IDs, and times
were removed or replaced. The native usage counters and record shapes remain.
It includes both `token_usage_record` and `event_msg/token_count` records for
the same usage, which must count once.

`session.jsonl` is a small constructed fixture for predictable totals, repeated
cumulative counters, and duplicate conversation events.
