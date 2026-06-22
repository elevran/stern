# stern

stern is a GitHub bot that manages pull requests via slash commands. It handles review labels, blocking labels, and GitHub native auto-merge based on PR eligibility.

- [Quickstart](docs/quickstart.md) — deploy stern to your repo in five minutes
- [Slash commands](docs/commands.md) — full command reference, including `/lgtm`, `/approve`, `/hold`, `/cherry-pick`, `/assign`, `/size`, and lifecycle
- [Configuration](docs/configuration.md) — `stern.yaml` field reference; covers solo / no-OWNERS mode at the bottom
- [Development](docs/development.md) — build, auth, replaying events from CI, dry-run, common errors
