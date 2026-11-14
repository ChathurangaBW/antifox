// Command antifox runs the AntiFox backend: the dual SQLite graph stores,
// the event-driven exploration engine, and the JSON HTTP API consumed by the
// shadcn/ui frontend.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"antifox/config"
	"antifox/selfupdate"
	"antifox/server"
)

// version is the build version, injected at release time via
// -ldflags "-X main.version=<tag>". Defaults to "dev" for local builds.
var version = "dev"

const banner = `
   _   _  _ _____ ___ ___ _____  __
  /_\ | \| |_   _|_ _| __/ _ \ \/ /
 / _ \| .  | | |  | || _| (_) >  <
/_/ \_\_|\_| |_| |___|_| \___/_/\_\
`

// printBanner writes the startup banner + version/runtime info to stdout.
func printBanner(addr string) {
	fmt.Print(banner)
	fmt.Println("  AI-Driven Autonomous Penetration Testing Platform")
	fmt.Printf("  Version %s  ·  %s/%s  ·  %s  ·  Listening on %s\n\n",
		version, runtime.GOOS, runtime.GOARCH, runtime.Version(), addr)
}

// main is a thin wrapper so run()'s defers execute before os.Exit — needed because the
// one-click update protocol communicates "relaunch me" to the supervisor via an exit code.
func main() { os.Exit(run()) }

func run() int {
	// First of all: on Linux, re-exec without the secrets in the start environment,
	// so /proc/<pid>/environ never shows them (see secrets_linux.go).
	handoffSecrets()
	// One-click update bootstrap: must run before opening any port or DB. On a staged
	// upgrade it swaps the binary and returns Restart (we exit 75; the supervisor relaunches
	// us). Smoke-test child processes short-circuit via ANTIFOX_SELFUPDATE_SMOKE.
	action, upState := selfupdate.Bootstrap()
	server.SetBootUpdateState(upState)
	if action == selfupdate.Restart {
		return selfupdate.ExitRestart
	}
	server.BuildVersion = version // hand the build version to the server (health + update check)

	var (
		addr    = flag.String("addr", ":8787", "HTTP listen address")
		dataDir = flag.String("data", filepath.Join(config.BaseDir(), "data"), "data directory for SQLite stores (default: data/ next to the executable)")
		proxy   = flag.String("proxy", ":8788", "traffic recording proxy address (empty to disable)")
	)
	flag.Parse()

	printBanner(*addr)

	// capture backend logs into the in-memory sink (still to stderr) so the /logs
	// page can show a live log stream. Do this first, to catch startup logs too.
	server.StartLogCapture()

	// surface which config file the binary reads (absolute, so `go run`'s relative
	// "config.json" — resolved against the CWD — is unambiguous).
	cfgPath := config.Path()
	if abs, e := filepath.Abs(cfgPath); e == nil {
		cfgPath = abs
	}
	if _, e := os.Stat(cfgPath); e == nil {
		log.Printf("[config] configuration file: %s", cfgPath)
	} else {
		log.Printf("[config] configuration file: %s (not found — only ANTIFOX_PG_DSN will be tried)", cfgPath)
	}

	// Persistence guard: if dataDir is under /tmp or the system temporary directory,
	// traffic, reports, and transcripts disappear on restart. The database is stored
	// separately, but file data must live on a persistent volume.
	if abs, _ := filepath.Abs(*dataDir); abs != "" {
		if abs == "/tmp" || strings.HasPrefix(abs, "/tmp/") || strings.HasPrefix(abs, os.TempDir()) {
			log.Printf("⚠️  [data] dataDir=%s is inside a temporary directory; restarting will delete traffic, reports, and transcripts. Use -data with a persistent directory such as ~/antifox-data or /opt/antifox/data.", abs)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr, err := server.NewManager(*dataDir, *proxy)
	if err != nil {
		log.Printf("open stores: %v", err)
		return 1
	}
	defer mgr.Close()

	// Default skillDir to <dataDir>/skills so bundled skills materialize into the
	// writable, persistent data volume (portable across cloud / on-prem deploys).
	skillDir := config.SkillDir(filepath.Join(*dataDir, "skills"))
	if abs, err := filepath.Abs(skillDir); err == nil {
		skillDir = abs
	}
	log.Printf("[config] skill directory: %s", skillDir)
	// WithSecretScrub: the binary drops the database DSN and model keys from its
	// environment once read, so agent shells and other child processes never inherit them.
	srv := server.New(ctx, mgr, skillDir, *dataDir, server.WithSecretScrub())
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("AntiFox %s backend listening on %s (data=%s, workers=%d)", version, *addr, *dataDir, mgr.Workers())
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	// Confirm a just-applied update once we've stayed up long enough (clears the upgrade
	// marker so the next boot doesn't count a failed attempt / roll back). No-op otherwise.
	settleTimer := time.AfterFunc(selfupdate.SettleDelay, selfupdate.Settle)
	defer settleTimer.Stop()

	code := 0
	select {
	case <-ctx.Done():
		log.Println("shutting down...")
	case <-server.RestartRequested():
		log.Println("restarting to apply update (exit 75)...")
		code = selfupdate.ExitRestart
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return code
}
