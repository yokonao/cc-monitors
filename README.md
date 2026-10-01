# cc-monitors

Monitors for [Claude Code](https://claude.com/claude-code). Each monitor
polls something and emits line-delimited JSON events on stdout, one per line:
`{"event": "...", "data": {...}}`. Diagnostic/retry logging goes to stderr
instead, since `Monitor` only wakes Claude on stdout lines.

Run a monitor under the `Monitor` tool for events that settle within its
deadline, such as CI. For events of unknown timing, such as review comments,
use [`relay`](#relay-experimental) to deliver them to a session regardless of
the deadline and of whether the session is still running.

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
After a PR or fix push, watch CI: Monitor({ command: "cc-monitors pr-ci <pr>", description: "PR <pr> CI", timeout_ms: 1800000 })
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
After opening a PR, watch for review feedback: Monitor({ command: "cc-monitors pr-comments <pr>", description: "PR <pr> comments", timeout_ms: 1800000 }), re-arming it when it expires
```

Since Claude Code 2.1.271, every `Monitor` watch expires after at most 30
minutes and Claude is asked to re-arm it. Reviews can arrive hours later, so
re-arming spends turns while nothing happens, and the watch stops with the
session.

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

## `relay` (experimental)

Runs monitors in a long-lived process and relays their events to the Claude
Code session that registered them. A session can end its turn right after
registering; when an event arrives, the session is woken by `SendMessage` if
it is running, or resumed in the background if it is stopped.

### Requirements

- `relay serve` running. Starting it and keeping it running (launchd, a
  supervisor, a terminal tab, ...) is up to you
- Whatever each monitor needs (e.g. `gh` authenticated) in the environment of
  `relay serve`. Monitors inherit its environment, so setup that exists only
  in your interactive shell is not available to them
- From inside the Claude Code sandbox, the socket allowed in your settings:

  ```json
  { "sandbox": { "network": { "allowUnixSockets": ["/Users/you/.local/state/cc-monitors/relay.sock"] } } }
  ```

`relay serve` keeps using the binary it was started with. Restart it after
updating cc-monitors.

### Usage

```
cc-monitors relay serve                                                   # start the relay process
cc-monitors relay add [--session <id>] <monitor> <target> [monitor flags]  # print the relay ID
cc-monitors relay rm  <relay-id>
cc-monitors relay ls  [--session <id>]
```

`relay add` takes everything from the monitor name onward as the monitor's
command line, run in the caller's cwd. Registering the same session, monitor
and target again updates the monitor flags and prints the existing relay ID.
`--session` takes a session ID or the short `id` from `claude agents`, and
defaults to the calling session (`$CLAUDE_CODE_SESSION_ID`).

The socket and state live in `$XDG_STATE_HOME/cc-monitors/`
(`~/.local/state/cc-monitors/` if unset). Every `relay` command accepts
`--socket` to use another socket path.

### Prompt

```
After opening a PR, relay review feedback: run `cc-monitors relay add pr-comments <pr>`
```

### Notices

Each event line is delivered as-is, prefixed with the monitor and target:

```
[cc-monitors relay] pr-comments https://github.com/o/r/pull/1: {"event":"new_comment","data":{...}}
[cc-monitors relay] pr-comments https://github.com/o/r/pull/1: watch finished.
[cc-monitors relay] pr-comments https://github.com/o/r/pull/1: watch was interrupted. Check for events during the gap.
[cc-monitors relay] pr-comments https://github.com/o/r/pull/1: relay removed after repeated monitor failures.
```

- When the monitor exits 0, the session is told the watch finished and the
  relay is removed
- When the monitor fails, it is restarted with a backoff from 1 minute up to
  30 minutes, and the session is told to check for events missed during the
  gap. After 10 consecutive failures the relay is removed
- When `relay serve` restarts, every restored session is told to check for
  events missed while it was down
- A failed delivery is retried every minute, up to 30 times, then dropped

### Limitations

Delivery relies on undocumented Claude Code behavior, which updates may break:

- The output of `claude agents --json`
- Resuming in the background with `claude --bg --resume`
- `SendMessage` from outside Claude Code: a headless `claude -p` session
  sends it on relay's behalf

`relay` follows Claude Code updates, will switch to an official mechanism
once one exists, and may be dropped if it can no longer keep up.

`SendMessage` addresses a session by name, so delivery to a running session
fails while another session has the same name.

## Development

```sh
go test ./...
golangci-lint run
golangci-lint fmt
```
