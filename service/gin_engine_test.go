package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func TestGinEngineClientIPDefaultsAndOverride(t *testing.T) {
	for _, trustProxy := range []bool{false, true} {
		name, want := "default ignores forged headers", "10.20.0.5"
		if trustProxy {
			name, want = "router may explicitly trust proxy", "198.51.100.8"
		}
		t.Run(name, func(t *testing.T) {
			r := newGinEngine()
			if trustProxy {
				if err := r.SetTrustedProxies([]string{"10.20.0.5"}); err != nil {
					t.Fatal(err)
				}
			}
			r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.20.0.5:4321"
			req.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.8")
			req.Header.Set("X-Real-IP", "203.0.113.9")
			req.Header.Set("X-Appengine-Remote-Addr", "203.0.113.9")
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if got := res.Body.String(); got != want {
				t.Fatalf("ClientIP() = %q, want %q", got, want)
			}
		})
	}
}

func TestConfigureGinClientIPFollowsForwardedChain(t *testing.T) {
	r := newGinEngine()
	if err := configureGinClientIP(r, []string{"10.20.0.0/24"}, nil); err != nil {
		t.Fatal(err)
	}
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.20.0.5:4321"                           // ingress -> Go
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.20.0.6") // client -> ingress
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if got := res.Body.String(); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q, want original client IP", got)
	}
}

func TestConfigureGinClientIPAcceptsCustomHeader(t *testing.T) {
	r := newGinEngine()
	if err := configureGinClientIP(
		r,
		[]string{"10.20.0.0/24"},
		[]string{"X-CDN-Client-IP"},
	); err != nil {
		t.Fatal(err)
	}
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.20.0.5:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-CDN-Client-IP", "198.51.100.8")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if got := res.Body.String(); got != "198.51.100.8" {
		t.Fatalf("ClientIP() = %q, want configured gateway header value", got)
	}
}

func TestConfigureGinClientIPRejectsInvalidValues(t *testing.T) {
	r := newGinEngine()
	if err := configureGinClientIP(r, []string{"not-a-cidr"}, nil); err == nil {
		t.Fatal("invalid trusted proxy configuration was accepted")
	}
	if err := configureGinClientIP(r, nil, []string{" "}); err == nil {
		t.Fatal("empty remote IP header name was accepted")
	}
}

func TestServerClientIPConfigParsing(t *testing.T) {
	const data = `[server]
trusted-proxies = ["10.20.0.0/24", "fd00:10:20::/64"]
remote-ip-headers = ["X-CDN-Client-IP"]
`
	path := filepath.Join(t.TempDir(), "client-ip.toml")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	loader := koanf.New(".")
	if err := loader.Load(file.Provider(path), toml.Parser()); err != nil {
		t.Fatal(err)
	}
	var parsed Config
	if err := loader.UnmarshalWithConf("", &parsed, koanf.UnmarshalConf{Tag: "toml"}); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Server.TrustedProxies) != 2 || parsed.Server.TrustedProxies[0] != "10.20.0.0/24" {
		t.Fatalf("trusted-proxies parsed as %#v", parsed.Server.TrustedProxies)
	}
	if len(parsed.Server.RemoteIPHeaders) != 1 || parsed.Server.RemoteIPHeaders[0] != "X-CDN-Client-IP" {
		t.Fatalf("remote-ip-headers parsed as %#v", parsed.Server.RemoteIPHeaders)
	}
}
