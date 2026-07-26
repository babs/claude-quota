package main

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"fyne.io/systray"
)

// TestCacheChanged locks the dedup gate that suppresses redundant systray
// signals: same value → no emit, new value or new item → emit. Uses zero-value
// MenuItem pointers purely as map keys (no methods called), which is why the
// gate is extracted from setMenuTitle/setMenuVisible — those touch un-mockable
// systray.
func TestCacheChanged(t *testing.T) {
	titles := map[*systray.MenuItem]string{}
	a := &systray.MenuItem{}
	b := &systray.MenuItem{}

	if !cacheChanged(titles, a, "x") {
		t.Fatal("first value for an item must count as changed")
	}
	if cacheChanged(titles, a, "x") {
		t.Fatal("same value must not count as changed")
	}
	if !cacheChanged(titles, a, "y") {
		t.Fatal("new value must count as changed")
	}
	if !cacheChanged(titles, b, "x") {
		t.Fatal("a different item must be tracked independently")
	}

	// Same gate, bool instantiation (visibility).
	shown := map[*systray.MenuItem]bool{}
	if !cacheChanged(shown, a, true) {
		t.Fatal("first visibility must count as changed")
	}
	if cacheChanged(shown, a, true) {
		t.Fatal("same visibility must not count as changed")
	}
	if !cacheChanged(shown, a, false) {
		t.Fatal("toggled visibility must count as changed")
	}
}

// uiTestApp is an App wired for updateUI without a live systray, plus the push
// counters its recorders feed.
type uiTestApp struct {
	*App
	icons    int
	tooltips int
}

// newUITestApp builds an App whose menu items stay nil (every setter
// nil-guards) and whose icon/tooltip pushes land in counters.
func newUITestApp(t *testing.T) *uiTestApp {
	t.Helper()

	creds := &OAuthCredentials{provider: ProviderClaude}
	quota := NewQuotaClient(creds, nil)
	pct := 42.0
	reset := time.Date(2026, 1, 2, 18, 0, 0, 0, time.Local)
	updated := time.Date(2026, 1, 2, 14, 30, 5, 0, time.Local)
	quota.state = QuotaState{
		Provider:       ProviderClaude,
		FiveHour:       &pct,
		FiveHourResets: &reset,
		LastUpdate:     &updated,
	}

	ta := &uiTestApp{}
	ta.App = &App{
		config:     defaultConfig(),
		creds:      creds,
		quota:      quota,
		setIcon:    func([]byte) { ta.icons++ },
		setTooltip: func(string) { ta.tooltips++ },
	}
	return ta
}

// TestUpdateUIProductionPushPath exercises updateUI with setIcon/setTooltip nil
// — the real systray path, which every other test bypasses via recorders.
// Without this, dropping the nil guard in pushIcon/pushTooltip panics in
// production while the whole suite stays green. It also pins that updateUI is
// safe before the tray is up: systray caches the values when its D-Bus props
// are not exported yet, which is the startup window iconRepushInterval exists
// for.
func TestUpdateUIProductionPushPath(t *testing.T) {
	// Linux-only on purpose: there systray.SetIcon/SetTooltip cache their
	// argument and return while the D-Bus props are unexported, so this is a
	// safe no-op. On darwin they are raw cgo (C.setIcon / C.setTooltip) with no
	// initialisation guard, and on Windows they hit wt.setIcon — neither is
	// callable without a running tray, and the pre-commit go-unit-tests hook
	// runs this suite on whatever platform the contributor is on.
	if runtime.GOOS != "linux" {
		t.Skip("exercises real systray calls; only a safe no-op on Linux")
	}

	ta := newUITestApp(t)
	ta.setIcon = nil
	ta.setTooltip = nil

	ta.updateUI()
	ta.updateUI()

	if ta.icons != 0 || ta.tooltips != 0 {
		t.Fatalf("recorders fired despite being nil: icons=%d tooltips=%d", ta.icons, ta.tooltips)
	}
}

// TestUpdateUIEncodeFailure pins the recovery path: a failed icon encode must
// not wedge the tooltip into re-pushing on every pass, and the next successful
// encode must go out even if it produces the bytes that were last pushed.
func TestUpdateUIEncodeFailure(t *testing.T) {
	ta := newUITestApp(t)

	// Healthy pass first, so lastIcon holds real bytes.
	ta.updateUI()
	if ta.icons != 1 {
		t.Fatalf("healthy pass pushed icon=%d, want 1", ta.icons)
	}

	// IconSize 0 makes png encoding fail ("invalid image size: 0x0"). Config
	// validation rejects it, so only a direct write can reach this branch.
	ta.config.IconSize = 0
	ta.updateUI()
	ta.updateUI()
	if ta.icons != 1 {
		t.Errorf("encode failure pushed icon=%d, want 1 (nothing encodable to push)", ta.icons)
	}
	if ta.tooltips != 1 {
		t.Errorf("encode failure pushed tooltip=%d, want 1 (must not re-push every pass)", ta.tooltips)
	}

	// Recovery: same state as the first healthy pass, so identical bytes. It
	// must still go out, because the failures invalidated what the tray holds.
	ta.config.IconSize = defaultConfig().IconSize
	ta.updateUI()
	if ta.icons != 2 {
		t.Errorf("after recovery pushed icon=%d, want 2 (identical bytes must still be re-sent)", ta.icons)
	}
}

// TestUpdateUIPushDedup is the guard the value-dedup needs: it asserts what
// updateUI actually pushes, not just what the helper policy returns. Removing
// the dedup reds the "unchanged state" case; removing the repush bypass at the
// call site reds the "self-heal" case.
func TestUpdateUIPushDedup(t *testing.T) {
	ta := newUITestApp(t)

	// First pass: lastPush is zero, so everything must go out.
	ta.updateUI()
	if ta.icons != 1 || ta.tooltips != 1 {
		t.Fatalf("first updateUI pushed icon=%d tooltip=%d, want 1/1", ta.icons, ta.tooltips)
	}

	// Second pass, identical state, well inside iconRepushInterval: dedup must
	// suppress both. Without it the tray re-emits on every poll for no reason.
	ta.updateUI()
	if ta.icons != 1 || ta.tooltips != 1 {
		t.Fatalf("unchanged state pushed icon=%d tooltip=%d, want 1/1 (dedup must suppress)", ta.icons, ta.tooltips)
	}

	// Age the last push past the interval: the self-heal must fire even though
	// nothing changed, otherwise a dropped push persists until the digits move.
	ta.lastPush = time.Now().Add(-iconRepushInterval - time.Second)
	ta.updateUI()
	if ta.icons != 2 || ta.tooltips != 2 {
		t.Fatalf("stale lastPush pushed icon=%d tooltip=%d, want 2/2 (repush must bypass dedup)", ta.icons, ta.tooltips)
	}

	// And the self-heal clock restarts, so it does not fire again immediately.
	ta.updateUI()
	if ta.icons != 2 || ta.tooltips != 2 {
		t.Fatalf("after repush pushed icon=%d tooltip=%d, want 2/2 (clock must reset)", ta.icons, ta.tooltips)
	}
}

// TestUpdateUIPushesOnChange pins the other half: a real state change must reach
// the tray even when the repush interval has not elapsed.
func TestUpdateUIPushesOnChange(t *testing.T) {
	ta := newUITestApp(t)
	ta.updateUI()

	changed := 77.0
	ta.quota.mu.Lock()
	ta.quota.state.FiveHour = &changed
	ta.quota.mu.Unlock()

	ta.updateUI()
	if ta.icons != 2 {
		t.Errorf("changed quota pushed icon=%d, want 2", ta.icons)
	}
	if ta.tooltips != 2 {
		t.Errorf("changed quota pushed tooltip=%d, want 2", ta.tooltips)
	}
}

// TestScheduleNext covers the poll deadline bookkeeping: scheduleNext must both
// publish the deadline for the UI and hand the same value back to pollLoop, so
// the "Next update" line and the timer can never disagree.
func TestScheduleNext(t *testing.T) {
	a := &App{}

	if got := a.nextUpdateAt(); got != nil {
		t.Fatalf("nextUpdateAt() before scheduling = %v, want nil", got)
	}

	const interval = 300 * time.Second
	before := time.Now()
	deadline := a.scheduleNext(interval)
	after := time.Now()

	if deadline.Before(before.Add(interval)) || deadline.After(after.Add(interval)) {
		t.Errorf("scheduleNext returned %v, want within [%v, %v]",
			deadline, before.Add(interval), after.Add(interval))
	}
	stored := a.nextUpdate.Load()
	if stored == nil {
		t.Fatal("scheduleNext must publish the deadline for the UI")
	}
	if !stored.Equal(deadline) {
		t.Errorf("published deadline %v != returned deadline %v", *stored, deadline)
	}

	// nextUpdateAt re-derives from the remaining monotonic duration, so it lands
	// on the same instant as long as no clock jump happened in between.
	at := a.nextUpdateAt()
	if at == nil {
		t.Fatal("nextUpdateAt() after scheduling = nil")
	}
	if drift := at.Sub(deadline); drift < -time.Second || drift > time.Second {
		t.Errorf("nextUpdateAt() drifted %v from the scheduled deadline", drift)
	}
}

// TestShouldRepush locks the self-heal that value-dedup alone would remove:
// systray can silently drop an icon/tooltip push, and identical state produces
// identical icon bytes forever, so the dedup must be bypassed periodically.
// Deleting the bypass turns this red.
func TestShouldRepush(t *testing.T) {
	now := time.Date(2026, 1, 2, 14, 30, 5, 0, time.Local)

	tests := []struct {
		name     string
		lastPush time.Time
		want     bool
	}{
		{"never pushed", time.Time{}, true},
		{"just pushed", now, false},
		{"within interval", now.Add(-iconRepushInterval + time.Second), false},
		{"interval elapsed", now.Add(-iconRepushInterval), true},
		{"long overdue", now.Add(-24 * time.Hour), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{lastPush: tc.lastPush}
			if got := a.shouldRepush(now); got != tc.want {
				t.Errorf("shouldRepush() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestScheduleNextRearms guards the invariant pollLoop leans on: each call moves
// the deadline forward, and the returned value always matches what the UI reads.
func TestScheduleNextRearms(t *testing.T) {
	a := &App{}
	first := a.scheduleNext(time.Second)
	second := a.scheduleNext(time.Hour)

	if !second.After(first) {
		t.Errorf("re-arming with a longer interval must move the deadline forward: %v !> %v", second, first)
	}
	if stored := a.nextUpdate.Load(); stored == nil || !stored.Equal(second) {
		t.Errorf("published deadline %v, want %v", stored, second)
	}
}

func TestBuildTooltip_Empty(t *testing.T) {
	state := QuotaState{}
	got := buildTooltip(state, ProviderClaude)
	if got != "Agent Quota (Claude)" {
		t.Errorf("buildTooltip(empty) = %q, want %q", got, "Agent Quota (Claude)")
	}
}

func TestBuildTooltip_Error(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Error: something broke") {
		t.Errorf("buildTooltip(error) = %q, missing error line", got)
	}
	// Error state should not include quota lines.
	if strings.Contains(got, "5h:") {
		t.Errorf("buildTooltip(error) should not contain quota lines")
	}
}

func TestBuildTooltip_WithQuota(t *testing.T) {
	v5 := 42.0
	v7 := 10.0
	state := QuotaState{
		Provider: ProviderClaude,
		FiveHour: &v5,
		SevenDay: &v7,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "5h: 42%") {
		t.Errorf("buildTooltip missing 5h line: %q", got)
	}
	if !strings.Contains(got, "7d: 10%") {
		t.Errorf("buildTooltip missing 7d line: %q", got)
	}
}

func TestBuildTooltip_WithAllQuotas(t *testing.T) {
	v5 := 42.0
	v7 := 10.0
	vs := 5.0
	state := QuotaState{
		Provider:       ProviderClaude,
		FiveHour:       &v5,
		SevenDay:       &v7,
		SevenDaySonnet: &vs,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Sonnet 7d: 5%") {
		t.Errorf("buildTooltip missing Sonnet 7d line: %q", got)
	}
}

func TestBuildTooltip_WithLastUpdate(t *testing.T) {
	now := time.Now().UTC()
	state := QuotaState{LastUpdate: &now}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Updated:") {
		t.Errorf("buildTooltip missing Updated line: %q", got)
	}
}

func TestBuildTooltip_WithProjection(t *testing.T) {
	v5 := 33.0
	proj := 36.0
	resets := time.Now().Add(23 * time.Minute)
	state := QuotaState{
		Provider:          ProviderClaude,
		FiveHour:          &v5,
		FiveHourResets:    &resets,
		FiveHourProjected: &proj,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "5h: 33%") {
		t.Errorf("buildTooltip missing 5h line: %q", got)
	}
	if !strings.Contains(got, "\n  - projected ~36% at reset") {
		t.Errorf("buildTooltip missing projection on separate line: %q", got)
	}
}

func TestBuildTooltip_WithSaturation(t *testing.T) {
	v5 := 80.0
	proj := 400.0
	resets := time.Now().Add(4 * time.Hour)
	sat := time.Now().Add(15 * time.Minute)
	state := QuotaState{
		Provider:           ProviderClaude,
		FiveHour:           &v5,
		FiveHourResets:     &resets,
		FiveHourProjected:  &proj,
		FiveHourSaturation: &sat,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "projected ~400% at reset") {
		t.Errorf("buildTooltip missing uncapped projection: %q", got)
	}
	if !strings.Contains(got, "saturates in") {
		t.Errorf("buildTooltip missing saturation line: %q", got)
	}
}

func TestBuildTooltip_ErrorHidesQuota(t *testing.T) {
	v := 42.0
	state := QuotaState{
		Provider: ProviderClaude,
		FiveHour: &v,
		Error:    "token expired",
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Error: token expired") {
		t.Errorf("buildTooltip missing error: %q", got)
	}
	// When there's an error, quota lines should be hidden.
	if strings.Contains(got, "5h:") {
		t.Errorf("buildTooltip should hide quota on error: %q", got)
	}
}

func TestExtraQuotaLabel_CustomLabel(t *testing.T) {
	state := QuotaState{Provider: ProviderCodex, SevenDaySonnetLabel: "Code review"}
	if got := extraQuotaLabel(state); got != "Code review" {
		t.Fatalf("extraQuotaLabel = %q, want 'Code review'", got)
	}
}

func TestExtraQuotaLabel_CodexDefault(t *testing.T) {
	state := QuotaState{Provider: ProviderCodex}
	if got := extraQuotaLabel(state); got != "Additional" {
		t.Fatalf("extraQuotaLabel(codex, no label) = %q, want 'Additional'", got)
	}
}

func TestExtraQuotaLabel_ClaudeDefault(t *testing.T) {
	state := QuotaState{Provider: ProviderClaude}
	if got := extraQuotaLabel(state); got != "Sonnet 7d" {
		t.Fatalf("extraQuotaLabel(claude) = %q, want 'Sonnet 7d'", got)
	}
}

func TestBuildTooltip_CodexUsesProviderTitle(t *testing.T) {
	v5 := 12.0
	state := QuotaState{
		Provider: ProviderCodex,
		FiveHour: &v5,
		SevenDay: &v5,
	}
	got := buildTooltip(state, ProviderCodex)
	if !strings.HasPrefix(got, "Agent Quota (Codex)") {
		t.Fatalf("buildTooltip() = %q, want Codex title", got)
	}
}
