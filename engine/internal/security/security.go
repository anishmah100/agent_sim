// Package security — launch-blocking middleware bundle:
//
//   - CORS allowlist (replaces the v0 wildcard)
//   - JWT verification on /api/v1/agent/register (HS256, shared secret)
//   - Per-IP token bucket rate limiter on /api/v1/agent/register
//
// Configured via flags in main.go. Each layer is independent — set its
// secret to empty / list to nil to disable. NOT INCLUDED: speech
// profanity filtering — agents/users may produce edgy content
// intentionally; the world relays speech verbatim.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// === CORS ===

// CORS returns a middleware that emits Access-Control-Allow-Origin only
// for origins in the allowlist. Pre-flight (OPTIONS) is handled here.
// An empty allowlist disables CORS entirely (no header emitted).
func CORS(allowlist []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowlist))
	for _, o := range allowlist {
		allowed[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			origin := strings.ToLower(r.Header.Get("Origin"))
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					rw.Header().Set("Access-Control-Allow-Origin", origin)
					rw.Header().Set("Vary", "Origin")
					rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					rw.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				}
			}
			if r.Method == http.MethodOptions {
				rw.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
}

// === JWT (HS256) ===

// VerifyJWT parses + verifies an HS256 JWT with the given secret. Returns
// the payload map on success. ErrJWTInvalid covers signature failures,
// expiry, malformed shapes, etc.
var ErrJWTInvalid = errors.New("invalid jwt")

func VerifyJWT(token string, secret []byte) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrJWTInvalid
	}
	// Header — must declare alg=HS256
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrJWTInvalid
	}
	var hdr struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return nil, ErrJWTInvalid
	}
	if hdr.Alg != "HS256" {
		return nil, ErrJWTInvalid
	}
	// Signature — compute HMAC-SHA256 over "header.payload"
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, ErrJWTInvalid
	}
	// Payload — must parse + must not be expired
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrJWTInvalid
	}
	var payload map[string]any
	if err := json.Unmarshal(pb, &payload); err != nil {
		return nil, ErrJWTInvalid
	}
	if expF, ok := payload["exp"].(float64); ok {
		if time.Now().Unix() > int64(expF) {
			return nil, ErrJWTInvalid
		}
	}
	return payload, nil
}

// SignJWT writes a minimal HS256 JWT with the given payload and a TTL
// from now. A non-zero ttl (positive or negative) sets the exp claim;
// pass 0 to omit exp entirely. Use in tests / dev tools.
func SignJWT(payload map[string]any, secret []byte, ttl time.Duration) (string, error) {
	if ttl != 0 {
		payload = copyMap(payload)
		payload["exp"] = time.Now().Add(ttl).Unix()
	}
	hb, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(hb)
	p := base64.RawURLEncoding.EncodeToString(pb)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(h + "." + p))
	s := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return h + "." + p + "." + s, nil
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RequireJWT wraps a handler so it 401s when the Authorization header is
// missing or invalid. The token is read from either:
//   - "Authorization: Bearer <token>"
//   - or the `user_token` JSON body field (for the v0 register endpoint)
//
// On success, the validated claims are attached to the request context
// under the key `claimsCtxKey` for downstream handlers.
type claimsCtxKey struct{}

func RequireJWT(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			tok := bearerToken(r)
			if tok == "" {
				// Try the user_token body field — read + replace body.
				if r.Method == http.MethodPost {
					// We have to buffer the body to peek + replay.
					tok, r = peekBodyToken(r)
				}
			}
			if tok == "" {
				http.Error(rw, `{"error":"auth required"}`, http.StatusUnauthorized)
				return
			}
			_, err := VerifyJWT(tok, secret)
			if err != nil {
				http.Error(rw, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// peekBodyToken reads the JSON body, extracts user_token, then returns a
// new request with the body replaced so the downstream handler still sees it.
func peekBodyToken(r *http.Request) (string, *http.Request) {
	if r.Body == nil {
		return "", r
	}
	const limit = 1 << 16
	buf := make([]byte, limit)
	n, _ := readFull(r.Body, buf)
	body := buf[:n]
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		return "", replaceBody(r, body)
	}
	tok, _ := probe["user_token"].(string)
	return tok, replaceBody(r, body)
}

func readFull(rdr interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := rdr.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
	}
	return total, nil
}

func replaceBody(r *http.Request, body []byte) *http.Request {
	r2 := r.Clone(r.Context())
	r2.Body = &nopReadCloser{data: body}
	r2.ContentLength = int64(len(body))
	return r2
}

type nopReadCloser struct {
	data []byte
	pos  int
}

func (n *nopReadCloser) Read(p []byte) (int, error) {
	if n.pos >= len(n.data) {
		return 0, errors.New("EOF")
	}
	c := copy(p, n.data[n.pos:])
	n.pos += c
	return c, nil
}
func (n *nopReadCloser) Close() error { return nil }

// === Rate limit (per IP, token bucket) ===

// RateLimit returns a middleware that allows at most `burst` requests
// per IP within any 1-second window, replenishing at `rate` requests/sec.
// Excess returns 429. A zero rate disables the limiter.
func RateLimit(rate float64, burst int) func(http.Handler) http.Handler {
	if rate <= 0 || burst <= 0 {
		return func(h http.Handler) http.Handler { return h }
	}
	type bucket struct {
		tokens float64
		last   time.Time
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()
			mu.Lock()
			b := buckets[ip]
			if b == nil {
				b = &bucket{tokens: float64(burst), last: now}
				buckets[ip] = b
			}
			elapsed := now.Sub(b.last).Seconds()
			b.tokens += elapsed * rate
			if b.tokens > float64(burst) {
				b.tokens = float64(burst)
			}
			b.last = now
			allow := b.tokens >= 1
			if allow {
				b.tokens--
			}
			mu.Unlock()
			if !allow {
				rw.Header().Set("Retry-After", "1")
				http.Error(rw, `{"error":"rate limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
}

func clientIP(r *http.Request) string {
	// Respect X-Forwarded-For first (we'll sit behind Fly's proxy).
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i > 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

