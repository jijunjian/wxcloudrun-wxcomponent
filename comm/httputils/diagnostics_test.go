package httputils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Capture the real stdout logger in a subprocess without changing production logging.
func TestUpstreamFailureDiagnostics(t *testing.T) {
	if os.Getenv("WX_DIAGNOSTICS_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestUpstreamFailureDiagnostics$")
		cmd.Env = append(os.Environ(), "WX_DIAGNOSTICS_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("request checks failed: %v\n%s", err, output)
		}
		logs := string(output)
		for _, want := range []string{"ERROR", "status=502", "gateway unavailable", "trace-test-123", "duration_ms=", "truncated=true"} {
			if !strings.Contains(logs, want) {
				t.Errorf("missing diagnostic %q in %s", want, logs)
			}
		}
		for _, secret := range []string{"query-private-value", "body-private-value", "response-private-value", "success-private-value", "large-response-tail"} {
			if strings.Contains(logs, secret) {
				t.Errorf("credential exposed in logs: %s", secret)
			}
		}
		return
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cloudbase_access_token") != "query-private-value" {
			t.Error("request token changed")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "trace-test-123")
		if r.URL.Path == "/ok" {
			fmt.Fprint(w, `{"pre_auth_code":"success-private-value", "large-response-tail"}`)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		if r.URL.Path == "/large" {
			fmt.Fprint(w, strings.Repeat("x", 5000)+"large-response-tail")
			return
		}
		fmt.Fprint(w, `{"errmsg":"gateway unavailable","access_token":"response-private-value","echo":"query-private-value"}`)
	}))
	defer server.Close()
	for _, method := range []string{"GET", "POST"} {
		url := server.URL + "/fail?cloudbase_access_token=query-private-value"
		var resp *http.Response
		var err error
		if method == "GET" {
			resp, _, err = RawGet(url)
		} else {
			resp, _, err = RawPost(url, []byte(`{"secret":"body-private-value"}`), "application/json")
		}
		if err == nil || resp == nil || resp.StatusCode != 502 {
			t.Fatalf("%s: expected upstream failure, got %v", method, err)
		}
	}
	_, _, largeErr := RawGet(server.URL + "/large?cloudbase_access_token=query-private-value")
	if largeErr == nil {
		t.Fatal("expected large gateway response to fail")
	}
	_, body, err := RawGet(server.URL + "/ok?cloudbase_access_token=query-private-value")
	if err != nil || string(body) != `{"pre_auth_code":"success-private-value", "large-response-tail"}` {
		t.Fatalf("success response changed: %s %v", body, err)
	}
}
