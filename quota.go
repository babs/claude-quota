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

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
