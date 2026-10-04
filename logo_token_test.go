package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Logo.dev's image CDN answers 401 to a secret key. With both keys configured
// the fetch must still send the publishable token.
func TestFetchBrandLogo_SendsPublishableTokenEvenWhenSecretIsSet(t *testing.T) {
	var gotToken, gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		gotReferer = r.Header.Get("Referer")
		if gotToken != "pk_public" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(bytes.Repeat([]byte{0x89}, 400))
	}))
	defer srv.Close()
	old := logoDevImageBaseURL
	logoDevImageBaseURL = srv.URL + "/"
	defer func() { logoDevImageBaseURL = old }()

	data, ct := fetchBrandLogo("kraftheinz.com", "sk_secret", "pk_public")
	if gotToken != "pk_public" {
		t.Fatalf("token sent to the image CDN = %q, want the publishable token", gotToken)
	}
	if gotReferer == "" {
		t.Error("publishable token must be sent with a Referer")
	}
	if len(data) == 0 || ct != "image/png" {
		t.Errorf("expected logo bytes, got %d bytes (%q)", len(data), ct)
	}
}

func TestFetchBrandLogo_SecretAloneFetchesNothing(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	old := logoDevImageBaseURL
	logoDevImageBaseURL = srv.URL + "/"
	defer func() { logoDevImageBaseURL = old }()

	if data, _ := fetchBrandLogo("kraftheinz.com", "sk_secret", ""); data != nil || called {
		t.Error("a secret key alone must not be sent to the image CDN")
	}
}
