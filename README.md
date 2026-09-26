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

## `pr-comments`

Watches a PR for new conversation comments, reviews and inline review
comments, until the PR is merged or closed (exit 0). Comments that already
exist at startup are not reported, nor are your own (so Claude's replies via
`gh` don't wake itself). Reviews without a body in the `COMMENTED` state are
skipped, since their inline comments are reported individually.

### Requirements

- [`gh`](https://cli.github.com/) authenticated (`gh auth login`)

### Usage

```
cc-monitors pr-comments <pr | url | branch> [--interval DURATION]
```

`--interval` takes a Go duration (e.g. `30s`, `1m`) and defaults to `30s`.

### Prompt

```
After opening a PR, watch for review feedback: Monitor({ command: "cc-monitors pr-comments <pr>", description: "PR <pr> comments", persistent: true })
```

### Events

| event            | when                         | data                                                     | exit |
| ---------------- | ---------------------------- | -------------------------------------------------------- | ---- |
| `comment`        | new conversation comment     | `author`, `body`, `url`                                  | —    |
| `review`         | new review submitted         | `author`, `state`, `body`, `url`                         | —    |
| `review_comment` | new inline review comment    | `author`, `body`, `url`, `path`, `line`, `in_reply_to`?  | —    |
| `pr_closed`      | PR merged or closed          | `merged`                                                 | 0    |

`body` is truncated to 1000 characters; follow `url` for the rest.

```json
{"data":{"author":"alice","body":"nit: rename this","url":"https://…","path":"main.go","line":42},"event":"review_comment"}
{"data":{"merged":true},"event":"pr_closed"}
```

After 5 consecutive `gh` failures, the process exits non-zero with the
reason on stderr.

## Development

```
go test ./...
golangci-lint run ./...
```
