# Changelog

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
