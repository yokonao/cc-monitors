# cc-monitors

Monitors for [Claude Code](https://claude.com/claude-code)'s `Monitor` tool.
Each subcommand polls something and emits line-delimited JSON events on
stdout, one per line: `{"event": "...", "data": {...}}`. Diagnostic/retry
logging goes to stderr instead, since `Monitor` only wakes Claude on stdout
lines.

## Install

```
go install github.com/yokonao/cc-monitors/cmd/cc-monitors@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/yokonao/cc-monitors/releases) (linux/darwin, amd64/arm64):

```
curl -sSL "https://github.com/yokonao/cc-monitors/releases/latest/download/cc-monitors_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" \
  | tar xz -C /usr/local/bin cc-monitors
```

## `pr-ci`

Watches a PR's CI checks. Green is the only terminal state (exit 0); a red
run keeps watching, since a fix push starts a new run.

### Requirements

- [`gh`](https://cli.github.com/) authenticated (`gh auth login`)

### Usage

```
cc-monitors pr-ci <pr | url | branch> [--interval DURATION]
```

`--interval` takes a Go duration (e.g. `30s`, `1m`) and defaults to `30s`.

### Prompt

Add one line to your `CLAUDE.md`:

```
After a PR or fix push, watch CI: Monitor({ command: "cc-monitors pr-ci <pr>", description: "PR <pr> CI", persistent: true })
```

### Events

| event           | when                                | data          | exit |
| --------------- | ----------------------------------- | ------------- | ---- |
| `check_failed`  | a check went red (once per failure) | `name`, `url` | —    |
| `checks_passed` | all checks settled green            | `total`       | 0    |

```json
{"data":{"name":"test","url":"https://…"},"event":"check_failed"}
{"data":{"total":3},"event":"checks_passed"}
```

After 5 consecutive `gh` failures, the process exits non-zero with the
reason on stderr.

## Development

```
go test ./...
golangci-lint run ./...
```
