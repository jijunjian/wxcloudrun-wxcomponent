package httputils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/WeixinCloud/wxcloudrun-wxcomponent/comm/log"
)

const diagnosticBodyLimit = 4096

var credentialPattern = regexp.MustCompile(`(?i)(["']?[a-z0-9_]*(?:token|secret|password|ticket|authorization|cookie|pre_auth_code)[a-z0-9_]*["']?\s*[:=]\s*)("[^"\r\n]*"|'[^'\r\n]*'|[^\s&,;}<]+)`)

// SafeDiagnostic removes credential fields from errors before logging them.
func SafeDiagnostic(value string) string {
	return credentialPattern.ReplaceAllString(value, `${1}"[REDACTED]"`)
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func safeRequestDiagnostic(req *http.Request, value string) string {
	// An upstream gateway may echo query credentials under an unrelated field.
	for key, values := range req.URL.Query() {
		if credentialPattern.MatchString(key + "=value") {
			for _, secret := range values {
				if secret != "" {
					value = strings.ReplaceAll(value, secret, "[REDACTED]")
				}
			}
		}
	}
	return SafeDiagnostic(value)
}

func logRequestError(req *http.Request, started time.Time, err error) error {
	result := fmt.Errorf("upstream request failed method=%s url=%s duration_ms=%d error=%q", req.Method, safeURL(req.URL.String()), time.Since(started).Milliseconds(), safeRequestDiagnostic(req, err.Error()))
	log.Error(result)
	return err
}

func upstreamStatusError(req *http.Request, resp *http.Response, started time.Time) error {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, diagnosticBodyLimit+1))
	truncated := len(body) > diagnosticBodyLimit
	if truncated {
		body = body[:diagnosticBodyLimit]
	}
	requestID := resp.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = resp.Header.Get("X-Tcloud-Request-Id")
	}
	result := fmt.Errorf("upstream request failed method=%s url=%s status=%d duration_ms=%d content_type=%q server=%q request_id=%q body=%q truncated=%t read_error=%t", req.Method, safeURL(req.URL.String()), resp.StatusCode, time.Since(started).Milliseconds(), safeRequestDiagnostic(req, resp.Header.Get("Content-Type")), safeRequestDiagnostic(req, resp.Header.Get("Server")), safeRequestDiagnostic(req, requestID), safeRequestDiagnostic(req, string(body)), truncated, readErr != nil)
	log.Error(result)
	return fmt.Errorf("http code: %d", resp.StatusCode)
}
