package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

type Config struct {
	OIDCIssuerURL        string
	ExpectedAudience     string
	RequiredTrustDomain  string
	RefreshInterval      time.Duration
	ListenAddr           string
	HTTPTimeout          time.Duration
}

type OIDCDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type verifier struct {
	mu      sync.RWMutex
	jwks    jwk.Set
	jwksURI string
	issuer  string
	http    *http.Client
}

type VerifyRequest struct {
	Token string `json:"token"`
}

type VerifyResponse struct {
	OK       bool                   `json:"ok"`
	SPIFFEID string                 `json:"spiffe_id,omitempty"`
	Claims   map[string]interface{} `json:"claims,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

func main() {
	cfg := loadConfig()

	ext, err := newExtClient()
	if err != nil {
		// Extension can be run locally for dev; in Lambda this must succeed.
		log.Printf("WARN: Extensions API not available: %v", err)
	}
	if ext != nil {
		if err := ext.register(); err != nil {
			log.Fatalf("extension register failed: %v", err)
		}
		log.Printf("registered extension")
	}

	v := &verifier{
		http: &http.Client{Timeout: cfg.HTTPTimeout},
	}
	if err := v.bootstrap(cfg); err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	go v.refreshLoop(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(w, r, v, cfg)
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	log.Printf("SPIFFE verifier listening on %s", cfg.ListenAddr)

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// In Lambda, block on the Extensions API event stream.
	// On SHUTDOWN, exit gracefully.
	if ext != nil {
		for {
			ev, err := ext.nextEvent(context.Background())
			if err != nil {
				log.Printf("extension event stream error: %v", err)
				time.Sleep(200 * time.Millisecond)
				continue
			}
			// We don't strictly need to parse events for this PoC; SHUTDOWN ends the process anyway.
			_ = ev
		}
	} else {
		// Local dev mode: keep running.
		select {}
	}

	_ = srv.Shutdown(context.Background())
}

func loadConfig() Config {
	return Config{
		OIDCIssuerURL:       mustEnv("OIDC_ISSUER_URL"),
		ExpectedAudience:    mustEnv("EXPECTED_AUDIENCE"),
		RequiredTrustDomain: mustEnv("REQUIRED_TRUST_DOMAIN"),
		RefreshInterval:     envDuration("JWKS_REFRESH", 120*time.Second),
		ListenAddr:          envStr("LISTEN_ADDR", "127.0.0.1:2773"),
		HTTPTimeout:         envDuration("HTTP_TIMEOUT", 2*time.Second),
	}
}

func (v *verifier) bootstrap(cfg Config) error {
	disc, err := v.fetchDiscovery(cfg.OIDCIssuerURL)
	if err != nil {
		return err
	}
	set, err := v.fetchJWKS(disc.JWKSURI)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.issuer = disc.Issuer
	v.jwksURI = disc.JWKSURI
	v.jwks = set
	v.mu.Unlock()

	log.Printf("bootstrap ok: issuer=%s jwks_uri=%s", disc.Issuer, disc.JWKSURI)
	return nil
}

func (v *verifier) refreshLoop(cfg Config) {
	t := time.NewTicker(cfg.RefreshInterval)
	defer t.Stop()
	for range t.C {
		v.mu.RLock()
		jwksURI := v.jwksURI
		v.mu.RUnlock()

		set, err := v.fetchJWKS(jwksURI)
		if err != nil {
			log.Printf("JWKS refresh failed: %v", err)
			continue
		}
		v.mu.Lock()
		v.jwks = set
		v.mu.Unlock()
		log.Printf("JWKS refreshed")
	}
}

func handleVerify(w http.ResponseWriter, r *http.Request, v *verifier, cfg Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Token) == "" {
		writeJSON(w, http.StatusBadRequest, VerifyResponse{OK: false, Error: "invalid request"})
		return
	}

	claims, spiffeID, err := v.verifyJWT(req.Token, cfg)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, VerifyResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, VerifyResponse{OK: true, SPIFFEID: spiffeID, Claims: claims})
}

func (v *verifier) verifyJWT(raw string, cfg Config) (map[string]interface{}, string, error) {
	v.mu.RLock()
	jwks := v.jwks
	issuer := v.issuer
	v.mu.RUnlock()

	keyfunc := func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("missing kid")
		}
		key, ok := jwks.LookupKeyID(kid)
		if !ok {
			return nil, fmt.Errorf("unknown kid")
		}
		var pub interface{}
		if err := key.Raw(&pub); err != nil {
			return nil, fmt.Errorf("bad jwk: %w", err)
		}
		return pub, nil
	}

	parsed, err := jwt.Parse(raw, keyfunc,
		jwt.WithIssuer(issuer),
		jwt.WithAudience(cfg.ExpectedAudience),
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
	)
	if err != nil || !parsed.Valid {
		return nil, "", fmt.Errorf("token invalid: %v", err)
	}

	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, "", fmt.Errorf("claims invalid")
	}

	sub, _ := mc["sub"].(string)
	if sub == "" {
		return nil, "", fmt.Errorf("missing sub")
	}
	// Trust domain enforcement
	if !strings.HasPrefix(sub, cfg.RequiredTrustDomain+"/") {
		return nil, "", fmt.Errorf("sub not in trust domain")
	}

	// Convert to JSON-safe map
	out := make(map[string]interface{}, len(mc))
	for k, v := range mc {
		out[k] = v
	}
	return out, sub, nil
}

func (v *verifier) fetchDiscovery(issuerURL string) (*OIDCDiscovery, error) {
	u := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"
	b, err := v.httpGet(u)
	if err != nil {
		return nil, err
	}
	var disc OIDCDiscovery
	if err := json.Unmarshal(b, &disc); err != nil {
		return nil, err
	}
	if disc.Issuer == "" || disc.JWKSURI == "" {
		return nil, fmt.Errorf("discovery missing fields")
	}
	return &disc, nil
}

func (v *verifier) fetchJWKS(jwksURI string) (jwk.Set, error) {
	b, err := v.httpGet(jwksURI)
	if err != nil {
		return nil, err
	}
	return jwk.Parse(b)
}

func (v *verifier) httpGet(url string) ([]byte, error) {
	resp, err := v.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("GET %s: %s: %s", url, resp.Status, string(body))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}
func envStr(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
func envDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
