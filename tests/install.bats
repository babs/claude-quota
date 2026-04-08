#!/usr/bin/env bats
#
# Bats tests for scripts/install.sh helper functions. These source the
# installer script and exercise the pure helpers directly — no network,
# no real sudo, no real launchctl/gio/systemd.
#
# Requires bats-core 1.x. Run via `bats tests/install.bats` or through the
# Makefile target.

setup() {
  # Resolve scripts/install.sh relative to the test file, not cwd.
  SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"
  INSTALL_SH="${SCRIPT_DIR}/scripts/install.sh"
  # Sourcing the installer is side-effect-free: the main() body only runs
  # when BASH_SOURCE[0] == $0, which is not the case inside bats.
  # shellcheck source=/dev/null
  source "$INSTALL_SH"

  # Hermetic sandbox HOME so detect_providers / cleanup_legacy_entries
  # don't touch the developer's real files.
  TEST_HOME="$(mktemp -d)"
  export HOME="$TEST_HOME"
}

teardown() {
  rm -rf "$TEST_HOME"
}

# ──────────────────────────────────────────────────────────────────────
# provider_display
# ──────────────────────────────────────────────────────────────────────
@test "provider_display: claude → Claude" {
  run provider_display claude
  [ "$status" -eq 0 ]
  [ "$output" = "Claude" ]
}

@test "provider_display: codex → Codex" {
  run provider_display codex
  [ "$status" -eq 0 ]
  [ "$output" = "Codex" ]
}

@test "provider_display: unknown id passes through unchanged" {
  run provider_display foobar
  [ "$status" -eq 0 ]
  [ "$output" = "foobar" ]
}

# ──────────────────────────────────────────────────────────────────────
# has_flag
# ──────────────────────────────────────────────────────────────────────
@test "has_flag: exact token match" {
  run has_flag -indicator -foo -indicator bar-proj
  [ "$status" -eq 0 ]
}

@test "has_flag: name=value form match" {
  run has_flag -indicator -foo -indicator=bar-proj
  [ "$status" -eq 0 ]
}

@test "has_flag: no match" {
  run has_flag -indicator -foo -bar
  [ "$status" -eq 1 ]
}

@test "has_flag: empty argv returns not-found" {
  run has_flag -indicator
  [ "$status" -eq 1 ]
}

@test "has_flag: -provider does NOT match -provider-mark" {
  run has_flag -provider -provider-mark
  [ "$status" -eq 1 ]
}

@test "has_flag: -provider does NOT match -provider-mark-color" {
  run has_flag -provider -provider-mark-color
  [ "$status" -eq 1 ]
}

@test "has_flag: -provider does NOT match -provider-mark=false" {
  run has_flag -provider -provider-mark=false
  [ "$status" -eq 1 ]
}

@test "has_flag: -provider does NOT match -providers" {
  run has_flag -provider -providers
  [ "$status" -eq 1 ]
}

@test "has_flag: -provider matches -provider" {
  run has_flag -provider -provider
  [ "$status" -eq 0 ]
}

@test "has_flag: -provider matches -provider=claude" {
  run has_flag -provider -provider=claude
  [ "$status" -eq 0 ]
}

# ──────────────────────────────────────────────────────────────────────
# desktop_quote_flags — per freedesktop spec §Exec
# ──────────────────────────────────────────────────────────────────────
@test "desktop_quote_flags: bare tokens stay bare" {
  run desktop_quote_flags -provider claude -indicator bar-proj
  [ "$status" -eq 0 ]
  [ "$output" = " -provider claude -indicator bar-proj" ]
}

@test "desktop_quote_flags: hex color containing # gets quoted" {
  run desktop_quote_flags -provider-mark-color "#DE7356"
  [ "$status" -eq 0 ]
  [ "$output" = ' -provider-mark-color "#DE7356"' ]
}

@test "desktop_quote_flags: space in value gets quoted" {
  run desktop_quote_flags "/home/first last/bin/claude-quota"
  [ "$status" -eq 0 ]
  [ "$output" = ' "/home/first last/bin/claude-quota"' ]
}

@test "desktop_quote_flags: backslash gets escaped AND quoted" {
  run desktop_quote_flags 'path\with\back'
  [ "$status" -eq 0 ]
  [ "$output" = ' "path\\with\\back"' ]
}

@test "desktop_quote_flags: double quote gets escaped AND quoted" {
  run desktop_quote_flags 'a"b'
  [ "$status" -eq 0 ]
  [ "$output" = ' "a\"b"' ]
}

@test "desktop_quote_flags: ampersand gets quoted" {
  run desktop_quote_flags "cmd&other"
  [ "$status" -eq 0 ]
  [ "$output" = ' "cmd&other"' ]
}

@test "desktop_quote_flags: dollar gets quoted" {
  run desktop_quote_flags 'foo$bar'
  [ "$status" -eq 0 ]
  [ "$output" = ' "foo$bar"' ]
}

@test "desktop_quote_flags: rejects embedded newline" {
  run desktop_quote_flags -foo $'bad\nval'
  [ "$status" -eq 1 ]
  [[ "$output" == *"embedded newline"* ]]
}

@test "desktop_quote_flags: rejects embedded carriage return" {
  run desktop_quote_flags -foo $'bad\rval'
  [ "$status" -eq 1 ]
  [[ "$output" == *"embedded newline"* ]]
}

@test "desktop_quote_flags: empty argv produces empty output" {
  run desktop_quote_flags
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ──────────────────────────────────────────────────────────────────────
# detect_providers — sandboxed HOME + env overrides
# ──────────────────────────────────────────────────────────────────────
@test "detect_providers: no creds → empty output" {
  run detect_providers
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "detect_providers: claude only" {
  mkdir -p "$HOME/.claude"
  touch "$HOME/.claude/.credentials.json"
  run detect_providers
  [ "$status" -eq 0 ]
  [ "$output" = "claude" ]
}

@test "detect_providers: codex only" {
  mkdir -p "$HOME/.codex"
  touch "$HOME/.codex/auth.json"
  run detect_providers
  [ "$status" -eq 0 ]
  [ "$output" = "codex" ]
}

@test "detect_providers: both creds → both lines, claude first" {
  mkdir -p "$HOME/.claude" "$HOME/.codex"
  touch "$HOME/.claude/.credentials.json" "$HOME/.codex/auth.json"
  run detect_providers
  [ "$status" -eq 0 ]
  # bats's $output joins lines with newlines
  [ "${lines[0]}" = "claude" ]
  [ "${lines[1]}" = "codex" ]
}

@test "detect_providers: CLAUDE_QUOTA_CLAUDE_HOME override" {
  local alt
  alt="$(mktemp -d)"
  mkdir -p "$alt/.claude"
  touch "$alt/.claude/.credentials.json"
  CLAUDE_QUOTA_CLAUDE_HOME="$alt" run detect_providers
  rm -rf "$alt"
  [ "$status" -eq 0 ]
  [ "$output" = "claude" ]
}

@test "detect_providers: CLAUDE_QUOTA_CODEX_HOME override" {
  local alt
  alt="$(mktemp -d)"
  mkdir -p "$alt/.codex"
  touch "$alt/.codex/auth.json"
  CLAUDE_QUOTA_CODEX_HOME="$alt" run detect_providers
  rm -rf "$alt"
  [ "$status" -eq 0 ]
  [ "$output" = "codex" ]
}

# ──────────────────────────────────────────────────────────────────────
# cleanup_legacy_entries — Linux glob scope
# ──────────────────────────────────────────────────────────────────────
@test "cleanup_legacy_entries: linux removes legacy, preserves new" {
  OS=linux
  mkdir -p "$HOME/.config/autostart" "$HOME/.local/share/applications"

  touch "$HOME/.config/autostart/claude-quota.desktop"
  touch "$HOME/.config/autostart/claude-quota-claude.desktop"
  touch "$HOME/.config/autostart/agent-quota-claude.desktop"

  touch "$HOME/.local/share/applications/claude-quota.desktop"
  touch "$HOME/.local/share/applications/agent-quota-codex.desktop"

  run cleanup_legacy_entries
  [ "$status" -eq 0 ]
  [[ "$output" == *"Migrated from legacy claude-quota naming"* ]]

  # Legacy: gone
  [ ! -e "$HOME/.config/autostart/claude-quota.desktop" ]
  [ ! -e "$HOME/.config/autostart/claude-quota-claude.desktop" ]
  [ ! -e "$HOME/.local/share/applications/claude-quota.desktop" ]

  # New: preserved
  [ -e "$HOME/.config/autostart/agent-quota-claude.desktop" ]
  [ -e "$HOME/.local/share/applications/agent-quota-codex.desktop" ]
}

@test "cleanup_legacy_entries: no legacy files → silent success" {
  OS=linux
  mkdir -p "$HOME/.config/autostart" "$HOME/.local/share/applications"
  run cleanup_legacy_entries
  [ "$status" -eq 0 ]
  # No "Migrated" message when nothing was removed.
  [[ "$output" != *"Migrated"* ]]
}

# ──────────────────────────────────────────────────────────────────────
# prune_obsolete_entries — set semantics on the install path
# ──────────────────────────────────────────────────────────────────────
@test "prune_obsolete_entries: linux removes provider not in keep list" {
  OS=linux
  mkdir -p "$HOME/.config/autostart" "$HOME/.local/share/applications"

  # Pretend a previous install left both providers behind.
  touch "$HOME/.local/share/applications/agent-quota-claude.desktop"
  touch "$HOME/.local/share/applications/agent-quota-codex.desktop"
  ln -sf "$HOME/.local/share/applications/agent-quota-claude.desktop" \
         "$HOME/.config/autostart/agent-quota-claude.desktop"
  ln -sf "$HOME/.local/share/applications/agent-quota-codex.desktop" \
         "$HOME/.config/autostart/agent-quota-codex.desktop"

  # User now only wants claude.
  run prune_obsolete_entries claude
  [ "$status" -eq 0 ]
  [[ "$output" == *"Pruned obsolete desktop entry"* ]]
  [[ "$output" == *"agent-quota-codex.desktop"* ]]

  # Claude survives.
  [ -e "$HOME/.local/share/applications/agent-quota-claude.desktop" ]
  [ -L "$HOME/.config/autostart/agent-quota-claude.desktop" ]

  # Codex is gone (both desktop file and autostart symlink).
  [ ! -e "$HOME/.local/share/applications/agent-quota-codex.desktop" ]
  [ ! -L "$HOME/.config/autostart/agent-quota-codex.desktop" ]
  [ ! -e "$HOME/.config/autostart/agent-quota-codex.desktop" ]
}

@test "prune_obsolete_entries: linux keeps everything when all in list" {
  OS=linux
  mkdir -p "$HOME/.config/autostart" "$HOME/.local/share/applications"
  touch "$HOME/.local/share/applications/agent-quota-claude.desktop"
  touch "$HOME/.local/share/applications/agent-quota-codex.desktop"

  run prune_obsolete_entries claude codex
  [ "$status" -eq 0 ]
  # No "Pruned" message when nothing was removed.
  [[ "$output" != *"Pruned"* ]]

  [ -e "$HOME/.local/share/applications/agent-quota-claude.desktop" ]
  [ -e "$HOME/.local/share/applications/agent-quota-codex.desktop" ]
}

@test "prune_obsolete_entries: empty keep list removes all agent-quota-* entries" {
  OS=linux
  mkdir -p "$HOME/.config/autostart" "$HOME/.local/share/applications"
  touch "$HOME/.local/share/applications/agent-quota-claude.desktop"
  touch "$HOME/.local/share/applications/agent-quota-codex.desktop"

  run prune_obsolete_entries
  [ "$status" -eq 0 ]
  [ ! -e "$HOME/.local/share/applications/agent-quota-claude.desktop" ]
  [ ! -e "$HOME/.local/share/applications/agent-quota-codex.desktop" ]
}

@test "prune_obsolete_entries: linux ignores non-agent-quota files" {
  OS=linux
  mkdir -p "$HOME/.local/share/applications"

  # An unrelated app's desktop file. Should NOT be touched.
  touch "$HOME/.local/share/applications/firefox.desktop"
  touch "$HOME/.local/share/applications/agent-quota-claude.desktop"

  run prune_obsolete_entries claude
  [ "$status" -eq 0 ]
  [ -e "$HOME/.local/share/applications/firefox.desktop" ]
  [ -e "$HOME/.local/share/applications/agent-quota-claude.desktop" ]
}

@test "prune_obsolete_entries: no entries → silent" {
  OS=linux
  mkdir -p "$HOME/.local/share/applications"
  run prune_obsolete_entries claude codex
  [ "$status" -eq 0 ]
  [[ "$output" != *"Pruned"* ]]
}
