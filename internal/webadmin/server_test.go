package webadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeLoopbackAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "ipv4", input: "127.0.0.1:18765", want: "127.0.0.1:18765"},
		{name: "localhost", input: "localhost:18765", want: "127.0.0.1:18765"},
		{name: "ipv6", input: "[::1]:18765", want: "[::1]:18765"},
		{name: "all interfaces rejected", input: "0.0.0.0:18765", wantErr: true},
		{name: "lan rejected", input: "192.168.1.10:18765", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeLoopbackAddress(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeLoopbackAddress(%q) unexpectedly succeeded: %q", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalizeLoopbackAddress(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestIndexCreatesLoopbackSession(t *testing.T) {
	s := &server{listenAddr: "127.0.0.1:18765", secret: strings.Repeat("a", 64)}

	responseRecorder := httptest.NewRecorder()
	s.handleIndex(responseRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}
	response := responseRecorder.Result()
	if len(response.Cookies()) != 1 || !response.Cookies()[0].HttpOnly || response.Cookies()[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie is not hardened: %#v", response.Cookies())
	}
}

func TestMutationRequiresSameOriginAndHeader(t *testing.T) {
	s := &server{listenAddr: "127.0.0.1:18765", secret: strings.Repeat("b", 64)}
	called := false
	handler := s.requireMutation(func(http.ResponseWriter, *http.Request) { called = true })

	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18765/api/test", strings.NewReader("{}"))
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.secret})
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusMethodNotAllowed || called {
		t.Fatalf("mutation without admin header was accepted: status=%d called=%v", response.Code, called)
	}

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18765/api/test", strings.NewReader("{}"))
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.secret})
	request.Header.Set("X-AgentDock-Admin", "1")
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("cross-origin mutation was accepted: status=%d called=%v", response.Code, called)
	}

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18765/api/test", strings.NewReader("{}"))
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.secret})
	request.Header.Set("X-AgentDock-Admin", "1")
	request.Header.Set("Origin", "http://127.0.0.1:18765")
	response = httptest.NewRecorder()
	handler(response, request)
	if !called {
		t.Fatal("same-origin authenticated mutation was rejected")
	}
}
