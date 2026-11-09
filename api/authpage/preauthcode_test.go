package authpage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/WeixinCloud/wxcloudrun-wxcomponent/comm/config"
	"github.com/gin-gonic/gin"
)

// Exercise the actual route over HTTP with a controlled upstream gateway.
// No database or real merchant authorization is touched.
func TestPreAuthCodeGatewayFailure(t *testing.T) {
	if os.Getenv("WX_PREAUTH_DIAGNOSTICS_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPreAuthCodeGatewayFailure$")
		cmd.Env = append(os.Environ(), "WX_PREAUTH_DIAGNOSTICS_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("route check failed: %v\n%s", err, output)
		}
		for _, want := range []string{"create preauthcode failed", "status=502", "auth_mode=cloudbase", "preauth-gateway-test"} {
			if !strings.Contains(string(output), want) {
				t.Errorf("missing %q in route logs: %s", want, output)
			}
		}
		return
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/cgi-bin/component/api_create_preauthcode" {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("X-Request-Id", "preauth-gateway-test")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "gateway unavailable")
	}))
	defer upstream.Close()
	proxyURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	previousTransport := http.DefaultTransport
	previousConfig := *config.WxApiConf
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	http.DefaultTransport = transport
	*config.WxApiConf = config.WxApi{UseCloudBaseAccessToken: true}
	defer func() {
		http.DefaultTransport = previousTransport
		*config.WxApiConf = previousConfig
		transport.CloseIdleConnections()
	}()
	router := gin.New()
	Routers(router.Group("/wxcomponent"))
	service := httptest.NewServer(router)
	defer service.Close()
	clientTransport := &http.Transport{}
	defer clientTransport.CloseIdleConnections()
	client := &http.Client{Transport: clientTransport}
	resp, err := client.Get(service.URL + "/wxcomponent/authpage/preauthcode")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || result.Code != 1002 || result.Data != "http code: 502" {
		t.Fatalf("public response changed: HTTP %d %+v", resp.StatusCode, result)
	}
}
