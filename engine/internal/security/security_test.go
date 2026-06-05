package security

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJWTRoundTrip(t *testing.T) {
	secret := []byte("very-secret")
	tok, err := SignJWT(map[string]any{"sub": "rob"}, secret, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := VerifyJWT(tok, secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims["sub"] != "rob" {
		t.Fatalf("expected sub=rob, got %v", claims["sub"])
	}
}

func TestJWTRejectsBadSignature(t *testing.T) {
	tok, _ := SignJWT(map[string]any{"sub": "rob"}, []byte("a"), time.Hour)
	if _, err := VerifyJWT(tok, []byte("b")); err == nil {
		t.Fatal("expected signature mismatch to fail")
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	tok, _ := SignJWT(map[string]any{"sub": "rob"}, []byte("s"), -time.Hour)
	if _, err := VerifyJWT(tok, []byte("s")); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestRequireJWTBearer(t *testing.T) {
	secret := []byte("s")
	tok, _ := SignJWT(map[string]any{"sub": "rob"}, secret, time.Hour)
	h := RequireJWT(secret)(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	// Bearer ok
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK {
		t.Fatalf("bearer ok: got %d", rw.Code)
	}

	// Missing
	r2 := httptest.NewRequest("POST", "/", nil)
	rw2 := httptest.NewRecorder()
	h.ServeHTTP(rw2, r2)
	if rw2.Code != http.StatusUnauthorized {
		t.Fatalf("missing: got %d", rw2.Code)
	}
}

func TestRequireJWTBodyToken(t *testing.T) {
	secret := []byte("s")
	tok, _ := SignJWT(map[string]any{"sub": "rob"}, secret, time.Hour)
	h := RequireJWT(secret)(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// confirm body still readable downstream
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		if !strings.Contains(buf.String(), tok) {
			t.Errorf("body lost in middleware: %q", buf.String())
		}
		rw.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBufferString(`{"user_token":"` + tok + `","persona_blob":{}}`)
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK {
		t.Fatalf("body token: got %d", rw.Code)
	}
}

func TestRateLimitBurstAndRefill(t *testing.T) {
	h := RateLimit(2, 3)(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
	// 3 requests burst — all OK
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "1.2.3.4:5555"
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, r)
		if rw.Code != http.StatusOK {
			t.Fatalf("burst[%d]: got %d", i, rw.Code)
		}
	}
	// 4th immediately should 429
	r4 := httptest.NewRequest("POST", "/", nil)
	r4.RemoteAddr = "1.2.3.4:5555"
	rw4 := httptest.NewRecorder()
	h.ServeHTTP(rw4, r4)
	if rw4.Code != http.StatusTooManyRequests {
		t.Fatalf("4th: got %d", rw4.Code)
	}
	// different IP isolated
	r5 := httptest.NewRequest("POST", "/", nil)
	r5.RemoteAddr = "9.9.9.9:1234"
	rw5 := httptest.NewRecorder()
	h.ServeHTTP(rw5, r5)
	if rw5.Code != http.StatusOK {
		t.Fatalf("other ip: got %d", rw5.Code)
	}
}

func TestCORSAllowlist(t *testing.T) {
	c := CORS([]string{"https://app.example"})
	h := c(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
	// Allowed origin
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "https://app.example")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if got := rw.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allowed origin not echoed: %q", got)
	}
	// Disallowed
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Origin", "https://evil.example")
	rw2 := httptest.NewRecorder()
	h.ServeHTTP(rw2, r2)
	if got := rw2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin echoed: %q", got)
	}
	// Preflight
	r3 := httptest.NewRequest("OPTIONS", "/", nil)
	r3.Header.Set("Origin", "https://app.example")
	rw3 := httptest.NewRecorder()
	h.ServeHTTP(rw3, r3)
	if rw3.Code != http.StatusNoContent {
		t.Fatalf("preflight: got %d", rw3.Code)
	}
}
