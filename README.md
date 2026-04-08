# claude-quota

> **Warning:** Claude quota support relies on an undocumented Anthropic endpoint.
> Codex quota support uses the public ChatGPT/Codex usage API.

Systray widget that displays Claude Code or Codex quota utilization.

Reads OAuth credentials from Claude Code (`~/.claude/.credentials.json`) or
Codex (`~/.codex/auth.json`), polls the provider quota API, and renders a
color-coded icon with live quota percentages. Multiple indicator styles available.

|         Systray icon          |              Hover tooltip              |             Context menu              |
| :---------------------------: | :-------------------------------------: | :-----------------------------------: |
| ![Systray icon](img/icon.png) | ![Hover tooltip](img/hover-tooltip.png) | ![Context menu](img/context-menu.png) |

## Features

- Claude: 5-hour, 7-day, and Sonnet 7-day quota tracking
- Codex: primary and secondary usage windows from `backend-api/wham/usage`
- Color-coded icon: green (<60%), yellow (60-85%), red (>85%)
- Multiple indicator styles: pie chart, bar, arc, bar with projection
- Burn-rate projection: estimates 5h utilization at window reset
- Optional text overlay toggle (`show_text`)
- Configurable icon size for HiDPI displays
- Reloads OAuth token from disk when expired (relies on `claude login`)
- Self-update from GitHub releases (`-update` or from the context menu)
- Cross-platform: Linux, Windows, macOS

## Prerequisites

- Authenticate your provider first: `claude login` or `codex login`
- `curl` and `xz` must be available (the install script checks for both)

## Install from release

Download the latest binary for your platform from
[Releases](https://github.com/babs/claude-quota/releases), then run it.

## One-liner install (macOS & Linux)

**Recommended: just run the installer with no `--providers` flag.** It
auto-detects providers from your credential files
(`~/.claude/.credentials.json`, `~/.codex/auth.json`) and installs one tray
per provider it finds. Re-running the installer after a `claude logout` /
`codex logout` automatically removes the corresponding tray, so the install
always reflects what you actually have credentials for.

```bash
curl -fsSL \
  https://raw.githubusercontent.com/babs/claude-quota/master/scripts/install.sh \
  | bash
```

If neither credential file exists, the installer aborts with a hint to run
`claude login` / `codex login` first.

- **macOS**: installs the binary to `/usr/local/bin`, registers one
  LaunchAgent per provider at `~/Library/LaunchAgents/com.github.babs.agent-quota.<provider>.plist`.
- **Linux**: installs the binary to `~/.local/share/claude-quota/`, creates
  one desktop entry per provider at `~/.local/share/applications/agent-quota-<provider>.desktop`
  (labeled `Agent Quota (Claude)` / `Agent Quota (Codex)`) and symlinks each
  into `~/.config/autostart/`.

### Set semantics

Each install run reflects **desired state**, not additive state. After every
run, the only `agent-quota-*` entries on disk are the ones in the current
provider set. So:

- Auto-detect with both creds present → both trays installed.
- Delete codex creds, re-run → codex tray pruned, claude tray kept.
- `--providers claude` (explicit) → only claude tray remains, even if a
  codex tray was previously installed. To install both, run with both creds
  present and let auto-detect handle it, or pass `--providers claude,codex`.

### Opinionated defaults

The installer auto-injects two flags on top of anything you pass (both
suppressed if you set them yourself):

- `-indicator bar-proj` — projected 5h-window view, most actionable for a
  tray that's visible all day.
- `-provider-mark` — only when installing two or more providers, so both
  trays are visually distinguishable.

### Extra flags

Any other flags you pass are persisted in the startup configuration (LaunchAgent
plist on macOS, `.desktop` entry on Linux):

```bash
curl -fsSL \
  https://raw.githubusercontent.com/babs/claude-quota/master/scripts/install.sh \
  | bash -s -- -stats -provider-mark-color '#DE7356'
```

### Other modes

Install explicitly for a subset of providers (skips auto-detect; **prunes
any provider not in the list**):

```bash
curl -fsSL \
  https://raw.githubusercontent.com/babs/claude-quota/master/scripts/install.sh \
  | bash -s -- --providers claude,codex
```

Install without configuring autostart (binary only, user manages startup manually):

```bash
curl -fsSL \
  https://raw.githubusercontent.com/babs/claude-quota/master/scripts/install.sh \
  | bash -s -- --no-autostart
```

Install a locally built binary instead of fetching from GitHub releases (dev
workflow):

```bash
go build -o claude-quota .
CLAUDE_QUOTA_BIN=$PWD/claude-quota ./scripts/install.sh
```

To uninstall (cleans up the current `com.github.babs.agent-quota.*` /
`agent-quota-*` entries plus any legacy `com.claude-quota*` /
`claude-quota*.desktop` entries from older installs):

```bash
curl -fsSL \
  https://raw.githubusercontent.com/babs/claude-quota/master/scripts/install.sh \
  | bash -s -- --uninstall
```

> Note: `-provider` is reserved by the installer — pass `--providers <list>`
> instead, or omit and let auto-detect choose. The installer injects
> `-provider <name>` into each entry.

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
./claude-quota                  # start the systray widget
./claude-quota -dry-run         # fetch quota once, print results, and exit
./claude-quota -version         # show version info
./claude-quota -update          # self-update to latest release
./claude-quota -provider codex  # use Codex quotas from ~/.codex/auth.json
./claude-quota -poll-interval 60
./claude-quota -font-size 24
./claude-quota -font-name bitmap  # pixel-crisp bitmap font
./claude-quota -icon-size 128     # for HiDPI / large systray panels
./claude-quota -provider-mark -provider-mark-position se
./claude-quota -indicator bar     # vertical bar indicator
./claude-quota -indicator arc     # progress ring indicator
./claude-quota -indicator bar-proj # side-by-side bar with burn-rate projection
./claude-quota -show-text=false   # hide percentage text on icon
./claude-quota -show-account      # show account email/org in menu
./claude-quota -stats             # enable local stats collection
```

Click the systray icon to see the quota breakdown with reset times.

## Configuration

Optional. First run creates `~/.config/claude-quota/config.json`:

```json
{
  "poll_interval_seconds": 300,
  "font_size": 34,
  "font_name": "bold",
  "halo_size": 2,
  "icon_size": 64,
  "indicator": "pie",
  "show_text": true,
  "provider_mark": false,
  "provider_mark_size": 14,
  "provider_mark_position": "SE",
  "show_account": false,
  "thresholds": {
    "warning": 60,
    "critical": 85
  }
}
```

| Setting                 | Config key              | Env var                           | CLI flag              | Default  |
| ----------------------- | ----------------------- | --------------------------------- | --------------------- | -------- |
| Provider                | `provider`              | `CLAUDE_QUOTA_PROVIDER`           | `-provider`           | auto-detect by newest credentials file |
| Dry-run mode            | n/a                     | n/a                               | `-dry-run`            | `false`   |
| Claude home dir         | `claude_home`           | `CLAUDE_QUOTA_CLAUDE_HOME`        | `-claude-home`        | `~`      |
| Codex home dir          | `codex_home`            | `CLAUDE_QUOTA_CODEX_HOME`         | `-codex-home`         | `~`      |
| Poll interval (seconds) | `poll_interval_seconds` | `CLAUDE_QUOTA_POLL_INTERVAL`      | `-poll-interval`      | `300`    |
| Font size               | `font_size`             | `CLAUDE_QUOTA_FONT_SIZE`          | `-font-size`          | `34`     |
| Font name               | `font_name`             | `CLAUDE_QUOTA_FONT_NAME`          | `-font-name`          | `"bold"` |
| Halo size               | `halo_size`             | `CLAUDE_QUOTA_HALO_SIZE`          | `-halo-size`          | `2`      |
| Icon size (px)          | `icon_size`             | `CLAUDE_QUOTA_ICON_SIZE`          | `-icon-size`          | `64`     |
| Indicator style         | `indicator`             | `CLAUDE_QUOTA_INDICATOR`          | `-indicator`          | `"pie"`  |
| Show text on icon       | `show_text`             | `CLAUDE_QUOTA_SHOW_TEXT`          | `-show-text`          | `true`   |
| Provider mark           | `provider_mark`         | `CLAUDE_QUOTA_PROVIDER_MARK`      | `-provider-mark`      | `false`  |
| Provider mark size      | `provider_mark_size`    | `CLAUDE_QUOTA_PROVIDER_MARK_SIZE` | `-provider-mark-size` | `14`     |
| Provider mark position  | `provider_mark_position`| `CLAUDE_QUOTA_PROVIDER_MARK_POSITION` | `-provider-mark-position` | `"SE"` |
| Provider mark color     | `provider_mark_color`   | `CLAUDE_QUOTA_PROVIDER_MARK_COLOR` | `-provider-mark-color` | provider default (`#RGB`, `#RRGGBB`, or `#RRGGBBAA`) |
| Show account in menu    | `show_account`          | `CLAUDE_QUOTA_SHOW_ACCOUNT`       | `-show-account`       | `false`  |
| Local stats collection  | `stats`                 | `CLAUDE_QUOTA_STATS`              | `-stats`              | `false`  |
| Warning threshold (%)   | `thresholds.warning`    | `CLAUDE_QUOTA_WARNING_THRESHOLD`  | `-warning-threshold`  | `60`     |
| Critical threshold (%)  | `thresholds.critical`   | `CLAUDE_QUOTA_CRITICAL_THRESHOLD` | `-critical-threshold` | `85`     |

> **Note:** When enabled, `-stats` stores quota snapshots in a local SQLite database
> for users who want to analyse their consumption over time. Data never leaves the machine.

`font_size` and `halo_size` are relative to the base icon size (64px). They scale
automatically with `icon_size` — e.g. at `icon_size: 128` the rendered font is 2x larger.

Available font names: `bold` (default), `regular`, `mono`, `monobold`, `bitmap`.
TTF fonts (`bold`, `regular`, `mono`, `monobold`) render smooth vector text.
The `bitmap` font uses pixel-scaled 7x13 bitmap rendering for a retro look.

Available indicator styles:

| Style      | Description                                                                                           |
| ---------- | ----------------------------------------------------------------------------------------------------- |
| `pie`      | Pie chart filling clockwise (default)                                                                 |
| `bar`      | Vertical bar filling bottom to top                                                                    |
| `arc`      | Progress ring filling clockwise from 12 o'clock                                                       |
| `bar-proj` | Two side-by-side bars: left = current 5h usage, right = projected usage at window reset (muted color) |

The `bar-proj` indicator extrapolates the average consumption rate over the
elapsed portion of the 5-hour window to estimate utilization at reset. The
projection is also shown in the tooltip and menu for all indicator styles
(e.g. `5h: 33% (resets in 23m, Mon 14:30)` followed by `  - ~36% at reset`
on a separate line). When projected usage exceeds 100%, a saturation time
is shown (e.g. `  - saturates in 1h 15m, Mon 13:15`).

Priority: CLI flag > environment variable > config file.

When `provider` is unset, the app auto-detects from available credentials:
if both `~/.claude/.credentials.json` and `~/.codex/auth.json` exist, it picks
the most recently modified file.

## Windows + WSL

If Claude Code is installed inside WSL, the credentials live in the WSL
filesystem. Point `claude-quota.exe` to the WSL home directory:

```powershell
claude-quota.exe -claude-home \\wsl$\<distro>\home\<username>
```

Or via environment variable:

```powershell
set CLAUDE_QUOTA_CLAUDE_HOME=\\wsl$\<distro>\home\<username>
claude-quota.exe
```

Replace `<distro>` with your WSL distribution name and `<username>` with your WSL username.
To list available WSL distributions, run `wsl -l -q` in PowerShell or cmd.

Codex credentials can be redirected the same way:

```powershell
claude-quota.exe -provider codex -codex-home \\wsl$\<distro>\home\<username>
```

## Autostart (Linux)

The install script configures autostart automatically (one entry per
provider). For manual setup, create one file per provider at
`~/.local/share/applications/agent-quota-<provider>.desktop`, for example
`agent-quota-claude.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=Agent Quota (Claude)
Comment=Tray widget for Claude quota (formerly claude-quota)
Exec=$HOME/.local/share/claude-quota/claude-quota -provider claude
Icon=$HOME/.local/share/claude-quota/claude-quota.svg
Hidden=false
NoDisplay=false
StartupNotify=false
Terminal=false
```

Then symlink each file into autostart:

```bash
ln -sf ~/.local/share/applications/agent-quota-claude.desktop ~/.config/autostart/
```

Repeat for Codex with `-provider codex` and the `codex` suffix.

## How it works

For Claude, the widget calls `api.anthropic.com/api/oauth/usage`.
For Codex, it calls the public `chatgpt.com/backend-api/wham/usage` endpoint.
When the token expires, it is reloaded from disk in case the CLI has
refreshed it externally. If the token is still expired, an amber warning
icon is shown and the tooltip tells you to run `claude login` or
`codex login`.

## License

MIT
