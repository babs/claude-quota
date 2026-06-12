package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchLatestVersion_LimitsResponseBody(t *testing.T) {
	origClient := updateHTTPClient
	updateHTTPClient = &http.Client{Timeout: 5 * time.Second}
	defer func() {
		updateHTTPClient = origClient
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"name":"v1.2.3","padding":"`)
		_, _ = fmt.Fprint(w, strings.Repeat("x", latestReleaseResponseLimit))
		_, _ = fmt.Fprint(w, `"}`)
	}))
	defer server.Close()

	origBaseURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() {
		githubAPIBaseURL = origBaseURL
	}()

	version, err := fetchLatestVersion()
	if err == nil {
		t.Fatalf("fetchLatestVersion() error = nil, want parse failure for oversized response (version %q)", version)
	}
	if !strings.Contains(err.Error(), "parse failed") {
		t.Fatalf("fetchLatestVersion() error = %v, want parse failure", err)
	}
}
