package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"fyne.io/systray"
)

// App ties together config, credentials, quota client, and systray.
type App struct {
	config Config
	quota  *QuotaClient
	quit   chan struct{} // closed on shutdown
	uiMu   sync.Mutex    // serializes updateUI calls

	// Menu items updated dynamically.
	mFiveHour           *systray.MenuItem
	mProjection         *systray.MenuItem
	mSaturation         *systray.MenuItem
	mSevenDay           *systray.MenuItem
	mSevenDayProjection *systray.MenuItem
	mSevenDaySaturation *systray.MenuItem
	mSevenDaySonnet     *systray.MenuItem
	mUpdated            *systray.MenuItem
	mRefresh            *systray.MenuItem
	mQuit               *systray.MenuItem
}

// NewApp creates an App from the given config and credentials.
func NewApp(cfg Config, creds *OAuthCredentials, client *http.Client) *App {
	return &App{
		config: cfg,
		quota:  NewQuotaClient(creds, client),
		quit:   make(chan struct{}),
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
	a.mRefresh = systray.AddMenuItem("Refresh", "Refresh quota now")
	a.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Initial fetch + icon update.
	a.quota.Fetch()
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
}

// eventLoop handles menu item clicks.
func (a *App) eventLoop() {
	for {
		select {
		case <-a.quit:
			return
		case <-a.mRefresh.ClickedCh:
			a.quota.Fetch()
			a.updateUI()
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
		a.quota.Fetch()
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

// updateUI refreshes the icon and menu items from current state.
// Serialized via uiMu because pollLoop and eventLoop may call concurrently,
// and the shared font.Face used during rendering is not goroutine-safe.
func (a *App) updateUI() {
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
