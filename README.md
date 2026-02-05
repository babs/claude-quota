# claude-quota

> **Warning:** This tool uses Claude Code's OAuth client ID to access your
> quota data via an undocumented API. This is not sanctioned by Anthropic
> and may violate the Terms of Service. Use at your own risk.

Systray widget that displays Claude API quota utilization.

Reads OAuth credentials from Claude Code CLI (`~/.claude/.credentials.json`),
polls the Anthropic usage API, and renders a color-coded pie chart icon
with live quota percentages.

## Features

- 5-hour, 7-day, and Sonnet 7-day quota tracking
- Color-coded icon: green (<60%), yellow (60-85%), red (>85%)
- Automatic OAuth token refresh
- Self-update from GitHub releases (`-update`)
- Cross-platform: Linux, Windows, macOS

## Prerequisites

Authenticate Claude Code first:

```bash
claude login
```

## Install from release

Download the latest binary for your platform from
[Releases](https://github.com/babs/claude-quota/releases), then run it.

## Build from source

Requires Go 1.24+.

```bash
go build -o claude-quota .
```

For a release build with version info and cross-compilation:

```bash
./release.sh
```

## Usage

```bash
./claude-quota            # start the systray widget
./claude-quota -version   # show version info
./claude-quota -update    # self-update to latest release
```

Click the systray icon to see the quota breakdown with reset times.

## Configuration

Optional. First run creates `~/.config/claude-quota/config.json`:

```json
{
  "poll_interval_seconds": 300,
  "thresholds": {
    "warning": 60,
    "critical": 85
  }
}
```

## Autostart (Linux)

Create `~/.config/autostart/claude-quota.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=Claude Quota Widget
Exec=/path/to/claude-quota
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
StartupNotify=false
Terminal=false
```

## How it works

The widget uses Claude Code's OAuth credentials to call
`api.anthropic.com/api/oauth/usage`. Tokens are refreshed automatically
when they expire and persisted back to the credentials file.

## License

MIT
