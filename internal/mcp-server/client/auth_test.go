package client

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestAuthClientCallAPIAndValidateToken(t *testing.T) {
	restore := stubAuthHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://colink.test/api/callbacks/validate" {
			t.Fatalf("url = %s", req.URL.String())
		}
		if req.Method != http.MethodGet || req.Header.Get("X-Invocation-ID") != "inv-1" || req.Header.Get("X-Callback-Token") != "token" {
			t.Fatalf("request method=%s headers=%#v", req.Method, req.Header)
		}
		return authTextResponse(http.StatusOK, `{"ok":true}`), nil
	})
	defer restore()

	authClient := NewAuthClient("https://colink.test", "inv-1", "token")
	if err := authClient.ValidateToken(); err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
}

func TestAuthClientCallAPIWithBodyAndErrors(t *testing.T) {
	restore := stubAuthHTTP(t, func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if req.URL.Path != "/api/callbacks/post-message" || !strings.Contains(string(body), "hello") {
			t.Fatalf("request path=%s body=%s", req.URL.Path, body)
		}
		return authTextResponse(http.StatusTeapot, "short and stout"), nil
	})
	defer restore()

	authClient := NewAuthClient("https://colink.test", "inv-1", "token")
	if _, err := authClient.CallAPI(http.MethodPost, "/post-message", map[string]string{"message": "hello"}); err == nil || !strings.Contains(err.Error(), "418") {
		t.Fatalf("CallAPI should surface status errors, got %v", err)
	}
	authClient = NewAuthClient("://bad-url", "inv-1", "token")
	if _, err := authClient.CallAPI(http.MethodGet, "/validate", nil); err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("CallAPI should fail on bad url, got %v", err)
	}
}

func stubAuthHTTP(t *testing.T, handler func(*http.Request) (*http.Response, error)) func() {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = authRoundTripFunc(handler)
	return func() {
		http.DefaultTransport = original
	}
}

type authRoundTripFunc func(*http.Request) (*http.Response, error)

func (f authRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func authTextResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
