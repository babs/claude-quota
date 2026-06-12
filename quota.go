package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var usageURL = "https://api.anthropic.com/api/oauth/usage"

// Undocumented internal endpoint used by the Codex CLI TUI rate-limit poller.
var codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

// userAgent is the User-Agent header sent to the Anthropic API.
// Shared between quota and profile API clients.
const userAgent = "claude-code/2.0.31"

// Error type constants for classifying fetch failures.
const (
	ErrTypeCredential = "credential"
	ErrTypeHTTP       = "http"
	ErrTypeNetwork    = "network"
	ErrTypeParse      = "parse"
)

// QuotaState holds the current quota snapshot.
type QuotaState struct {
	Provider             Provider
	FiveHour             *float64
	FiveHourResets       *time.Time
	FiveHourProjected    *float64   // projected 5h utilization at window reset
	FiveHourSaturation   *time.Time // projected time when 5h quota hits 100%
	SevenDay             *float64
	SevenDayResets       *time.Time
	SevenDayProjected    *float64   // projected 7d utilization at window reset
	SevenDaySaturation   *time.Time // projected time when 7d quota hits 100%
	SevenDaySonnet       *float64
	SevenDaySonnetResets *time.Time
	SevenDaySonnetLabel  string
	AccountEmail         string // populated from Codex usage response
	LastUpdate           *time.Time
	Error                string
	ErrorType            string // credential, http, network, parse
	HTTPStatus           int    // HTTP status code when ErrorType is "http"
	TokenExpired         bool
}

// usageResponse matches the JSON returned by the usage API.
type usageResponse struct {
	FiveHour       *usageBucket `json:"five_hour"`
	SevenDay       *usageBucket `json:"seven_day"`
	SevenDaySonnet *usageBucket `json:"seven_day_sonnet"`
}

type usageBucket struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type codexUsageResponse struct {
	AccountID           string               `json:"account_id"`
	Email               string               `json:"email"`
	PlanType            string               `json:"plan_type"`
	RateLimit           *codexRateLimit      `json:"rate_limit"`
	CodeReviewRateLimit *codexRateLimit      `json:"code_review_rate_limit"`
	AdditionalLimits    []codexNamedRateInfo `json:"additional_rate_limits"`
}

type codexRateLimit struct {
	Allowed         bool         `json:"allowed"`
	LimitReached    bool         `json:"limit_reached"`
	PrimaryWindow   *codexWindow `json:"primary_window"`
	SecondaryWindow *codexWindow `json:"secondary_window"`
}

type codexNamedRateInfo struct {
	Name            string       `json:"name"`
	Title           string       `json:"title"`
	PrimaryWindow   *codexWindow `json:"primary_window"`
	SecondaryWindow *codexWindow `json:"secondary_window"`
	Window          *codexWindow `json:"window"`
}

type codexWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds *int     `json:"limit_window_seconds"`
	ResetAfterSeconds  *int     `json:"reset_after_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

// QuotaClient fetches and stores quota state.
type QuotaClient struct {
	mu     sync.RWMutex
	state  QuotaState
	creds  *OAuthCredentials
	client *http.Client
}

// NewQuotaClient creates a new quota client.
func NewQuotaClient(creds *OAuthCredentials, client *http.Client) *QuotaClient {
	return &QuotaClient{
		creds:  creds,
		client: client,
	}
}

// State returns a consistent snapshot of the current quota state.
func (qc *QuotaClient) State() QuotaState {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return qc.state
}

// Fetch fetches quota from the Anthropic OAuth usage API. Returns true on success.
func (qc *QuotaClient) Fetch() bool {
	provider := qc.creds.Provider()
	token, err := qc.creds.GetAccessToken()
	if err != nil {
		log.Printf("Credential error: %v", err)
		qc.mu.Lock()
		qc.state = QuotaState{
			Provider:     provider,
			Error:        truncate(err.Error(), 50),
			ErrorType:    ErrTypeCredential,
			TokenExpired: errors.Is(err, ErrTokenExpired),
		}
		qc.mu.Unlock()
		return false
	}

	req, err := http.NewRequest("GET", qc.usageURL(), nil)
	if err != nil {
		log.Printf("Request error: %v", err)
		qc.setErrorTyped(truncate(err.Error(), 50), ErrTypeNetwork, 0)
		return false
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if provider == ProviderCodex {
		if accountID := qc.creds.AccountID(); accountID != "" {
			req.Header.Set("ChatGPT-Account-Id", accountID)
		}
		req.Header.Set("User-Agent", "claude-quota")
	} else {
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	}

	resp, err := qc.client.Do(req)
	if err != nil {
		log.Printf("Fetch failed: %v", err)
		qc.setErrorTyped(truncate(err.Error(), 50), ErrTypeNetwork, 0)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var msg string
		switch resp.StatusCode {
		case 401:
			msg = fmt.Sprintf("Token invalid \u2014 run '%s'", loginCommand(provider))
		case 403:
			if provider == ProviderCodex {
				msg = "Access forbidden by ChatGPT backend"
			} else {
				msg = "Scope missing user:profile"
			}
		default:
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		log.Printf("API error: %s", msg)
		qc.setErrorTyped(msg, ErrTypeHTTP, resp.StatusCode)
		return false
	}

	newState, err := qc.decodeState(io.LimitReader(resp.Body, 1<<20), provider)
	if err != nil {
		log.Printf("JSON parse failed: %v", err)
		qc.setErrorTyped(truncate(err.Error(), 50), ErrTypeParse, 0)
		return false
	}

	qc.mu.Lock()
	qc.state = newState
	qc.mu.Unlock()
	return true
}

func (qc *QuotaClient) usageURL() string {
	if qc.creds.Provider() == ProviderCodex {
		return codexUsageURL
	}
	return usageURL
}

func (qc *QuotaClient) decodeState(r io.Reader, provider Provider) (QuotaState, error) {
	switch provider {
	case ProviderCodex:
		var data codexUsageResponse
		if err := json.NewDecoder(r).Decode(&data); err != nil {
			return QuotaState{}, err
		}
		return buildCodexState(data), nil
	default:
		var data usageResponse
		if err := json.NewDecoder(r).Decode(&data); err != nil {
			return QuotaState{}, err
		}
		return buildClaudeState(data), nil
	}
}

func buildClaudeState(data usageResponse) QuotaState {
	newState := QuotaState{Provider: ProviderClaude}
	parseBucket(data.FiveHour, &newState.FiveHour, &newState.FiveHourResets)
	parseBucket(data.SevenDay, &newState.SevenDay, &newState.SevenDayResets)
	parseBucket(data.SevenDaySonnet, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets)
	newState.SevenDaySonnetLabel = "Sonnet 7d"

	now := time.Now().UTC()
	newState.LastUpdate = &now
	populateWindowForecasts(&newState, now, fiveHourWindow, sevenDayWindow)
	return newState
}

func buildCodexState(data codexUsageResponse) QuotaState {
	newState := QuotaState{Provider: ProviderCodex, AccountEmail: data.Email}

	var fiveHourWindowDuration = fiveHourWindow
	var sevenDayWindowDuration = sevenDayWindow
	if data.RateLimit != nil {
		fiveHourWindowDuration = parseCodexWindow(data.RateLimit.PrimaryWindow, &newState.FiveHour, &newState.FiveHourResets, fiveHourWindow)
		sevenDayWindowDuration = parseCodexWindow(data.RateLimit.SecondaryWindow, &newState.SevenDay, &newState.SevenDayResets, sevenDayWindow)
	}

	if data.CodeReviewRateLimit != nil {
		if parseCodexWindow(data.CodeReviewRateLimit.SecondaryWindow, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets, 0) == 0 {
			parseCodexWindow(data.CodeReviewRateLimit.PrimaryWindow, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets, 0)
		}
		if newState.SevenDaySonnet != nil {
			newState.SevenDaySonnetLabel = "Code review"
		}
	}
	if newState.SevenDaySonnet == nil {
		for _, extra := range data.AdditionalLimits {
			if parseCodexWindow(extra.Window, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets, 0) == 0 &&
				parseCodexWindow(extra.SecondaryWindow, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets, 0) == 0 {
				parseCodexWindow(extra.PrimaryWindow, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets, 0)
			}
			if newState.SevenDaySonnet != nil {
				if extra.Title != "" {
					newState.SevenDaySonnetLabel = extra.Title
				} else if extra.Name != "" {
					newState.SevenDaySonnetLabel = extra.Name
				}
				break
			}
		}
	}

	now := time.Now().UTC()
	newState.LastUpdate = &now
	populateWindowForecasts(&newState, now, fiveHourWindowDuration, sevenDayWindowDuration)
	return newState
}

func populateWindowForecasts(state *QuotaState, now time.Time, fiveWindow, sevenWindow time.Duration) {
	if state.FiveHour != nil && state.FiveHourResets != nil {
		state.FiveHourProjected = computeProjection(*state.FiveHour, *state.FiveHourResets, now, fiveWindow)
	}
	if state.FiveHourProjected != nil && *state.FiveHourProjected > 100 {
		state.FiveHourSaturation = computeSaturationTime(*state.FiveHour, *state.FiveHourResets, now, fiveWindow)
	}
	if state.SevenDay != nil && state.SevenDayResets != nil {
		state.SevenDayProjected = computeProjection(*state.SevenDay, *state.SevenDayResets, now, sevenWindow)
	}
	if state.SevenDayProjected != nil && *state.SevenDayProjected > 100 {
		state.SevenDaySaturation = computeSaturationTime(*state.SevenDay, *state.SevenDayResets, now, sevenWindow)
	}
}

// setErrorTyped resets state to an error-only snapshot with classification.
func (qc *QuotaClient) setErrorTyped(msg, errType string, httpStatus int) {
	qc.mu.Lock()
	qc.state = QuotaState{
		Provider:   qc.creds.Provider(),
		Error:      msg,
		ErrorType:  errType,
		HTTPStatus: httpStatus,
	}
	qc.mu.Unlock()
}

// setError resets state to an error-only snapshot (untyped, for backward compat).
func (qc *QuotaClient) setError(msg string) {
	qc.setErrorTyped(msg, "", 0)
}

// parseBucket extracts utilization and reset time from an API bucket.
func parseBucket(bucket *usageBucket, util **float64, resets **time.Time) {
	if bucket == nil {
		return
	}
	if bucket.Utilization != nil {
		v := *bucket.Utilization
		*util = &v
	}
	if bucket.ResetsAt != nil {
		t, err := time.Parse(time.RFC3339, *bucket.ResetsAt)
		if err != nil {
			log.Printf("Failed to parse reset time %q: %v", *bucket.ResetsAt, err)
			return
		}
		*resets = &t
	}
}

func parseCodexWindow(window *codexWindow, util **float64, resets **time.Time, fallback time.Duration) time.Duration {
	if window == nil {
		return 0
	}
	if window.UsedPercent != nil {
		v := *window.UsedPercent
		*util = &v
	}
	if window.ResetAt != nil {
		t := time.Unix(*window.ResetAt, 0).UTC()
		*resets = &t
	}
	if window.LimitWindowSeconds != nil && *window.LimitWindowSeconds > 0 {
		return time.Duration(*window.LimitWindowSeconds) * time.Second
	}
	return fallback
}

// fiveHourWindow is the assumed duration of the 5-hour quota window.
// This value is not derivable from the API (which only returns resets_at).
// If Anthropic changes the window duration, this constant must be updated.
const fiveHourWindow = 5 * time.Hour

// sevenDayWindow is the assumed duration of the 7-day quota window.
// Same caveat as fiveHourWindow: not derivable from the API.
const sevenDayWindow = 7 * 24 * time.Hour

// computeProjection estimates utilization at window reset by extrapolating the
// average consumption rate over the elapsed portion of the window. Returns nil
// when the window hasn't meaningfully started or has already ended.
//
// Formula: projected = current * windowDuration / timeElapsed
// where timeElapsed = windowDuration - timeUntilReset.
func computeProjection(current float64, resetsAt time.Time, now time.Time, windowDuration time.Duration) *float64 {
	if current <= 0 || !resetsAt.After(now) || windowDuration <= 0 {
		return nil
	}
	timeUntilReset := resetsAt.Sub(now)
	timeElapsed := windowDuration - timeUntilReset
	if timeElapsed <= 0 {
		return nil
	}
	projected := current * windowDuration.Seconds() / timeElapsed.Seconds()
	return &projected
}

// computeSaturationTime estimates when utilization will reach 100%, based on
// the average consumption rate over the elapsed portion of the window. Returns
// nil when saturation won't occur before reset or inputs are invalid.
func computeSaturationTime(current float64, resetsAt time.Time, now time.Time, windowDuration time.Duration) *time.Time {
	if current <= 0 || current >= 100 || !resetsAt.After(now) || windowDuration <= 0 {
		return nil
	}
	timeUntilReset := resetsAt.Sub(now)
	timeElapsed := windowDuration - timeUntilReset
	if timeElapsed <= 0 {
		return nil
	}
	// rate = current / timeElapsed; timeToSaturation = (100 - current) / rate
	timeToSaturation := time.Duration(float64(timeElapsed) * (100 - current) / current)
	saturation := now.Add(timeToSaturation)
	if !saturation.Before(resetsAt) {
		return nil
	}
	return &saturation
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
