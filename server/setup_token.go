package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"strings"
)

// First-run setup token, ported from ARTEX (#137).
//
// Until an admin password exists, POST /api/auth/init sets it. Without a token,
// whoever reaches the port first claims the instance, and the compose file
// publishes the port on every interface. The token is generated at startup,
// printed only to the server log, and required by /api/auth/init, so only the
// operator who can read the log can complete setup:
//
//	docker compose logs antifox | grep "Setup token"

// initSetupToken generates the token when the platform has no admin yet. A read
// failure leaves no token, which makes init fail closed rather than open.
func (s *Server) initSetupToken() {
	if s.m == nil || s.m.pg == nil {
		return
	}
	needs, err := s.m.pg.RBACNeedsBootstrap()
	if err != nil || !needs {
		return
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("[auth] generate setup token: %v", err)
	}
	s.setupMu.Lock()
	s.setupToken = hex.EncodeToString(buf)
	s.setupMu.Unlock()
	log.Printf("[auth] no admin password is set. Setup token for the /setup page: %s", s.setupToken)
}

// checkSetupToken compares got with the active token in constant time. It fails
// when no token is active.
func (s *Server) checkSetupToken(got string) bool {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	got = strings.TrimSpace(got)
	return s.setupToken != "" && subtle.ConstantTimeCompare([]byte(got), []byte(s.setupToken)) == 1
}

// clearSetupToken retires the token once setup has succeeded.
func (s *Server) clearSetupToken() {
	s.setupMu.Lock()
	s.setupToken = ""
	s.setupMu.Unlock()
}
