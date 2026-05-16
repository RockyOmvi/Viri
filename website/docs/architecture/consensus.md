# Consensus

Viri uses **HotStuff BFT**, a leader-based Byzantine Fault Tolerance consensus protocol with linear message complexity.

## Protocol Phases

Each round (view) has a leader who drives consensus through four phases:

1. **Prepare**: Leader proposes a block → validators respond with votes
2. **Pre-commit**: Leader collects PrepareQC → validators pre-commit
3. **Commit**: Leader collects PreCommitQC → validators commit
4. **Decide**: Leader collects CommitQC → validators finalize

## View Change

When a leader fails (timeout), validators trigger a view change:

1. Validators send timeout messages with their latest QC
2. New leader collects `2f+1` timeout messages
3. New leader starts the next view with a safe proposal

## Leader Rotation

The leader for each view is deterministically selected from the active validator set using round-robin rotation.

## Key Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| Block Time | 2.5s | Target time between blocks |
| View Timeout | 3s | Time before leader rotation |
| Validators | 4 | Minimum for production |
| Max Block Size | 1MB | Maximum block size |
