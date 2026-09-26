# cc-monitors

Monitors for [Claude Code](https://claude.com/claude-code)'s `Monitor` tool.
Each subcommand polls something and emits line-delimited JSON events on
stdout, one per line: `{"event": "...", "data": {...}}`. Diagnostic/retry
logging goes to stderr instead, since `Monitor` only wakes Claude on stdout
lines.

## Install

See [docs/install.md](docs/install.md).

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

| event         | when                             | data               | exit |
| ------------- | -------------------------------- | ------------------ | ---- |
| `new_comment` | a new comment or review appeared | `kind`, `id`, `api` | —    |
| `pr_closed`   | PR merged or closed              | `merged`           | 0    |

Events carry no content: read the latest state with `gh api <api>`. `kind` is
`comment` (conversation), `review` or `review_comment` (inline).

```json
{"data":{"kind":"review_comment","id":1583153997,"api":"https://api.github.com/repos/cli/cli/pulls/comments/1583153997"},"event":"new_comment"}
{"data":{"merged":true},"event":"pr_closed"}
```

After 5 consecutive `gh` failures, the process exits non-zero with the
reason on stderr.

## Development

```sh
go test ./...
golangci-lint run
golangci-lint fmt
```
