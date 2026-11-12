package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"antifox/db"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtKeyFilename = "jwt.key"
	authPassKey    = "auth.password_hash" // Old single user password (used only for initial migration to rbac_users)
	// defaultSessionTTL SessionJWT)Default for length——At the same time,"Timeout"Window:
	// Backend slides in active use (see requireAuth),More than that. token Expiry requires re-entry.
	// Available Settings auth.session_ttl_minutes Overwrite (minutes).
	defaultSessionTTL = 60 * time.Minute
	sessionTTLKey     = "auth.session_ttl_minutes"
	keyChars          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type ctxKey int

const accessCtxKey ctxKey = 1

// authClaims holds the subject id + username + the password version at issue time (token_version).
// Ver Let's change the secrets./"Deactivated or kicked down" is created: token_version That's right, the difference is no.
type authClaims struct {
	Username string `json:"usr"`
	Ver      int    `json:"ver"`
	jwt.RegisteredClaims
}

// CurrentAccess From the request context to the current login user RBAC Authorise portraitsrequireAuth Injection.
func CurrentAccess(r *http.Request) *db.RBACAccess {
	acc, _ := r.Context().Value(accessCtxKey).(*db.RBACAccess)
	return acc
}

// loadOrCreateJWTKey reads / generates the 32-byte HS256 signing key.
func loadOrCreateJWTKey(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, jwtKeyFilename)
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) >= 32 {
		return []byte(strings.TrimSpace(string(data))), nil
	}
	buf := make([]byte, 32)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(keyChars))))
		if err != nil {
			return nil, fmt.Errorf("generate jwt key: %w", err)
		}
		buf[i] = keyChars[n.Int64()]
	}
	if err := os.WriteFile(path, buf, 0600); err != nil {
		return nil, fmt.Errorf("write jwt key: %w", err)
	}
	log.Printf("[auth] New JWT key Written %s", path)
	return buf, nil
}

// sessionTTL Read session length (minutes) settings, not set/It's illegal to retreat.
func (s *Server) sessionTTL() time.Duration {
	if s.m != nil && s.m.pg != nil {
		if m := s.m.pg.GetInt(sessionTTLKey, 0); m > 0 {
			return time.Duration(m) * time.Minute
		}
	}
	return defaultSessionTTL
}

// signJWT issues an HS256 token with the default session TTL (tests use this).
func signJWT(key []byte, userID, username string) (string, error) {
	return signJWTTTL(key, userID, username, 0, defaultSessionTTL)
}

// signJWTTTL issues an HS256 token that expires after ttl, stamped with the user's
// current token_version (ver) so a later password change / disable invalidates it.
func signJWTTTL(key []byte, userID, username string, ver int, ttl time.Duration) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, authClaims{
		Username: username,
		Ver:      ver,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}).SignedString(key)
}

// parseJWT validates and returns the claims.
func parseJWT(tokenStr string, key []byte) (*authClaims, bool) {
	var c authClaims
	t, err := jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return key, nil
	})
	if err != nil || !t.Valid {
		return nil, false
	}
	return &c, true
}

// extractToken reads the JWT from Authorization: Bearer, antifox_token cookie, or ?token=.
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("antifox_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

// requireAuth Verify JWT + Parsing User/Permission Injection context. /api/auth/* and /api/health Exemption.
func (s *Server) requireAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/auth/") || p == "/api/health" {
			h.ServeHTTP(w, r)
			return
		}
		claims, ok := parseJWT(extractToken(r), s.jwtKey)
		if !ok {
			writeErr(w, 401, "Not authorized or token Expired")
			return
		}
		// Parsing authorized images RBAC Forced and managed) , account disabled / token The version is rejected if it expires.
		if s.m != nil && s.m.pg != nil && claims.Subject != "" {
			acc, err := s.m.pg.ResolveRBACAccess(claims.Subject)
			if err == nil && acc != nil {
				if !acc.User.Enabled {
					writeErr(w, 403, "Account disabled")
					return
				}
				// token Version validation: Decrypt / After Disable token_version Self-additional, old token Carrying. Ver
				// If you're behind, you're out.
				if claims.Ver != acc.User.TokenVersion {
					writeErr(w, 401, "Login is invalid, please re-entry")
					return
				}
				// Slide duration (%2)= Free timeout:token After half a mile and still active, you'll be issued a new one. token Yes.
				// X-Refresh-Token Responds to a headback and replaces storage at the front end; when free time exceeds the duration of the session, the log-over is required.
				if claims.ExpiresAt != nil {
					ttl := s.sessionTTL()
					if rem := time.Until(claims.ExpiresAt.Time); rem > 0 && rem < ttl/2 {
						if fresh, err := signJWTTTL(s.jwtKey, claims.Subject, claims.Username, acc.User.TokenVersion, ttl); err == nil {
							w.Header().Set("Access-Control-Expose-Headers", "X-Refresh-Token")
							w.Header().Set("X-Refresh-Token", fresh)
						}
					}
				}
				r = r.WithContext(context.WithValue(r.Context(), accessCtxKey, acc))
				// Permission to enforce interception. Safety valves: unmagnified routers released;admin(Full powers)Never stop; switch rbac.enforce Default true.
				if s.m.pg.GetBool("rbac.enforce", true) {
					if perm := permissionForRequest(r.Method, r.URL.Path); perm != "" && !acc.Permissions[perm] {
						writeErr(w, 403, "Insufficient permission:"+perm)
						return
					}
				}
			}
		}
		h.ServeHTTP(w, r)
	})
}

// bootstrapRBAC Initialize old single user passwords to internalize admin,And sowing characters./Permissions (things, etc.).
func (s *Server) bootstrapRBAC() {
	if s.m == nil || s.m.pg == nil {
		return
	}
	legacyHash, _, _ := s.m.pg.GetSetting(authPassKey)
	if err := s.m.pg.BootstrapRBAC(legacyHash); err != nil {
		log.Printf("[rbac] bootstrap Failed: %v", err)
		return
	}
	if strings.TrimSpace(legacyHash) != "" {
		log.Printf("[rbac] Multiple user permissions ready (old password moved to inline) admin)")
	} else {
		log.Printf("[rbac] Multi-user permissions ready")
	}
}

// GET /api/auth/status — Has it been initialized (with platform users).
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	needs, err := pg.RBACNeedsBootstrap()
	if err != nil {
		writeErr(w, 500, "Query failed")
		return
	}
	writeJSON(w, 200, map[string]any{"initialized": !needs})
}

// POST /api/auth/init — Initial launch administrator password (build-in) admin).
func (s *Server) authInit(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	ip := clientIP(r)
	// fail-closed:Refuse all initialized guard queries when they are wrong and never let them go as "not initialized"
	// Password (otherwise, the query failed to be used to reset the administrator password).
	needs, err := pg.RBACNeedsBootstrap()
	if err != nil {
		pg.RecordAudit("", "auth", "init", "fail", ip, "Initialization of guard queries failed (rejected)", nil)
		writeErr(w, 503, "Services are temporarily unavailable")
		return
	}
	if !needs {
		pg.RecordAudit("", "auth", "init", "fail", ip, "Platform has been initialized and reset passwords are refused", nil)
		writeErr(w, 403, "Initialized")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Request format error")
		return
	}
	if msg := validatePasswordStrength(req.Password); msg != "" {
		writeErr(w, 400, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "Password encryption failed")
		return
	}
	if err := pg.BootstrapRBAC(string(hash)); err != nil {
		writeErr(w, 500, "Initialization failed: "+err.Error())
		return
	}
	_ = pg.SetSetting(authPassKey, string(hash)) // Compatibility reservations
	ver := 0
	if u, e := pg.GetRBACUserByID("admin"); e == nil && u != nil {
		ver = u.TokenVersion
	}
	tok, err := signJWTTTL(s.jwtKey, "admin", "admin", ver, s.sessionTTL())
	if err != nil {
		writeErr(w, 500, "token Failed to generate")
		return
	}
	pg.RecordAudit("admin", "auth", "init", "success", ip, "Initialisation administrator password", nil)
	writeJSON(w, 200, map[string]any{"token": tok})
}

// POST /api/auth/change-password — Change the password of the current login user.
func (s *Server) authChangePassword(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	claims, ok := parseJWT(extractToken(r), s.jwtKey)
	if !ok {
		writeErr(w, 401, "Unauthorized")
		return
	}
	u, err := pg.GetRBACUserByID(claims.Subject)
	if err != nil {
		writeErr(w, 401, "User does not exist")
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Request format error")
		return
	}
	if msg := validatePasswordStrength(req.NewPassword); msg != "" {
		writeErr(w, 400, msg)
		return
	}
	ip := clientIP(r)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		pg.RecordAudit(u.Username, "auth", "change_password", "fail", ip, "Current password error", nil)
		writeErr(w, 401, "Current password error")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "Password encryption failed")
		return
	}
	// UpdateRBACUserPassword It'll grow. token_version,Cancel this request token All old token.
	if err := pg.UpdateRBACUserPassword(u.ID, string(newHash)); err != nil {
		writeErr(w, 500, "Save Failed: "+err.Error())
		return
	}
	pg.RecordAudit(u.Username, "auth", "change_password", "success", ip, "Change password successfully", nil)
	// Send a new version to the caller token,Avoid being kicked just after the password has been changed; other equipment is old token Still not valid.
	fresh := ""
	if nu, e := pg.GetRBACUserByID(u.ID); e == nil && nu != nil {
		fresh, _ = signJWTTTL(s.jwtKey, nu.ID, nu.Username, nu.TokenVersion, s.sessionTTL())
	}
	writeJSON(w, 200, map[string]any{"ok": true, "token": fresh})
}

// POST /api/auth/login — Verify username/Password returns JWT.
func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Request format error")
		return
	}
	ip := clientIP(r)
	uname := strings.TrimSpace(req.Username)
	// Violent decomposition protection: press IP + Account double-dimensional check lock status. Direct hit. 429,Don't do anything anymore.
	// Password Matchbcrypt It's exhausting itself. CPU,"Turn in the front and slow down the amount of effort." CPU/Photo by Flickr user @FaceCho.org.
	if locked, retry := s.loginLimiter.locked(ip, uname); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		pg.RecordAudit(uname, "auth", "login_locked", "fail", ip,
			fmt.Sprintf("Trigger violence to break lock and cool the rest. %ds", int(retry.Seconds())), nil)
		writeErr(w, 429, fmt.Sprintf("Try too often, account number./Source temporarily locked, please. %d Try again in seconds.", int(retry.Seconds())))
		return
	}
	u, err := pg.GetRBACUserByUsername(req.Username)
	if err != nil || u == nil || !u.Enabled {
		reason := "Error with username or password"
		if u != nil && !u.Enabled {
			reason = "Account disabled"
		}
		s.registerLoginFailure(pg, ip, uname)
		pg.RecordAudit(uname, "auth", "login_failed", "fail", ip, reason, nil)
		log.Printf("[auth] Login failed=%q ip=%s Reason=%s", uname, ip, reason)
		writeErr(w, 401, reason)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		s.registerLoginFailure(pg, ip, u.Username)
		pg.RecordAudit(u.Username, "auth", "login_failed", "fail", ip, "Password error", nil)
		log.Printf("[auth] Login failed=%q ip=%s Reason=Password error", u.Username, ip)
		writeErr(w, 401, "Error with username or password")
		return
	}
	tok, err := signJWTTTL(s.jwtKey, u.ID, u.Username, u.TokenVersion, s.sessionTTL())
	if err != nil {
		writeErr(w, 500, "token Failed to generate")
		return
	}
	s.loginLimiter.reset(ip, u.Username) // Successful login zero. IP+Account failure count
	pg.RecordAudit(u.Username, "auth", "login", "success", ip, "Login successful", nil)
	log.Printf("[auth] Login Successful User=%q ip=%s", u.Username, ip)
	writeJSON(w, 200, map[string]any{"token": tok})
}

// registerLoginFailure Write down a failure to the stopper; write an audit that is easily visible on the log page if the lock is triggered.
func (s *Server) registerLoginFailure(pg *db.DB, ip, username string) {
	if s.loginLimiter.fail(ip, username, s.loginPolicy()) {
		pg.RecordAudit(username, "auth", "login_locked", "fail", ip, "Number of failures above threshold, temporarily locked", nil)
		log.Printf("[auth] Violent crack locking user=%q ip=%s", username, ip)
	}
}

// validatePasswordStrength returns an empty string when the password passes, and the reason otherwise
// Rule: Length 8–128 bits, containing at least two characters (letters) / Numbers / ) and cannot be a single duplicate character.
func validatePasswordStrength(pw string) string {
	if len(pw) < 8 {
		return "Password at least 8 bit"
	}
	if len(pw) > 128 {
		return "Overlong password (maximum) 128 Bits)"
	}
	var hasLetter, hasDigit, hasOther bool
	allSame := true
	for i, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasOther = true
		}
		if i > 0 && rune(pw[i]) != rune(pw[0]) {
			allSame = false
		}
	}
	if allSame {
		return "The password cannot be a single duplicate. Arguments"
	}
	classes := 0
	for _, b := range []bool{hasLetter, hasDigit, hasOther} {
		if b {
			classes++
		}
	}
	if classes < 2 {
		return "Password must contain at least two characters (letters) / Numbers / Symbol)"
	}
	return ""
}

// clientIP Extract Client IP(Priority X-Forwarded-For / X-Real-IP,Back RemoteAddr).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}
