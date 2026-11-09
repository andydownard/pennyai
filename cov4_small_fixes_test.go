package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// place_id comes from the client and is placed in a Google URL path: it must be
// escaped so "/" and "?" cannot change the upstream path or query.
func TestPlacesDetail_EscapesPlaceID(t *testing.T) {
	server := newTestServer(t)
	server.config.GooglePlacesAPIKey = "places-key"
	cookie, _ := loginUser(t, server, "5550010003")

	var gotPath, gotQuery string
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		return stubResponse(200, `{"formattedAddress":"1 Main St"}`), nil
	})

	rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=a%2Fb%3Fc%3D1", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}
	if gotPath != "/v1/places/a%2Fb%3Fc=1" && gotPath != "/v1/places/a%2Fb%3Fc%3D1" {
		t.Errorf("upstream path = %q, want the place_id escaped as one segment", gotPath)
	}
	if gotQuery != "" {
		t.Errorf("upstream query = %q, want none", gotQuery)
	}
}

// With no Logo.dev token nothing is fetched, so the warm-up must not sleep per brand.
func TestWarmLogoCache_NoTokenDoesNotSleep(t *testing.T) {
	server := newTestServer(t)
	clearSeededBrands(t, server)
	server.config.LogoDevToken = ""
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("no outbound call expected: %s", r.URL)
	})
	for i := 0; i < 6; i++ {
		server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)",
			fmt.Sprintf("Brand%d", i), strings.ToLower(fmt.Sprintf("brand%d.com", i)))
	}
	start := time.Now()
	server.warmLogoCache()
	if d := time.Since(start); d > time.Second {
		t.Errorf("warm-up took %v with no token; want no per-brand sleep", d)
	}
}
