package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var usageURL = "https://api.anthropic.com/api/oauth/usage"

// QuotaState holds the current quota snapshot.
type QuotaState struct {
	FiveHour             *float64
	FiveHourResets       *time.Time
	FiveHourProjected    *float64   // projected 5h utilization at window reset
	FiveHourSaturation   *time.Time // projected time when 5h quota hits 100%
	SevenDay             *float64
	SevenDayResets       *time.Time
	SevenDaySonnet       *float64
	SevenDaySonnetResets *time.Time
	LastUpdate           *time.Time
	Error                string
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
	token, err := qc.creds.GetAccessToken()
	if err != nil {
		log.Printf("Credential error: %v", err)
		qc.mu.Lock()
		qc.state = QuotaState{
			Error:        truncate(err.Error(), 50),
			TokenExpired: errors.Is(err, ErrTokenExpired),
		}
		qc.mu.Unlock()
		return false
	}

	req, err := http.NewRequest("GET", usageURL, nil)
	if err != nil {
		log.Printf("Request error: %v", err)
		qc.setError(truncate(err.Error(), 50))
		return false
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "claude-code/2.0.31")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")

	resp, err := qc.client.Do(req)
	if err != nil {
		log.Printf("Fetch failed: %v", err)
		qc.setError(truncate(err.Error(), 50))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var msg string
		switch resp.StatusCode {
		case 401:
			msg = "Token invalid \u2014 run 'claude login'"
		case 403:
			msg = "Scope missing user:profile"
		default:
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		log.Printf("API error: %s", msg)
		qc.setError(msg)
		return false
	}

	var data usageResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("JSON parse failed: %v", err)
		qc.setError(truncate(err.Error(), 50))
		return false
	}

	newState := QuotaState{}
	parseBucket(data.FiveHour, &newState.FiveHour, &newState.FiveHourResets)
	parseBucket(data.SevenDay, &newState.SevenDay, &newState.SevenDayResets)
	parseBucket(data.SevenDaySonnet, &newState.SevenDaySonnet, &newState.SevenDaySonnetResets)

	now := time.Now().UTC()
	newState.LastUpdate = &now

	// Compute 5h projection: extrapolate average consumption rate to end of window.
	if newState.FiveHour != nil && newState.FiveHourResets != nil {
		newState.FiveHourProjected = computeProjection(
			*newState.FiveHour, *newState.FiveHourResets, now, fiveHourWindow,
		)
	}

	// Compute saturation time when projected > 100%.
	if newState.FiveHourProjected != nil && *newState.FiveHourProjected > 100 {
		newState.FiveHourSaturation = computeSaturationTime(
			*newState.FiveHour, *newState.FiveHourResets, now, fiveHourWindow,
		)
	}

	qc.mu.Lock()
	qc.state = newState
	qc.mu.Unlock()
	return true
}

// setError resets state to an error-only snapshot.
func (qc *QuotaClient) setError(msg string) {
	qc.mu.Lock()
	qc.state = QuotaState{Error: msg}
	qc.mu.Unlock()
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

// fiveHourWindow is the assumed duration of the 5-hour quota window.
// This value is not derivable from the API (which only returns resets_at).
// If Anthropic changes the window duration, this constant must be updated.
const fiveHourWindow = 5 * time.Hour

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

// computeSaturationTime estimates when 5h utilization will reach 100%, based on
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
