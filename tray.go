package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"golang.org/x/mod/semver"
)

// updatePhase tracks the state of the update menu item.
type updatePhase int

const (
	updatePhaseCheck   updatePhase = iota // "Check for Updates" / "Up to date"
	updatePhaseReady                      // "Update to vX.X.X"
	updatePhaseApplied                    // "Restart to apply update"
)

// iconRepushInterval bounds how long a dropped icon/tooltip push can persist.
// Two ways a push is lost silently: fyne.io/systray discards SetIcon/SetTooltip
// that land between its createPropSpec() snapshot and instance.props being
// assigned (onReady runs on its own goroutine, concurrently with that setup),
// and some panels only repaint on the NewIcon signal after losing the pixmap.
// Value dedup alone would leave either state stale until the quota digits move
// — possibly overnight on an idle machine — so re-push unconditionally at this
// cadence. Costs one D-Bus signal per interval; the churn this dedup exists to
// kill was the old 1 Hz menu ticker, not this.
//
// This is a floor, not a guarantee: the check only runs inside updateUI, which
// fires on poll or manual refresh, so the real self-heal period is
// max(iconRepushInterval, poll_interval_seconds). A very long poll interval
// stretches it accordingly — the alternative, a dedicated ticker, would restore
// the background goroutine this change removed.
const iconRepushInterval = 10 * time.Minute

// App ties together config, credentials, quota client, and systray.
type App struct {
	config           Config
	creds            *OAuthCredentials
	quota            *QuotaClient
	stats            *StatsStore
	resolver         *AccountResolver
	account          AccountInfo
	quit             chan struct{}             // closed on shutdown
	restartRequested atomic.Bool               // set before shutdown to trigger re-exec
	nextUpdate       atomic.Pointer[time.Time] // when pollLoop will next fetch
	fetchMu          sync.Mutex                // serializes refreshAccount+Fetch+record across goroutines
	uiMu             sync.Mutex                // serializes updateUI calls

	// Last values pushed to systray, to suppress redundant D-Bus signals: every
	// setter re-emits (and the panel re-renders) even when the value is unchanged.
	// Guarded by uiMu.
	lastTitles  map[*systray.MenuItem]string
	lastShown   map[*systray.MenuItem]bool
	lastIcon    []byte
	lastTooltip string
	lastPush    time.Time // when icon+tooltip were last pushed unconditionally

	// Indirection for the two package-level systray calls updateUI makes. Nil
	// means "call systray" — the production path. systray keeps its state in
	// package globals with no seam of its own, so recorders here are the only
	// way a test can assert which pushes updateUI actually performs; without
	// them the dedup and the iconRepushInterval self-heal are both unfalsifiable.
	setIcon    func([]byte)
	setTooltip func(string)

	// Parsed once at construction from config.ProviderMarkColor to avoid
	// re-parsing the hex string on every render tick. A.A == 0 means "no
	// override" (parseHexColor always returns A = 255 on success).
	providerMarkColor color.RGBA

	// Update state.
	updateMu      sync.Mutex
	updateVersion string // latest version when an update is available
	updatePhase   updatePhase

	// Menu items updated dynamically.
	mAccountEmail       *systray.MenuItem
	mAccountOrg         *systray.MenuItem
	mProvider           *systray.MenuItem
	mFiveHour           *systray.MenuItem
	mProjection         *systray.MenuItem
	mSaturation         *systray.MenuItem
	mSevenDay           *systray.MenuItem
	mSevenDayProjection *systray.MenuItem
	mSevenDaySaturation *systray.MenuItem
	mSevenDaySonnet     *systray.MenuItem
	mUpdated            *systray.MenuItem
	mNextUpdate         *systray.MenuItem
	mStats              *systray.MenuItem
	mRefresh            *systray.MenuItem
	mCheckUpdate        *systray.MenuItem
	mQuit               *systray.MenuItem
}

// NewApp creates an App from the given config and credentials.
func NewApp(cfg Config, creds *OAuthCredentials, client *http.Client, stats *StatsStore, resolver *AccountResolver) *App {
	a := &App{
		config:   cfg,
		creds:    creds,
		quota:    NewQuotaClient(creds, client),
		stats:    stats,
		resolver: resolver,
		quit:     make(chan struct{}),
	}
	// Config load already validated this via parseHexColor; a.A stays 0 when
	// no override is set, which renderIcon treats as "use the default accent".
	if cfg.ProviderMarkColor != "" {
		if c, err := parseHexColor(cfg.ProviderMarkColor); err == nil {
			a.providerMarkColor = c
		}
	}
	return a
}

// Run starts the systray. Blocks until the tray exits.
func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}

// Shutdown signals the app to stop.
func (a *App) Shutdown() {
	select {
	case <-a.quit:
		// already closed
	default:
		close(a.quit)
	}
	systray.Quit()
}

// onReady is called by systray when the tray is ready.
func (a *App) onReady() {
	systray.SetTitle("")
	systray.SetTooltip(providerQuotaTitle(a.creds.Provider()))

	// Create menu items.
	if a.config.ShowAccount {
		a.mAccountEmail = systray.AddMenuItem("", "Account email")
		a.mAccountEmail.Disable()
		a.mAccountEmail.Hide()
		a.mAccountOrg = systray.AddMenuItem("", "Organization name")
		a.mAccountOrg.Disable()
		a.mAccountOrg.Hide()
	}
	a.mProvider = systray.AddMenuItem(fmt.Sprintf("Provider: %s", providerDisplayName(a.creds.Provider())), "Active quota provider")
	a.mProvider.Disable()
	a.mFiveHour = systray.AddMenuItem("5h: --", "5-hour quota")
	a.mFiveHour.Disable()
	a.mProjection = systray.AddMenuItem("", "Projected utilization at reset")
	a.mProjection.Disable()
	a.mProjection.Hide()
	a.mSaturation = systray.AddMenuItem("", "Projected saturation time")
	a.mSaturation.Disable()
	a.mSaturation.Hide()
	a.mSevenDay = systray.AddMenuItem("7d: --", "7-day quota")
	a.mSevenDay.Disable()
	a.mSevenDayProjection = systray.AddMenuItem("", "Projected 7d utilization at reset")
	a.mSevenDayProjection.Disable()
	a.mSevenDayProjection.Hide()
	a.mSevenDaySaturation = systray.AddMenuItem("", "Projected 7d saturation time")
	a.mSevenDaySaturation.Disable()
	a.mSevenDaySaturation.Hide()
	a.mSevenDaySonnet = systray.AddMenuItem(extraQuotaLabel(QuotaState{Provider: a.creds.Provider()})+": --", "Additional quota bucket")
	a.mSevenDaySonnet.Disable()

	systray.AddSeparator()

	a.mUpdated = systray.AddMenuItem("Updated: --", "Last update time")
	a.mUpdated.Disable()
	a.mNextUpdate = systray.AddMenuItem("Next update: --", "Next scheduled quota refresh")
	a.mNextUpdate.Disable()
	if a.stats != nil {
		a.mStats = systray.AddMenuItem(fmt.Sprintf("Stats: %s", statsDBPath), "Stats database location")
		a.mStats.Disable()
	}
	a.mRefresh = systray.AddMenuItem("Refresh", "Refresh quota now")
	a.mCheckUpdate = systray.AddMenuItem(fmt.Sprintf("Check for Updates (current %s)", Version), "Check for a newer version")
	a.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Initial fetch + icon update. Schedule the first poll before rendering so
	// the "Next update" line is populated from the start, and hand the deadline
	// to pollLoop so the line and the timer cannot disagree.
	a.fetchCycle()
	next := a.scheduleNext(time.Duration(a.config.PollIntervalSeconds) * time.Second)
	a.updateUI()

	// Start background loops.
	go a.pollLoop(next)
	go a.eventLoop()
}

// onExit is called when the systray is shutting down.
func (a *App) onExit() {
	select {
	case <-a.quit:
	default:
		close(a.quit)
	}
	if a.stats != nil {
		_ = a.stats.Close()
	}
}

// eventLoop handles menu item clicks.
func (a *App) eventLoop() {
	for {
		select {
		case <-a.quit:
			return
		case <-a.mRefresh.ClickedCh:
			a.fetchCycle()
			a.updateUI()
		case <-a.mCheckUpdate.ClickedCh:
			a.handleUpdateClick()
		case <-a.mQuit.ClickedCh:
			a.Shutdown()
			return
		}
	}
}

// pollLoop periodically fetches quota and updates the UI. next is the first
// deadline, produced by onReady's scheduleNext so the "Next update" line shown
// at startup is the one this loop actually waits on. Carrying the deadline in a
// parameter rather than re-reading a.nextUpdate keeps it un-nil-able and leaves
// the atomic as a one-way channel to the UI.
func (a *App) pollLoop(next time.Time) {
	interval := time.Duration(a.config.PollIntervalSeconds) * time.Second

	for {
		// Wait on the stored deadline, not a fresh interval, so the "Next update"
		// line is exact rather than optimistic by the render duration. The initial
		// fetch already happened in onReady, so we wait before fetching.
		select {
		case <-a.quit:
			return
		case <-time.After(time.Until(next)):
		}

		a.fetchCycle()
		// Re-arm before rendering, otherwise updateUI would show the deadline that
		// just fired.
		next = a.scheduleNext(interval)
		a.updateUI()
	}
}

// scheduleNext records when pollLoop will next fetch, for the "Next update"
// line, and returns that deadline. A manual Refresh does not call this, so it
// never shifts the real schedule.
func (a *App) scheduleNext(interval time.Duration) time.Time {
	t := time.Now().Add(interval)
	a.nextUpdate.Store(&t)
	return t
}

// nextUpdateAt returns the next-fetch deadline as a wall-clock time, or nil if
// none is scheduled. The stored deadline's wall-clock component goes stale
// across a suspend or an NTP step, while its monotonic reading — the one
// time.After actually waits on — does not; re-deriving from the remaining
// monotonic duration keeps the displayed time honest in both cases.
func (a *App) nextUpdateAt() *time.Time {
	next := a.nextUpdate.Load()
	if next == nil {
		return nil
	}
	at := time.Now().Add(time.Until(*next))
	return &at
}

// fetchCycle runs the full refresh-account + fetch-quota + record cycle.
// Serialized via fetchMu so pollLoop and eventLoop don't race on a.account.
func (a *App) fetchCycle() {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	a.refreshAccount()
	if a.quota.Fetch() {
		// Codex email comes from the usage API, not a separate profile endpoint.
		if state := a.quota.State(); state.AccountEmail != "" {
			a.account.EmailAddress = state.AccountEmail
		}
		a.recordStats()
	} else {
		a.recordError()
	}
}

// refreshAccount resolves account identity, re-reading credentials if they changed.
// Must be called under fetchMu.
func (a *App) refreshAccount() {
	if a.resolver == nil {
		return
	}
	if a.stats == nil && !a.config.ShowAccount {
		return
	}
	snap, err := a.creds.ReloadAndSnapshot()
	if err != nil {
		log.Printf("Credential reload failed: %v", err)
		// Clear stale identity so fetches aren't attributed to wrong account.
		a.account = AccountInfo{}
		return
	}
	if !snap.Changed && a.account.AccountUUID != "" {
		return
	}
	a.account = a.resolver.Resolve(snap)
}

// recordStats records the current quota state if stats collection is enabled.
// Must be called under fetchMu.
func (a *App) recordStats() {
	if a.stats == nil {
		return
	}
	a.stats.RecordFetch(a.quota.State(), a.account.AccountUUID)
}

// recordError records a fetch error if stats collection is enabled.
// Must be called under fetchMu.
func (a *App) recordError() {
	if a.stats == nil {
		return
	}
	state := a.quota.State()
	if state.Error == "" {
		return
	}
	a.stats.RecordError(state.Provider, a.account.AccountUUID, state.ErrorType, state.HTTPStatus, state.Error)
}

// cacheChanged reports whether val differs from the last value recorded for mi
// in cache, updating the record. Pure map bookkeeping — the systray call stays
// with the caller; generic so title (string) and visibility (bool) share one
// gate.
func cacheChanged[T comparable](cache map[*systray.MenuItem]T, mi *systray.MenuItem, val T) bool {
	if prev, ok := cache[mi]; ok && prev == val {
		return false
	}
	cache[mi] = val
	return true
}

// setMenuTitle sets a menu item's title only when it changed, skipping the
// redundant D-Bus signal otherwise. Caller must hold uiMu.
//
// Every title write must go through here: a raw mi.SetTitle would desync the
// cache from the widget and make the *next* setMenuTitle swallow a real update.
// Use setMenuTitleLocked from goroutines that don't already hold uiMu.
func (a *App) setMenuTitle(mi *systray.MenuItem, s string) {
	if mi == nil {
		return
	}
	if a.lastTitles == nil {
		a.lastTitles = make(map[*systray.MenuItem]string)
	}
	if cacheChanged(a.lastTitles, mi, s) {
		mi.SetTitle(s)
	}
}

// setMenuTitleLocked is setMenuTitle for callers outside updateUI.
func (a *App) setMenuTitleLocked(mi *systray.MenuItem, s string) {
	a.uiMu.Lock()
	defer a.uiMu.Unlock()
	a.setMenuTitle(mi, s)
}

// setMenuVisible shows/hides a menu item only on a visibility change, skipping
// the redundant layout signal otherwise. Caller must hold uiMu.
func (a *App) setMenuVisible(mi *systray.MenuItem, show bool) {
	if mi == nil {
		return
	}
	if a.lastShown == nil {
		a.lastShown = make(map[*systray.MenuItem]bool)
	}
	if !cacheChanged(a.lastShown, mi, show) {
		return
	}
	if show {
		mi.Show()
	} else {
		mi.Hide()
	}
}

// shouldRepush reports whether this pass must bypass the icon/tooltip dedup so
// a silently dropped push cannot persist — see iconRepushInterval. A zero
// lastPush forces the first pass, which is the one most likely to race systray's
// own setup. Caller must hold uiMu.
func (a *App) shouldRepush(now time.Time) bool {
	return now.Sub(a.lastPush) >= iconRepushInterval
}

// pushIcon and pushTooltip send to systray unless a test substituted a recorder.
func (a *App) pushIcon(iconData []byte) {
	if a.setIcon != nil {
		a.setIcon(iconData)
		return
	}
	systray.SetIcon(iconData)
}

func (a *App) pushTooltip(tooltip string) {
	if a.setTooltip != nil {
		a.setTooltip(tooltip)
		return
	}
	systray.SetTooltip(tooltip)
}

// updateUI refreshes the icon and menu items from current state.
// Serialized via uiMu because pollLoop and eventLoop may call concurrently,
// and the shared font.Face used during rendering is not goroutine-safe.
func (a *App) updateUI() {
	// Snapshot account under fetchMu to avoid racing with refreshAccount.
	a.fetchMu.Lock()
	account := a.account
	a.fetchMu.Unlock()

	a.uiMu.Lock()
	defer a.uiMu.Unlock()
	state := a.quota.State()

	repush := a.shouldRepush(time.Now())

	// Update icon.
	img := renderIcon(state, a.config.Thresholds, RenderOptions{
		FontSize:             a.config.FontSize,
		IconSize:             a.config.IconSize,
		FontName:             a.config.FontName,
		HaloSize:             a.config.HaloSize,
		Indicator:            a.config.Indicator,
		ShowText:             configShowText(a.config),
		ProviderMark:         a.config.ProviderMark,
		ProviderMarkSize:     a.config.ProviderMarkSize,
		ProviderMarkPosition: a.config.ProviderMarkPosition,
		ProviderMarkColor:    a.providerMarkColor,
	})
	iconData, err := iconToBytes(img)
	if err != nil {
		log.Printf("Icon encode error: %v", err)
		// Mark the icon dirty so the first successful encode pushes even if it
		// produces the same bytes as the last one that made it out. Gating
		// lastPush on the push succeeding instead would pin repush to true and
		// re-emit the tooltip on every single pass.
		a.lastIcon = nil
	} else if repush || !bytes.Equal(iconData, a.lastIcon) {
		a.lastIcon = iconData
		a.pushIcon(iconData)
	}

	// Update tooltip.
	if tooltip := buildTooltip(state, a.creds.Provider()); repush || tooltip != a.lastTooltip {
		a.lastTooltip = tooltip
		a.pushTooltip(tooltip)
	}
	if repush {
		a.lastPush = time.Now()
	}

	// Update menu items.
	if a.mAccountEmail != nil {
		if account.EmailAddress != "" {
			a.setMenuTitle(a.mAccountEmail, "Acct: "+account.EmailAddress)
			a.setMenuVisible(a.mAccountEmail, true)
		} else {
			a.setMenuVisible(a.mAccountEmail, false)
		}
	}
	if a.mAccountOrg != nil {
		if account.OrganizationName != "" {
			a.setMenuTitle(a.mAccountOrg, "Org: "+account.OrganizationName)
			a.setMenuVisible(a.mAccountOrg, true)
		} else {
			a.setMenuVisible(a.mAccountOrg, false)
		}
	}
	if a.mProvider != nil {
		a.setMenuTitle(a.mProvider, fmt.Sprintf("Provider: %s", providerDisplayName(state.Provider)))
	}
	a.setMenuTitle(a.mFiveHour, formatQuotaLine("5h", state.FiveHour, state.FiveHourResets))
	if state.FiveHour != nil {
		if projLine := formatProjectionLine(state.FiveHourProjected); projLine != "" {
			a.setMenuTitle(a.mProjection, projLine)
			a.setMenuVisible(a.mProjection, true)
		} else {
			a.setMenuVisible(a.mProjection, false)
		}
		if satLine := formatSaturationLine(state.FiveHourSaturation); satLine != "" {
			a.setMenuTitle(a.mSaturation, satLine)
			a.setMenuVisible(a.mSaturation, true)
		} else {
			a.setMenuVisible(a.mSaturation, false)
		}
	} else {
		a.setMenuVisible(a.mProjection, false)
		a.setMenuVisible(a.mSaturation, false)
	}
	a.setMenuTitle(a.mSevenDay, formatQuotaLine("7d", state.SevenDay, state.SevenDayResets))
	if state.SevenDay != nil {
		if projLine := formatProjectionLine(state.SevenDayProjected); projLine != "" {
			a.setMenuTitle(a.mSevenDayProjection, projLine)
			a.setMenuVisible(a.mSevenDayProjection, true)
		} else {
			a.setMenuVisible(a.mSevenDayProjection, false)
		}
		if satLine := formatSaturationLine(state.SevenDaySaturation); satLine != "" {
			a.setMenuTitle(a.mSevenDaySaturation, satLine)
			a.setMenuVisible(a.mSevenDaySaturation, true)
		} else {
			a.setMenuVisible(a.mSevenDaySaturation, false)
		}
	} else {
		a.setMenuVisible(a.mSevenDayProjection, false)
		a.setMenuVisible(a.mSevenDaySaturation, false)
	}
	a.setMenuTitle(a.mSevenDaySonnet, formatQuotaLine(extraQuotaLabel(state), state.SevenDaySonnet, state.SevenDaySonnetResets))

	a.setMenuTitle(a.mUpdated, formatClockLine("Updated", state.LastUpdate))
	a.setMenuTitle(a.mNextUpdate, formatClockLine("Next update", a.nextUpdateAt()))
}

// handleUpdateClick dispatches the click based on current update phase.
func (a *App) handleUpdateClick() {
	a.updateMu.Lock()
	phase := a.updatePhase
	version := a.updateVersion
	a.updateMu.Unlock()

	switch phase {
	case updatePhaseCheck:
		a.doUpdateCheck()
	case updatePhaseReady:
		a.doApplyUpdate(version)
	case updatePhaseApplied:
		a.setMenuTitleLocked(a.mCheckUpdate, "Restarting...")
		a.mCheckUpdate.Disable()
		a.restartRequested.Store(true)
		a.Shutdown()
	}
}

// doUpdateCheck checks GitHub for a newer version and updates the menu.
func (a *App) doUpdateCheck() {
	a.setMenuTitleLocked(a.mCheckUpdate, "Checking...")
	a.mCheckUpdate.Disable()
	go func() {
		defer a.mCheckUpdate.Enable()

		log.Printf("Checking for updates (current: %s)...", Version)
		latest, err := fetchLatestVersion()
		if err != nil {
			log.Printf("Update check failed: %v", err)
			a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Update check failed: %v", err))
			return
		}

		switch semver.Compare(latest, Version) {
		case 1:
			log.Printf("Update available: %s", latest)
			a.updateMu.Lock()
			a.updateVersion = latest
			a.updatePhase = updatePhaseReady
			a.updateMu.Unlock()
			a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Update available: %s (current: %s)", latest, Version))
		case -1:
			log.Printf("Newer than latest release (%s)", latest)
			a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Newer than latest release (%s)", latest))
		default:
			log.Printf("Up to date (%s)", Version)
			a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Up to date (%s)", Version))
		}
	}()
}

// doApplyUpdate downloads and applies the given version.
func (a *App) doApplyUpdate(version string) {
	a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Updating to %s...", version))
	a.mCheckUpdate.Disable()
	go func() {
		if err := applyUpdate(version); err != nil {
			log.Printf("Update error: %v", err)
			a.setMenuTitleLocked(a.mCheckUpdate, fmt.Sprintf("Update failed: %v", err))
			// Reset to ready so user can retry.
			a.updateMu.Lock()
			a.updatePhase = updatePhaseReady
			a.updateMu.Unlock()
			a.mCheckUpdate.Enable()
			return
		}
		a.updateMu.Lock()
		a.updatePhase = updatePhaseApplied
		a.updateMu.Unlock()
		a.setMenuTitleLocked(a.mCheckUpdate, "Restart to apply update")
		a.mCheckUpdate.Enable()
	}()
}

// buildTooltip generates tooltip text from state.
func buildTooltip(state QuotaState, provider Provider) string {
	lines := providerQuotaTitle(provider)

	if state.Error != "" {
		lines += "\nError: " + state.Error
	} else {
		if state.FiveHour != nil {
			lines += "\n" + formatQuotaLine("5h", state.FiveHour, state.FiveHourResets)
			if state.FiveHourProjected != nil {
				lines += "\n" + formatProjectionLine(state.FiveHourProjected)
			}
			if state.FiveHourSaturation != nil {
				lines += "\n" + formatSaturationLine(state.FiveHourSaturation)
			}
		}
		if state.SevenDay != nil {
			lines += "\n" + formatQuotaLine("7d", state.SevenDay, state.SevenDayResets)
			if state.SevenDayProjected != nil {
				lines += "\n" + formatProjectionLine(state.SevenDayProjected)
			}
			if state.SevenDaySaturation != nil {
				lines += "\n" + formatSaturationLine(state.SevenDaySaturation)
			}
		}
		if state.SevenDaySonnet != nil {
			lines += "\n" + formatQuotaLine(extraQuotaLabel(state), state.SevenDaySonnet, state.SevenDaySonnetResets)
		}
	}

	if state.LastUpdate != nil {
		lines += "\n" + formatClockLine("Updated", state.LastUpdate)
	}

	return lines
}

func extraQuotaLabel(state QuotaState) string {
	if state.SevenDaySonnetLabel != "" {
		return state.SevenDaySonnetLabel
	}
	if state.Provider == ProviderCodex {
		return "Additional"
	}
	return "Sonnet 7d"
}
