#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────
# Constants — safe to read at source time. Helper functions below depend on
# these. Runtime state (OS, INSTALL_DIR, PROVIDERS, …) is set inside main().
# ──────────────────────────────────────────────────────────────────────────
REPO="babs/claude-quota"
BINARY="claude-quota"
ICON_URL="https://raw.githubusercontent.com/${REPO}/master/img/claude-quota.svg"
# Reverse-DNS LaunchAgent label prefix. Mirrors the github.com/babs namespace
# so multiple "agent-quota" forks/projects could coexist on the same machine
# without colliding on launchctl labels.
LABEL_PREFIX="com.github.babs.agent-quota"
DESKTOP_PREFIX="agent-quota"

usage() {
  echo "Usage: $0 [--uninstall] [--no-autostart] [--providers <list>] [flags for ${BINARY}...]"
  echo ""
  echo "  --uninstall          Remove ${BINARY} and all autostart entries (legacy + new)"
  echo "  --no-autostart       Install without configuring autostart"
  echo "  --providers <list>   Comma-separated subset of: claude,codex"
  echo "                       One autostart entry per provider."
  echo ""
  echo "Recommended: omit --providers and let the installer auto-detect from"
  echo "your credential files (~/.claude/.credentials.json, ~/.codex/auth.json)."
  echo "Re-running after a 'claude logout' / 'codex logout' will then automatically"
  echo "remove the corresponding tray entry — the install reflects desired state."
  echo ""
  echo "Set semantics: each invocation deploys exactly the providers that are"
  echo "present on disk (auto-detected) or explicitly listed. Any agent-quota-*"
  echo "entries for providers NOT in that set are pruned. To 'add' codex to an"
  echo "existing claude install, run with both creds present, not with"
  echo "--providers codex (which would remove the claude tray)."
  echo ""
  echo "  Any other flags are passed to ${BINARY} and persisted in autostart config."
  echo "  -provider is reserved by the installer — do not pass it here."
  echo ""
  echo "Installer defaults (suppressed when the user passes the same flag):"
  echo "  -indicator bar-proj"
  echo "  -provider-mark             (only when installing 2+ providers)"
  echo ""
  echo "Env vars:"
  echo "  CLAUDE_QUOTA_BIN           Path to a local ${BINARY} binary to install"
  echo "                             instead of downloading from GitHub releases."
  echo "  CLAUDE_QUOTA_CLAUDE_HOME   Override \$HOME for Claude credential detection."
  echo "  CLAUDE_QUOTA_CODEX_HOME    Override \$HOME for Codex credential detection."
  echo ""
  echo "Example: $0 -provider-mark-color '#DE7356'"
  exit 0
}

# provider_display maps a provider id to its human label. Mirrors
# providerDisplayName in provider.go so install labels match the tray menu.
provider_display() {
  case "$1" in
    claude) printf 'Claude' ;;
    codex)  printf 'Codex' ;;
    *)      printf '%s' "$1" ;;
  esac
}

# has_flag returns 0 if the first argument appears in the remaining arguments
# as either an exact token or a `name=value` form. Used to detect when a user
# flag should suppress an installer default.
has_flag() {
  local needle="$1"; shift
  local f
  for f in "$@"; do
    [[ "$f" == "$needle" || "$f" == "${needle}="* ]] && return 0
  done
  return 1
}

# detect_providers probes well-known credential files and prints the list of
# installed providers, one per line. Mirrors defaultProvider in provider.go,
# honoring CLAUDE_QUOTA_{CLAUDE,CODEX}_HOME env overrides. Note: the runtime
# also honors claude_home/codex_home from config.json; we intentionally do
# not parse the JSON here to keep the installer dependency-free. Users who
# rely on config-file home overrides should pass --providers explicitly.
detect_providers() {
  local claude_home="${CLAUDE_QUOTA_CLAUDE_HOME:-$HOME}"
  local codex_home="${CLAUDE_QUOTA_CODEX_HOME:-$HOME}"
  [[ -f "${claude_home}/.claude/.credentials.json" ]] && printf 'claude\n'
  [[ -f "${codex_home}/.codex/auth.json" ]]          && printf 'codex\n'
  # Always succeed: the caller consumes the output via process substitution
  # and decides what to do with an empty result. If we let the last `[[ -f ]]`
  # exit status leak out, detect_providers would return 1 whenever the codex
  # file is missing — hostile to test harnesses and surprising for future
  # callers that use `$(detect_providers)` or `if detect_providers; then`.
  return 0
}

# prune_obsolete_entries removes agent-quota-* entries (and their macOS plist
# equivalents) for providers that are NOT in the supplied keep list. This is
# what makes install runs reflect *desired state* rather than be additive:
# if the user previously installed claude+codex and then runs the installer
# after deleting their codex creds, the next auto-detect run only finds
# claude — and we want the codex tray to disappear too, instead of being
# orphaned and erroring out at next login.
#
# The keep list is the current PROVIDERS array, passed positionally so this
# stays a pure helper that works for both auto-detect and explicit modes.
#
# `shopt -s nullglob` is set without restoration: nothing downstream in this
# script depends on nullglob being off, and avoiding scope-save/restore keeps
# the function portable to bash 3.2 (macOS /bin/bash).
prune_obsolete_entries() {
  local -a keep=("$@")
  shopt -s nullglob
  local removed=0
  local provider keep_set
  # Build a space-padded membership string so `[[ "$keep_set" == *" $p "* ]]`
  # tests work without needing associative arrays (bash 3.2 compat).
  keep_set=" ${keep[*]} "

  if [[ "$OS" = "darwin" ]]; then
    local plist label
    for plist in "$HOME/Library/LaunchAgents/${LABEL_PREFIX}."*.plist; do
      [[ -f "$plist" ]] || continue
      label=$(basename "$plist" .plist)
      provider="${label##*.}"
      if [[ "$keep_set" != *" $provider "* ]]; then
        launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null || true
        rm -f "$plist"
        echo "Pruned obsolete LaunchAgent: $plist"
        removed=$((removed + 1))
      fi
    done
  elif [[ "$OS" = "linux" ]]; then
    local desktop autostart
    for desktop in "$HOME/.local/share/applications/${DESKTOP_PREFIX}-"*.desktop; do
      [[ -e "$desktop" ]] || continue
      provider=$(basename "$desktop" .desktop)
      provider="${provider#"${DESKTOP_PREFIX}"-}"
      if [[ "$keep_set" != *" $provider "* ]]; then
        rm -f "$desktop"
        echo "Pruned obsolete desktop entry: $desktop"
        removed=$((removed + 1))
        autostart="$HOME/.config/autostart/${DESKTOP_PREFIX}-${provider}.desktop"
        if [[ -L "$autostart" || -e "$autostart" ]]; then
          rm -f "$autostart"
          echo "Pruned obsolete autostart symlink: $autostart"
        fi
      fi
    done
  fi
  if [[ $removed -gt 0 ]]; then
    echo "Pruned ${removed} obsolete entries no longer in the install set."
  fi
}

# cleanup_legacy_entries removes pre-rebrand (com.claude-quota /
# claude-quota.desktop) autostart files so a user upgrading from the old
# installer does not end up with two sets of entries — and two processes —
# running side by side at the next login. Called from both the install path
# (as a migration step) and uninstall (for completeness).
cleanup_legacy_entries() {
  shopt -s nullglob
  local removed=0
  local plist label entry
  if [[ "$OS" = "darwin" ]]; then
    local plists=(
      "$HOME/Library/LaunchAgents/com.claude-quota."*.plist
      "$HOME/Library/LaunchAgents/com.claude-quota.plist"
    )
    for plist in "${plists[@]}"; do
      [[ -f "$plist" ]] || continue
      label=$(basename "$plist" .plist)
      launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null || true
      rm -f "$plist"
      echo "Removed legacy LaunchAgent: $plist"
      removed=$((removed + 1))
    done
  elif [[ "$OS" = "linux" ]]; then
    local entries=(
      "$HOME/.config/autostart/claude-quota"*.desktop
      "$HOME/.local/share/applications/claude-quota"*.desktop
    )
    for entry in "${entries[@]}"; do
      [[ -e "$entry" ]] || continue
      rm -f "$entry"
      echo "Removed legacy desktop entry: $entry"
      removed=$((removed + 1))
    done
  fi
  if [[ $removed -gt 0 ]]; then
    echo "Migrated from legacy claude-quota naming (${removed} entries removed)."
  fi
}

uninstall() {
  # Same rationale as cleanup_legacy_entries: set nullglob without restoring
  # to stay portable to bash 3.2. uninstall is the script's terminal action
  # anyway, so leaking shell options doesn't matter.
  shopt -s nullglob
  local plist label entry

  if [[ "$OS" = "darwin" ]]; then
    local plists=(
      "$HOME/Library/LaunchAgents/${LABEL_PREFIX}."*.plist
      "$HOME/Library/LaunchAgents/com.claude-quota."*.plist
      "$HOME/Library/LaunchAgents/com.claude-quota.plist"
    )
    for plist in "${plists[@]}"; do
      [[ -f "$plist" ]] || continue
      label=$(basename "$plist" .plist)
      if launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null; then
        echo "LaunchAgent ${label} stopped."
      fi
      rm -f "$plist"
      echo "Removed $plist"
    done
    # Fail loud if the binary removal is blocked (e.g. sudo rejected) rather
    # than silently printing "Uninstalled." while the file is still on disk.
    if [[ -f "${INSTALL_DIR}/${BINARY}" ]]; then
      sudo rm -f "${INSTALL_DIR}/${BINARY}"
      echo "Removed ${INSTALL_DIR}/${BINARY}"
    fi
  elif [[ "$OS" = "linux" ]]; then
    if pkill -x "$BINARY" 2>/dev/null; then
      echo "Stopped running ${BINARY} instances."
    fi
    local entries=(
      "$HOME/.config/autostart/${DESKTOP_PREFIX}-"*.desktop
      "$HOME/.config/autostart/claude-quota"*.desktop
      "$HOME/.local/share/applications/${DESKTOP_PREFIX}-"*.desktop
      "$HOME/.local/share/applications/claude-quota"*.desktop
    )
    for entry in "${entries[@]}"; do
      rm -f "$entry"
      echo "Removed $entry"
    done
    rm -f "${INSTALL_DIR}/${BINARY}" "${INSTALL_DIR}/claude-quota.svg"
    echo "Removed binary and icon"
    if rmdir "$INSTALL_DIR" 2>/dev/null; then
      echo "Removed $INSTALL_DIR"
    else
      echo "Note: $INSTALL_DIR not empty, kept in place"
    fi
  fi
  echo "Uninstalled."
}

# desktop_quote_flags renders an argv into freedesktop Exec= form. Per spec
# (§Exec), an argument must be wrapped in double quotes ONLY if it contains a
# reserved character: space, tab, newline, " ' \ > < ~ | & ; $ * ? # ( ) `
# Arguments without reserved chars are emitted bare to match distro convention
# and avoid noisy Exec lines. Rejects embedded newlines (and CR) because the
# .desktop Exec= line is single-line and there is no way to escape a newline
# inside a quoted Exec value.
desktop_quote_flags() {
  # Reserved character class for bracket expression: space, tab, newline,
  # then the punctuation listed in the spec. $'...' resolves \t and \n.
  local reserved=$' \t\n"'\''\<>~|&;$*?#()`'
  local out="" f esc
  for f in "$@"; do
    case "$f" in
      *$'\n'*|*$'\r'*)
        echo "Error: flag value contains an embedded newline; cannot encode in .desktop Exec line." >&2
        return 1
        ;;
      *["$reserved"]*)
        esc=${f//\\/\\\\}
        esc=${esc//\"/\\\"}
        out+=" \"${esc}\""
        ;;
      *)
        out+=" ${f}"
        ;;
    esac
  done
  printf '%s' "$out"
}

# install_entry installs the autostart entry for a single provider. Takes the
# provider name followed by the shared effective flags (which will be prefixed
# with -provider <provider>).
install_entry() {
  local provider="$1"; shift
  local flags=(-provider "$provider" "$@")
  local display name comment label
  display="$(provider_display "$provider")"
  name="Agent Quota (${display})"
  comment="Tray widget for ${display} quota (formerly claude-quota)"
  label="${LABEL_PREFIX}.${provider}"

  if [[ "$OS" = "linux" ]]; then
    local desktop_path="$HOME/.local/share/applications/${DESKTOP_PREFIX}-${provider}.desktop"
    local autostart_path="$HOME/.config/autostart/${DESKTOP_PREFIX}-${provider}.desktop"
    local icon_path="${INSTALL_DIR}/claude-quota.svg"
    local exec_line
    # Quote the binary path too: INSTALL_DIR is under $HOME on Linux, so a
    # HOME with whitespace would otherwise split the Exec token on the first
    # unquoted space.
    exec_line="$(desktop_quote_flags "${INSTALL_DIR}/${BINARY}" "${flags[@]}")"
    # desktop_quote_flags emits a leading space; strip it so Exec= is clean.
    exec_line="${exec_line# }"

    mkdir -p "$(dirname "$desktop_path")"
    cat > "$desktop_path" << EOF
[Desktop Entry]
Type=Application
Name=${name}
Comment=${comment}
Exec=${exec_line}
Icon=${icon_path}
Hidden=false
NoDisplay=false
StartupNotify=false
Terminal=false
EOF
    echo "Desktop entry installed: $desktop_path"

    if [[ "$AUTOSTART" = "1" ]]; then
      mkdir -p "$(dirname "$autostart_path")"
      ln -sf "$desktop_path" "$autostart_path"
      echo "Autostart symlink: $autostart_path -> $desktop_path"
      # `|| true` on each launcher: with set -e, a failure in a headless or
      # dbus-less session would otherwise abort the install loop halfway
      # through a multi-provider install. We have symlinks + binary in place
      # regardless; the user's next login will pick them up.
      if command -v gio &>/dev/null; then
        gio launch "$desktop_path" || true
      elif command -v gtk-launch &>/dev/null; then
        gtk-launch "${DESKTOP_PREFIX}-${provider}" || true
      elif command -v dex &>/dev/null; then
        dex "$desktop_path" || true
      else
        nohup "${INSTALL_DIR}/${BINARY}" "${flags[@]}" &>/dev/null &
      fi
      echo "Started ${name}."
    fi

  elif [[ "$OS" = "darwin" ]]; then
    # Gate the plist write on AUTOSTART=1: launchd auto-discovers every plist
    # in ~/Library/LaunchAgents at login and honors RunAtLoad=true, so merely
    # writing the file is equivalent to enabling autostart. If the user asked
    # for --no-autostart we skip plist emission entirely. The top-level
    # "Skipping autostart configuration" echo already covers this case.
    if [[ "$AUTOSTART" != "1" ]]; then
      return 0
    fi

    local plist_path="$HOME/Library/LaunchAgents/${label}.plist"
    local log_path="/tmp/${DESKTOP_PREFIX}-${provider}.log"
    mkdir -p "$(dirname "$plist_path")"

    local plist_args="        <string>${INSTALL_DIR}/${BINARY}</string>"
    local flag
    for flag in "${flags[@]}"; do
      plist_args="${plist_args}
        <string>${flag}</string>"
    done

    cat > "$plist_path" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${label}</string>
    <key>ProgramArguments</key>
    <array>
${plist_args}
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${log_path}</string>
    <key>StandardErrorPath</key>
    <string>${log_path}</string>
</dict>
</plist>
EOF
    echo "LaunchAgent plist: $plist_path"

    launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null || true
    launchctl bootstrap "gui/$(id -u)" "$plist_path"
    echo "LaunchAgent ${label} installed and started."
  fi
}

# ──────────────────────────────────────────────────────────────────────────
# main drives the imperative install flow. Sourcing this file (for tests)
# executes only the constant + function definitions above; main is called
# only when the script is invoked directly.
# ──────────────────────────────────────────────────────────────────────────
main() {
  command -v curl &>/dev/null || { echo "Required command not found: curl"; exit 1; }
  # xz is only needed when downloading the packaged release; skip the check
  # when CLAUDE_QUOTA_BIN bypasses the download path so dev boxes without xz
  # can still use the installer.
  if [[ -z "${CLAUDE_QUOTA_BIN:-}" ]]; then
    command -v xz &>/dev/null || { echo "Required command not found: xz"; exit 1; }
  fi

  # Detect OS and arch. OS and INSTALL_DIR are package-globals that helper
  # functions (uninstall, install_entry, cleanup_legacy_entries) consult.
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)        ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
  esac
  case "$OS" in
    darwin)
      INSTALL_DIR="/usr/local/bin"
      ;;
    linux)
      INSTALL_DIR="$HOME/.local/share/claude-quota"
      ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
  esac

  UNINSTALL=0
  AUTOSTART=1
  BINARY_FLAGS=()
  local PROVIDERS_ARG=""
  # Two-word --providers <value> handling: flip a pending flag so the next
  # non-option argument becomes the value.
  local shift_providers=0
  local arg
  for arg in "$@"; do
    if [[ $shift_providers -eq 1 ]]; then
      if [[ -z "$arg" ]]; then
        echo "Error: --providers requires a non-empty value." >&2
        exit 2
      fi
      PROVIDERS_ARG="$arg"
      shift_providers=0
      continue
    fi
    case "$arg" in
      --uninstall)    UNINSTALL=1 ;;
      --no-autostart) AUTOSTART=0 ;;
      --providers=*)
        PROVIDERS_ARG="${arg#*=}"
        if [[ -z "$PROVIDERS_ARG" ]]; then
          echo "Error: --providers= requires a non-empty value." >&2
          exit 2
        fi
        ;;
      --providers)    shift_providers=1 ;;
      -h|--help)      usage ;;
      *)              BINARY_FLAGS+=("$arg") ;;
    esac
  done
  if [[ $shift_providers -eq 1 ]]; then
    echo "Error: --providers requires a value." >&2
    exit 2
  fi

  # Reject -provider in BINARY_FLAGS on install paths only. On --uninstall
  # all flags are ignored anyway, and aborting there would surprise users.
  if [[ "$UNINSTALL" != "1" ]] && has_flag -provider "${BINARY_FLAGS[@]+"${BINARY_FLAGS[@]}"}"; then
    echo "Error: -provider is reserved by the installer." >&2
    echo "Use --providers <list> instead (or omit to auto-detect)." >&2
    exit 2
  fi

  # Resolve the final provider list. Precedence: explicit --providers > auto-detect.
  PROVIDERS=()
  if [[ -n "$PROVIDERS_ARG" ]]; then
    local -a raw_providers
    IFS=',' read -r -a raw_providers <<< "$PROVIDERS_ARG"
    local p
    for p in "${raw_providers[@]}"; do
      p="${p// /}"
      case "$p" in
        claude|codex) PROVIDERS+=("$p") ;;
        "")           continue ;;
        *)            echo "Error: unknown provider '$p' in --providers. Allowed: claude, codex." >&2; exit 2 ;;
      esac
    done
    if [[ ${#PROVIDERS[@]} -eq 0 ]]; then
      echo "Error: --providers=${PROVIDERS_ARG} yielded no valid providers." >&2
      exit 2
    fi
  fi

  if [[ "$UNINSTALL" != "1" ]]; then
    if [[ ${#PROVIDERS[@]} -eq 0 ]]; then
      local p
      while IFS= read -r p; do
        [[ -n "$p" ]] && PROVIDERS+=("$p")
      done < <(detect_providers)
      if [[ ${#PROVIDERS[@]} -eq 0 ]]; then
        echo "Error: no provider credentials detected." >&2
        echo "Run 'claude login' or 'codex login' first, or pass --providers claude,codex explicitly." >&2
        exit 1
      fi
      echo "Auto-detected providers: ${PROVIDERS[*]}"
    else
      echo "Using --providers: ${PROVIDERS[*]}"
    fi
  fi

  [[ "$UNINSTALL" = "1" ]] && { uninstall; exit 0; }

  # Inject installer-opinionated defaults once, shared across all entries.
  # These are suppressed if the user already set the same flag, so explicit
  # wins.
  effective_flags=("${BINARY_FLAGS[@]+"${BINARY_FLAGS[@]}"}")
  if has_flag -indicator "${effective_flags[@]+"${effective_flags[@]}"}"; then
    echo "Keeping user-specified -indicator (skipping installer default bar-proj)."
  else
    effective_flags+=(-indicator bar-proj)
  fi
  if [[ ${#PROVIDERS[@]} -ge 2 ]]; then
    if has_flag -provider-mark "${effective_flags[@]+"${effective_flags[@]}"}"; then
      echo "Keeping user-specified -provider-mark (skipping installer default for multi-provider install)."
    else
      effective_flags+=(-provider-mark)
    fi
  fi

  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT

  # Source the binary: either a user-supplied local file (CLAUDE_QUOTA_BIN) or
  # the latest GitHub release. Local override is useful for dev builds and for
  # smoke-testing installer changes without cutting a release.
  if [[ -n "${CLAUDE_QUOTA_BIN:-}" ]]; then
    if [[ ! -f "$CLAUDE_QUOTA_BIN" ]]; then
      echo "Error: CLAUDE_QUOTA_BIN=${CLAUDE_QUOTA_BIN} is not a regular file." >&2
      exit 1
    fi
    echo "Using local binary: $CLAUDE_QUOTA_BIN"
    cp "$CLAUDE_QUOTA_BIN" "$TMP/${BINARY}"
    chmod +x "$TMP/${BINARY}"
  else
    echo "Fetching latest release..."
    local LATEST
    LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | cut -d'"' -f4)
    [[ -z "$LATEST" ]] && { echo "Failed to fetch latest release."; exit 1; }
    echo "Latest: $LATEST"

    local ASSET URL
    ASSET="${BINARY}-${OS}-${ARCH}"
    URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}.xz"
    echo "Downloading $URL..."
    curl -fsSL "$URL" -o "$TMP/${ASSET}.xz"
    xz -d "$TMP/${ASSET}.xz"
    mv "$TMP/${ASSET}" "$TMP/${BINARY}"
    chmod +x "$TMP/${BINARY}"
  fi

  mkdir -p "$INSTALL_DIR"
  if [[ "$OS" = "darwin" ]]; then
    sudo mv "$TMP/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  else
    mv "$TMP/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  fi
  echo "Installed: ${INSTALL_DIR}/${BINARY}"

  # Linux: always install the shared icon; per-provider desktop files
  # reference it. When invoked from a source checkout (./scripts/install.sh),
  # prefer the local SVG so devs testing icon changes don't silently get the
  # master copy. Guard on $0 looking like our script file: when piped via
  # `curl | bash`, $0 is typically "bash" and dirname resolves relative to
  # cwd, which we don't want to inherit.
  if [[ "$OS" = "linux" ]]; then
    local ICON_PATH LOCAL_ICON
    ICON_PATH="${INSTALL_DIR}/claude-quota.svg"
    LOCAL_ICON=""
    if [[ "$0" == */install.sh && -f "$(dirname "$0")/../img/claude-quota.svg" ]]; then
      LOCAL_ICON="$(dirname "$0")/../img/claude-quota.svg"
    fi
    if [[ -n "$LOCAL_ICON" ]]; then
      echo "Using local icon: $LOCAL_ICON"
      cp "$LOCAL_ICON" "$ICON_PATH"
    else
      echo "Downloading icon..."
      curl -fsSL "$ICON_URL" -o "$ICON_PATH" || echo "Warning: icon download failed, continuing without icon"
    fi
  fi

  if [[ "$AUTOSTART" = "0" ]]; then
    echo "Skipping autostart configuration (--no-autostart)."
  fi

  # On Linux, stop any running instances BEFORE deploying new entries, so a
  # later iteration doesn't kill the process started by an earlier one. pkill
  # -x matches the bare binary name, covering all providers at once. Wait
  # briefly (up to ~2s) for processes to actually exit so the new ones don't
  # race with the old on stats.db or the tray socket. Escalate to SIGKILL if
  # anything is still alive at the end of the polite window.
  if [[ "$OS" = "linux" && "$AUTOSTART" = "1" ]]; then
    if pkill -x "$BINARY" 2>/dev/null; then
      echo "Stopped running ${BINARY} instances."
      local _
      for _ in 1 2 3 4 5 6 7 8 9 10; do
        pgrep -x "$BINARY" >/dev/null 2>&1 || break
        sleep 0.2
      done
      if pgrep -x "$BINARY" >/dev/null 2>&1; then
        echo "Warning: ${BINARY} still running after 2s, sending SIGKILL."
        pkill -9 -x "$BINARY" 2>/dev/null || true
        sleep 0.2
      fi
    fi
  fi

  # Migrate legacy entries. Gating differs per OS:
  #  * Linux: ~/.config/autostart entries are explicit symlinks, so skipping
  #    them on --no-autostart honors the "don't touch startup" intent — a
  #    legacy symlink keeps working as the user set it up.
  #  * macOS: launchd auto-discovers EVERY plist in ~/Library/LaunchAgents at
  #    login and honors RunAtLoad=true. Leaving a legacy com.claude-quota.plist
  #    in place means the updated binary would still be started at next login
  #    with the legacy flags — silently contradicting --no-autostart. So on
  #    darwin we always clean up legacy plists regardless of AUTOSTART.
  if [[ "$OS" = "darwin" || "$AUTOSTART" = "1" ]]; then
    cleanup_legacy_entries
  fi

  # Prune obsolete agent-quota-* entries: any provider that has a desktop
  # file or plist on disk but is NOT in the current PROVIDERS set gets
  # removed. This makes install.sh "set semantics" — each invocation
  # reflects the desired state, not additive — so a user who deletes their
  # codex creds and re-runs the installer (auto-detect) gets the codex tray
  # cleanly removed instead of orphaned. Same gating as legacy cleanup.
  if [[ "$OS" = "darwin" || "$AUTOSTART" = "1" ]]; then
    prune_obsolete_entries "${PROVIDERS[@]}"
  fi

  local provider
  for provider in "${PROVIDERS[@]}"; do
    install_entry "$provider" "${effective_flags[@]+"${effective_flags[@]}"}"
  done

  # On Linux, INSTALL_DIR is under $HOME and not in PATH by default, so the
  # bare binary name wouldn't resolve. Print the full path so the verify hint
  # is copy-pasteable on every platform.
  echo "Done. Run '${INSTALL_DIR}/${BINARY} -version' to verify."
}

# Only run main when the script is invoked directly. Test harnesses that
# `source scripts/install.sh` to exercise individual helpers skip main
# entirely — this keeps sourcing side-effect-free.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
