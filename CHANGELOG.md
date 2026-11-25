# Changelog

## v0.8.1 - 2026-11-25

### Changed

- `relay` runs its headless `claude -p` with `--no-session-persistence`, so deliveries no longer leave a transcript under `~/.claude/projects` or show up in `claude --resume`.

## v0.8.0 - 2026-11-23

### Changed

- `relay` delivers events through a plugin it writes to a temporary directory for each delivery, whose slash command sends the message by session ID without a model call, instead of having a headless Haiku session call `SendMessage`. Delivery takes well under a second, no longer runs `claude agents`, and works while another session has the same name.

## v0.7.0 - 2026-11-15

### Added

- `exec` runs a command you provide as a monitor and forwards the `{"event": ..., "data": {...}}` lines it prints. Under `relay`, only commands listed in `$XDG_CONFIG_HOME/cc-monitors/exec.allow` can be added.
- `relay` sets `CC_MONITORS_LAST_EVENT` to the last event it forwarded when it restarts a failed monitor.

### Changed

- `relay` no longer resumes a stopped session with `claude --bg --resume`. An event for a session missing from `claude agents` is retried and then dropped like any failed delivery, because the listing can miss a running session and resuming it forked the session.

## v0.6.2 - 2026-11-07

### Changed

- When `claude` fails without printing to stderr, `relay`'s error quotes the last 300 bytes of its stdout after the exit status. Since Claude Code 2.1.319, `claude -p` in auto mode exits 1 with the reason on stdout when it refused an action for lack of a safety check.

## v0.6.1 - 2026-11-04

### Added

- `relay serve` logs what `claude` prints on stderr even when the call succeeds, such as the warning before a claude.ai sign-in expires (Claude Code 2.1.318 and later). Each distinct line is logged once per process.

## v0.6.0 - 2026-10-15

### Added

- `relay add --session` accepts a session name, or a unique part of one, as `claude attach` does.

### Fixed

- `relay` retries a message the receiving session held or did not deliver (Claude Code 2.1.271 and later) instead of counting it as delivered.

## v0.5.1 - 2026-10-13

### Added

- Monitors report `poll still running after <duration>` on stderr every minute while a poll has not returned.

### Fixed

- A monitor no longer hangs past its fetch timeout when a process spawned by `gh` keeps its output open. The same applies to `relay`'s calls to `claude`.

## v0.5.0 - 2026-10-12

### Changed

- **Breaking:** `pr-ci`, `pr-comments` and `pr-merge` accept only a PR URL (`https://github.com/<owner>/<repo>/pull/<number>`). A number or branch was resolved against the working directory's repository and could watch the wrong PR. Pass the URL instead, and re-add relays that used a number or branch.
- Requires Go 1.27.2 or later.

## v0.4.0 - 2026-10-03

### Added

- `pr-merge` watches a PR until it is merged or closed.

## v0.3.0 - 2026-10-02

### Added

- `--version` prints the version.
- `relay` (experimental) runs monitors in a long-lived process and relays their events to Claude Code sessions.

## v0.2.1 - 2026-09-26

### Added

- Build provenance attestations for the release archives. See [docs/install.md](docs/install.md).

## v0.2.0 - 2026-09-26

### Added

- `pr-comments` watches a PR for new comments and reviews until it is merged or closed.

## v0.1.1 - 2026-09-06

### Changed

- Requires Go 1.27.1 or later.

## v0.1.0 - 2026-09-05

Initial release with `pr-ci`.
