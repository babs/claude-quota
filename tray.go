package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
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

// App ties together config, credentials, quota client, and systray.
type App struct {
	config           Config
	creds            *OAuthCredentials
	quota            *QuotaClient
	stats            *StatsStore
	resolver         *AccountResolver
	account          AccountInfo
	quit             chan struct{} // closed on shutdown
	restartRequested bool          // set before shutdown to trigger re-exec
	fetchMu          sync.Mutex    // serializes refreshAccount+Fetch+record across goroutines
	uiMu             sync.Mutex    // serializes updateUI calls

	// Update state.
	updateMu      sync.Mutex
	updateVersion string // latest version when an update is available
	updatePhase   updatePhase

	// Menu items updated dynamically.
	mAccountEmail       *systray.MenuItem
	mAccountOrg         *systray.MenuItem
	mFiveHour           *systray.MenuItem
	mProjection         *systray.MenuItem
	mSaturation         *systray.MenuItem
	mSevenDay           *systray.MenuItem
	mSevenDayProjection *systray.MenuItem
	mSevenDaySaturation *systray.MenuItem
	mSevenDaySonnet     *systray.MenuItem
	mUpdated            *systray.MenuItem
	mStats              *systray.MenuItem
	mRefresh            *systray.MenuItem
	mCheckUpdate        *systray.MenuItem
	mQuit               *systray.MenuItem
}

// NewApp creates an App from the given config and credentials.
func NewApp(cfg Config, creds *OAuthCredentials, client *http.Client, stats *StatsStore, resolver *AccountResolver) *App {
	return &App{
		config:   cfg,
		creds:    creds,
		quota:    NewQuotaClient(creds, client),
		stats:    stats,
		resolver: resolver,
		quit:     make(chan struct{}),
	}
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
	systray.SetTooltip("Claude Quota")

	// Create menu items.
	if a.config.ShowAccount {
		a.mAccountEmail = systray.AddMenuItem("", "Account email")
		a.mAccountEmail.Disable()
		a.mAccountEmail.Hide()
		a.mAccountOrg = systray.AddMenuItem("", "Organization name")
		a.mAccountOrg.Disable()
		a.mAccountOrg.Hide()
	}
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
	a.mSevenDaySonnet = systray.AddMenuItem("Sonnet 7d: --", "7-day Sonnet quota")
	a.mSevenDaySonnet.Disable()

	systray.AddSeparator()

	a.mUpdated = systray.AddMenuItem("Updated: --", "Last update time")
	a.mUpdated.Disable()
	if a.stats != nil {
		a.mStats = systray.AddMenuItem(fmt.Sprintf("Stats: %s", statsDBPath), "Stats database location")
		a.mStats.Disable()
	}
	a.mRefresh = systray.AddMenuItem("Refresh", "Refresh quota now")
	a.mCheckUpdate = systray.AddMenuItem(fmt.Sprintf("Check for Updates (current %s)", Version), "Check for a newer version")
	a.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Initial fetch + icon update.
	a.fetchCycle()
	a.updateUI()

	// Start background loops.
	go a.pollLoop()
	go a.updatedTicker()
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
		a.stats.Close()
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

// pollLoop periodically fetches quota and updates the UI.
func (a *App) pollLoop() {
	interval := time.Duration(a.config.PollIntervalSeconds) * time.Second

	// Wait first — initial fetch already happened in onReady.
	select {
	case <-a.quit:
		return
	case <-time.After(interval):
	}

	for {
		a.fetchCycle()
		a.updateUI()

		select {
		case <-a.quit:
			return
		case <-time.After(interval):
		}
	}
}

// updatedTicker refreshes the "Updated: Xs ago" menu item every 10 seconds.
func (a *App) updatedTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.quit:
			return
		case <-ticker.C:
			state := a.quota.State()
			a.mUpdated.SetTitle(formatUpdatedAgo(state.LastUpdate))
		}
	}
}

// fetchCycle runs the full refresh-account + fetch-quota + record cycle.
// Serialized via fetchMu so pollLoop and eventLoop don't race on a.account.
func (a *App) fetchCycle() {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	a.refreshAccount()
	if a.quota.Fetch() {
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
	a.stats.RecordError(a.account.AccountUUID, state.ErrorType, state.HTTPStatus, state.Error)
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

	// Update icon.
	img := renderIcon(state, a.config.Thresholds, RenderOptions{
		FontSize:  a.config.FontSize,
		IconSize:  a.config.IconSize,
		FontName:  a.config.FontName,
		HaloSize:  a.config.HaloSize,
		Indicator: a.config.Indicator,
		ShowText:  configShowText(a.config),
	})
	iconData, err := iconToBytes(img)
	if err != nil {
		log.Printf("Icon encode error: %v", err)
	} else {
		systray.SetIcon(iconData)
	}

	// Update tooltip.
	systray.SetTooltip(buildTooltip(state))

	// Update menu items.
	if a.mAccountEmail != nil {
		if account.EmailAddress != "" {
			a.mAccountEmail.SetTitle("Acct: " + account.EmailAddress)
			a.mAccountEmail.Show()
		} else {
			a.mAccountEmail.Hide()
		}
	}
	if a.mAccountOrg != nil {
		if account.OrganizationName != "" {
			a.mAccountOrg.SetTitle("Org: " + account.OrganizationName)
			a.mAccountOrg.Show()
		} else {
			a.mAccountOrg.Hide()
		}
	}
	a.mFiveHour.SetTitle(formatQuotaLine("5h", state.FiveHour, state.FiveHourResets))
	if state.FiveHour != nil {
		if projLine := formatProjectionLine(state.FiveHourProjected); projLine != "" {
			a.mProjection.SetTitle(projLine)
			a.mProjection.Show()
		} else {
			a.mProjection.Hide()
		}
		if satLine := formatSaturationLine(state.FiveHourSaturation); satLine != "" {
			a.mSaturation.SetTitle(satLine)
			a.mSaturation.Show()
		} else {
			a.mSaturation.Hide()
		}
	} else {
		a.mProjection.Hide()
		a.mSaturation.Hide()
	}
	a.mSevenDay.SetTitle(formatQuotaLine("7d", state.SevenDay, state.SevenDayResets))
	if state.SevenDay != nil {
		if projLine := formatProjectionLine(state.SevenDayProjected); projLine != "" {
			a.mSevenDayProjection.SetTitle(projLine)
			a.mSevenDayProjection.Show()
		} else {
			a.mSevenDayProjection.Hide()
		}
		if satLine := formatSaturationLine(state.SevenDaySaturation); satLine != "" {
			a.mSevenDaySaturation.SetTitle(satLine)
			a.mSevenDaySaturation.Show()
		} else {
			a.mSevenDaySaturation.Hide()
		}
	} else {
		a.mSevenDayProjection.Hide()
		a.mSevenDaySaturation.Hide()
	}
	a.mSevenDaySonnet.SetTitle(formatQuotaLine("Sonnet 7d", state.SevenDaySonnet, state.SevenDaySonnetResets))

	a.mUpdated.SetTitle(formatUpdatedAgo(state.LastUpdate))
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
		a.mCheckUpdate.SetTitle("Restarting...")
		a.mCheckUpdate.Disable()
		a.restartRequested = true
		a.Shutdown()
	}
}

// doUpdateCheck checks GitHub for a newer version and updates the menu.
func (a *App) doUpdateCheck() {
	a.mCheckUpdate.SetTitle("Checking...")
	a.mCheckUpdate.Disable()
	go func() {
		defer a.mCheckUpdate.Enable()

		log.Printf("Checking for updates (current: %s)...", Version)
		latest, err := fetchLatestVersion()
		if err != nil {
			log.Printf("Update check failed: %v", err)
			a.mCheckUpdate.SetTitle(fmt.Sprintf("Update check failed: %v", err))
			return
		}

		switch semver.Compare(latest, Version) {
		case 1:
			log.Printf("Update available: %s", latest)
			a.updateMu.Lock()
			a.updateVersion = latest
			a.updatePhase = updatePhaseReady
			a.updateMu.Unlock()
			a.mCheckUpdate.SetTitle(fmt.Sprintf("Update available: %s (current: %s)", latest, Version))
		case -1:
			log.Printf("Newer than latest release (%s)", latest)
			a.mCheckUpdate.SetTitle(fmt.Sprintf("Newer than latest release (%s)", latest))
		default:
			log.Printf("Up to date (%s)", Version)
			a.mCheckUpdate.SetTitle(fmt.Sprintf("Up to date (%s)", Version))
		}
	}()
}

// doApplyUpdate downloads and applies the given version.
func (a *App) doApplyUpdate(version string) {
	a.mCheckUpdate.SetTitle(fmt.Sprintf("Updating to %s...", version))
	a.mCheckUpdate.Disable()
	go func() {
		if err := applyUpdate(version); err != nil {
			log.Printf("Update error: %v", err)
			a.mCheckUpdate.SetTitle(fmt.Sprintf("Update failed: %v", err))
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
		a.mCheckUpdate.SetTitle("Restart to apply update")
		a.mCheckUpdate.Enable()
	}()
}

// buildTooltip generates tooltip text from state.
func buildTooltip(state QuotaState) string {
	lines := "Claude Quota"

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
			lines += "\n" + formatQuotaLine("Sonnet 7d", state.SevenDaySonnet, state.SevenDaySonnetResets)
		}
	}

	if state.LastUpdate != nil {
		local := state.LastUpdate.Local()
		lines += fmt.Sprintf("\nUpdated: %s", local.Format("15:04:05"))
	}

	return lines
}
