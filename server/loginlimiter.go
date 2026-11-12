package server

import (
	"sync"
	"time"
)

// Login failure limitr: press IP Count one slide window each of the two dimensions of the account, and the ultra-threshold value of the failure number within the window is a locked period
// The cooling period. Process memory (single case deployment sufficient, zero acceptable restart), no DB Write——So that every failure doesn't go down.
// Inversely magnifying the face of the attack. Lock/Key events, such as unlocking, are written by the caller and can be viewed on the log page.
//
// The meaning of two dimensions: only by account lock → The attackers can use a wrong password to use an account number to lock legitimate users to death.DoS);Overlay Press IP
// Lock → A single source sweeps multiple user names. IP Block. Two barrels of either shot were refused.

type loginBucket struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*loginBucket
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{buckets: map[string]*loginBucket{}}
}

// loginPolicy It's a truncable parameter for the capper, from settings Read (safe default if not set).
type loginPolicy struct {
	enabled       bool
	userThreshold int
	ipThreshold   int
	window        time.Duration
	cooldown      time.Duration
}

// loginPolicy Assembly current policy: all available settings Overwrite, invalid value retreat default.
func (s *Server) loginPolicy() loginPolicy {
	pol := loginPolicy{
		enabled:       true,
		userThreshold: 10,
		ipThreshold:   30,
		window:        15 * time.Minute,
		cooldown:      15 * time.Minute,
	}
	if s.m == nil || s.m.pg == nil {
		return pol
	}
	pg := s.m.pg
	pol.enabled = pg.GetBool("auth.lockout_enabled", true)
	if v := pg.GetInt("auth.lockout_user_threshold", 0); v > 0 {
		pol.userThreshold = v
	}
	if v := pg.GetInt("auth.lockout_ip_threshold", 0); v > 0 {
		pol.ipThreshold = v
	}
	if v := pg.GetInt("auth.lockout_window_minutes", 0); v > 0 {
		pol.window = time.Duration(v) * time.Minute
	}
	if v := pg.GetInt("auth.lockout_cooldown_minutes", 0); v > 0 {
		pol.cooldown = time.Duration(v) * time.Minute
	}
	return pol
}

func ipKey(ip string) string     { return "ip:" + ip }
func userKey(user string) string { return "u:" + user }

// locked Report on IP or whether the account number is locked in cooling and returns to the remaining cooling hours (the larger of the two barrels).
func (l *loginLimiter) locked(ip, user string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	var remain time.Duration
	for _, k := range []string{ipKey(ip), userKey(user)} {
		if b := l.buckets[k]; b != nil && now.Before(b.lockedUntil) {
			if r := time.Until(b.lockedUntil); r > remain {
				remain = r
			}
		}
	}
	return remain > 0, remain
}

// fail One failed. IP The barrel with the account. Back true Means this failure locks a barrel into a state.
// (For callers to write a lock audit). The policy is closed without any status.locked Forever for fake.
func (l *loginLimiter) fail(ip, user string, pol loginPolicy) bool {
	if !pol.enabled {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweepLocked(now, pol)
	newlyLocked := false
	if l.recordLocked(ipKey(ip), now, pol.window, pol.cooldown, pol.ipThreshold) {
		newlyLocked = true
	}
	if user != "" && l.recordLocked(userKey(user), now, pol.window, pol.cooldown, pol.userThreshold) {
		newlyLocked = true
	}
	return newlyLocked
}

// recordLocked A one-time recording failed on a barrel, and if necessary the window slipped, the threshold was set for the cooling due.
// Back true Means this is exactly the time to trigger the lock. Caller Holding l.mu.
func (l *loginLimiter) recordLocked(key string, now time.Time, window, cooldown time.Duration, threshold int) bool {
	b := l.buckets[key]
	if b == nil {
		b = &loginBucket{windowStart: now}
		l.buckets[key] = b
	}
	if now.Sub(b.windowStart) > window {
		b.windowStart = now
		b.count = 0
	}
	b.count++
	if b.count >= threshold {
		b.lockedUntil = now.Add(cooldown)
		b.count = 0
		b.windowStart = now
		return true
	}
	return false
}

// reset Clear it. IP Counts the failure of the account (call when login is successful).
func (l *loginLimiter) reset(ip, user string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, ipKey(ip))
	delete(l.buckets, userKey(user))
}

// sweepLocked Clear out old drums that are not locked and that have expired in windows when the volume of barrels is too large to be different.
// Username/Source map Hold on to memory pressure. Caller Holding l.mu.
func (l *loginLimiter) sweepLocked(now time.Time, pol loginPolicy) {
	if len(l.buckets) < 4096 {
		return
	}
	for k, b := range l.buckets {
		if now.After(b.lockedUntil) && now.Sub(b.windowStart) > pol.window {
			delete(l.buckets, k)
		}
	}
}
