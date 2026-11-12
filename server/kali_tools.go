package server

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"antifox/db"
	actool "antifox/engine/tool"
)

// KaliToolSpec describes one entry in the built-in Kali & ProjectDiscovery CLI
// tool catalog. Each entry is seeded into the PostgreSQL `tools` table as a
// `kind="shell"` environment declaration (bound by default to worker, pentest,
// and auto), and carries dynamic self-installer / updater metadata in `exec` so
// tools are never frozen at an outdated embedded version.
type KaliToolSpec struct {
	Key            string   `json:"key"`
	Binary         string   `json:"binary"`
	Category       string   `json:"category"` // recon | web_discovery | vuln_scan | exploitation | auth_crypto
	Description    string   `json:"description"`
	InstallMethod  string   `json:"install_method"`            // github_release | git_script | pip | apt
	GitHubRepo     string   `json:"github_repo,omitempty"`     // owner/repo for github_release or git_script
	GitEntryScript string   `json:"git_entry,omitempty"`       // relative script path inside git repo (for git_script)
	GitInterpreter string   `json:"git_interpreter,omitempty"` // python3 | bash | ruby | perl
	AptPkg         string   `json:"apt_pkg,omitempty"`
	PipPkg         string   `json:"pip_pkg,omitempty"`
	PostInstallCmd string   `json:"post_install_cmd,omitempty"`
	VersionArgs    []string `json:"version_args,omitempty"`
}

// KaliToolStatus reports the live installation status of a catalog tool.
type KaliToolStatus struct {
	Key           string `json:"key"`
	Binary        string `json:"binary"`
	Category      string `json:"category"`
	Installed     bool   `json:"installed"`
	Path          string `json:"path,omitempty"`
	Version       string `json:"version,omitempty"`
	InstallMethod string `json:"install_method"`
	GitHubRepo    string `json:"github_repo,omitempty"`
	Message       string `json:"message,omitempty"`
}

const settingKaliCatalogVersion = "kali_tool_catalog_v1"

// KaliToolCatalog returns the curated Kali Linux & ProjectDiscovery CLI toolset.
// Descriptions explicitly include structured JSON/JSONL output flags that pair
// with `insert_assets(scan_file=...)` for deterministic graph & asset ingestion.
func KaliToolCatalog() []KaliToolSpec {
	return []KaliToolSpec{
		// ── Reconnaissance & Asset Discovery ──────────────────────────────────
		{
			Key:           "subfinder",
			Binary:        "subfinder",
			Category:      "recon",
			Description:   "Fast passive subdomain enumeration (ProjectDiscovery). Usage: `subfinder -d example.com -silent -json -o subfinder.jsonl`, then call `insert_assets(scan_file=\"subfinder.jsonl\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/subfinder",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:           "dnsx",
			Binary:        "dnsx",
			Category:      "recon",
			Description:   "Fast multi-purpose DNS toolkit and resolver (ProjectDiscovery). Usage: `dnsx -l subdomains.txt -a -cname -resp -silent -json -o dnsx.jsonl`, then call `insert_assets(scan_file=\"dnsx.jsonl\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/dnsx",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:           "naabu",
			Binary:        "naabu",
			Category:      "recon",
			Description:   "Fast SYN/CONNECT port scanner (ProjectDiscovery). Usage: `naabu -host example.com -p - -rate 1000 -silent -json -o naabu.jsonl`, then call `insert_assets(scan_file=\"naabu.jsonl\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/naabu",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:           "httpx",
			Binary:        "httpx",
			Category:      "recon",
			Description:   "HTTP probe & technology fingerprinter (ProjectDiscovery). Usage: `httpx -l targets.txt -status-code -title -tech-detect -follow-redirects -silent -json -o httpx.jsonl`, then call `insert_assets(scan_file=\"httpx.jsonl\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/httpx",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:           "nmap",
			Binary:        "nmap",
			Category:      "recon",
			Description:   "Network mapper for port scanning, service/version detection, and NSE scripts. Usage: `nmap -sV -sC -T4 -Pn -oN nmap.txt <target>`.",
			InstallMethod: "apt",
			AptPkg:        "nmap",
			VersionArgs:   []string{"--version"},
		},
		{
			Key:            "whatweb",
			Binary:         "whatweb",
			Category:       "recon",
			Description:    "Next-generation web scanner identifying CMS, frameworks, server headers, and versions. Usage: `whatweb -a 3 --color=never <url>`.",
			InstallMethod:  "apt",
			AptPkg:         "whatweb",
			GitHubRepo:     "urbanadventurer/WhatWeb",
			GitEntryScript: "whatweb",
			GitInterpreter: "ruby",
			VersionArgs:    []string{"--version"},
		},
		{
			Key:           "wafw00f",
			Binary:        "wafw00f",
			Category:      "recon",
			Description:   "Web Application Firewall (WAF) fingerprinting tool. Run before aggressive fuzzing to detect Cloudflare/ModSecurity/AWS WAF. Usage: `wafw00f <url>`.",
			InstallMethod: "pip",
			PipPkg:        "wafw00f",
			AptPkg:        "wafw00f",
			VersionArgs:   []string{"--version"},
		},

		// ── Web Crawling, Content & Parameter Discovery ───────────────────────
		{
			Key:           "katana",
			Binary:        "katana",
			Category:      "web_discovery",
			Description:   "Next-generation web crawler with JS parsing (ProjectDiscovery). Usage: `katana -u https://example.com -d 3 -jc -silent -jsonl -o katana.jsonl`, then call `insert_assets(scan_file=\"katana.jsonl\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/katana",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:           "ffuf",
			Binary:        "ffuf",
			Category:      "web_discovery",
			Description:   "Fast web fuzzer for directories, vhosts, and parameters. Usage: `ffuf -u https://example.com/FUZZ -w wordlist.txt -mc 200,204,301,302,307,401,403 -ac -of json -o ffuf.json`, then call `insert_assets(scan_file=\"ffuf.json\")`.",
			InstallMethod: "github_release",
			GitHubRepo:    "ffuf/ffuf",
			AptPkg:        "ffuf",
			VersionArgs:   []string{"-V"},
		},
		{
			Key:           "gobuster",
			Binary:        "gobuster",
			Category:      "web_discovery",
			Description:   "Directory/file, DNS, and vhost busting tool. Usage: `gobuster dir -u https://example.com -w wordlist.txt -q -no-error`.",
			InstallMethod: "github_release",
			GitHubRepo:    "OJ/gobuster",
			AptPkg:        "gobuster",
			VersionArgs:   []string{"version"},
		},
		{
			Key:           "feroxbuster",
			Binary:        "feroxbuster",
			Category:      "web_discovery",
			Description:   "Fast recursive content discovery tool in Rust. Usage: `feroxbuster -u https://example.com -w wordlist.txt --auto-bail --silent`.",
			InstallMethod: "github_release",
			GitHubRepo:    "epi052/feroxbuster",
			VersionArgs:   []string{"--version"},
		},
		{
			Key:           "arjun",
			Binary:        "arjun",
			Category:      "web_discovery",
			Description:   "HTTP parameter discovery suite for finding hidden GET/POST/JSON parameters. Usage: `arjun -u https://example.com/api/ endpoint -oJ arjun.json`.",
			InstallMethod: "pip",
			PipPkg:        "arjun",
			VersionArgs:   []string{"--help"},
		},

		// ── Vulnerability Scanning & Verification ─────────────────────────────
		{
			Key:            "nuclei",
			Binary:         "nuclei",
			Category:       "vuln_scan",
			Description:    "Template-based vulnerability scanner for CVEs, misconfigurations, and exposures (ProjectDiscovery). Usage: `nuclei -u https://example.com -severity critical,high,medium -silent -jsonl -o nuclei.jsonl`.",
			InstallMethod:  "github_release",
			GitHubRepo:     "projectdiscovery/nuclei",
			PostInstallCmd: "nuclei -ut -silent",
			VersionArgs:    []string{"-version"},
		},
		{
			Key:            "nikto",
			Binary:         "nikto",
			Category:       "vuln_scan",
			Description:    "Web server scanner for dangerous files, outdated server software, and misconfigurations. Usage: `nikto -h https://example.com -Tuning 123457890ab -maxtime 180s`.",
			InstallMethod:  "git_script",
			GitHubRepo:     "sullo/nikto",
			GitEntryScript: "program/nikto.pl",
			GitInterpreter: "perl",
			AptPkg:         "nikto",
			VersionArgs:    []string{"-Version"},
		},
		{
			Key:           "dalfox",
			Binary:        "dalfox",
			Category:      "vuln_scan",
			Description:   "Parameter analysis and XSS scanner in Go. Usage: `dalfox url \"https://example.com/search?q=test\" --silence --format json`.",
			InstallMethod: "github_release",
			GitHubRepo:    "hahwul/dalfox",
			VersionArgs:   []string{"version"},
		},
		{
			Key:            "testssl",
			Binary:         "testssl",
			Category:       "vuln_scan",
			Description:    "Comprehensive TLS/SSL cipher, protocol, and cryptographic vulnerability scanner (testssl.sh). Usage: `testssl --quiet --fast https://example.com`.",
			InstallMethod:  "git_script",
			GitHubRepo:     "drwetter/testssl.sh",
			GitEntryScript: "testssl.sh",
			GitInterpreter: "bash",
			VersionArgs:    []string{"--version"},
		},
		{
			Key:           "sslscan",
			Binary:        "sslscan",
			Category:      "vuln_scan",
			Description:   "Fast SSL/TLS cipher suite and certificate scanner. Usage: `sslscan --no-colour example.com:443`.",
			InstallMethod: "apt",
			AptPkg:        "sslscan",
			VersionArgs:   []string{"--version"},
		},

		// ── Exploitation, OOB & Auth/Crypto Verification ──────────────────────
		{
			Key:            "sqlmap",
			Binary:         "sqlmap",
			Category:       "exploitation",
			Description:    "Automatic SQL injection detection and exploitation tool. Always keeps latest upstream rules via git/pip. Usage: `sqlmap -u \"https://example.com/item?id=1\" --batch --random-agent --level=2 --risk=2`.",
			InstallMethod:  "git_script",
			GitHubRepo:     "sqlmapproject/sqlmap",
			GitEntryScript: "sqlmap.py",
			GitInterpreter: "python3",
			AptPkg:         "sqlmap",
			PipPkg:         "sqlmap",
			VersionArgs:    []string{"--version"},
		},
		{
			Key:           "interactsh_client",
			Binary:        "interactsh-client",
			Category:      "exploitation",
			Description:   "Out-of-Band (OOB) interaction client (DNS/HTTP/SMTP callbacks) for confirming blind SSRF, blind XXE, blind RCE, and Log4Shell (ProjectDiscovery). Usage: `interactsh-client -json -o oob.jsonl`.",
			InstallMethod: "github_release",
			GitHubRepo:    "projectdiscovery/interactsh",
			VersionArgs:   []string{"-version"},
		},
		{
			Key:            "jwt_tool",
			Binary:         "jwt_tool",
			Category:       "auth_crypto",
			Description:    "Toolkit for validating, forging, and attacking JSON Web Tokens (alg=none, RS256->HS256 key confusion, weak HMAC keys, kid injection). Usage: `jwt_tool <JWT> -M at`.",
			InstallMethod:  "git_script",
			GitHubRepo:     "ticarpi/jwt_tool",
			GitEntryScript: "jwt_tool.py",
			GitInterpreter: "python3",
			VersionArgs:    []string{"-h"},
		},
		{
			Key:           "hydra",
			Binary:        "hydra",
			Category:      "auth_crypto",
			Description:   "Parallelized network login cracker supporting HTTP forms, Basic Auth, SSH, FTP, MySQL, PostgreSQL, Redis, and SMB. Usage: `hydra -l admin -P pass.txt <target> <service>`.",
			InstallMethod: "apt",
			AptPkg:        "hydra",
			VersionArgs:   []string{"-h"},
		},
	}
}

// LookupKaliSpec finds a catalog spec by tool key or binary name.
func LookupKaliSpec(name string) (KaliToolSpec, bool) {
	q := strings.ToLower(strings.TrimSpace(name))
	q = strings.ReplaceAll(q, "-", "_")
	for _, sp := range KaliToolCatalog() {
		if sp.Key == q || strings.ReplaceAll(strings.ToLower(sp.Binary), "-", "_") == q {
			return sp, true
		}
	}
	return KaliToolSpec{}, false
}

// ensureKaliBinDir prepares `<dataDir>/bin` and `<dataDir>/tools`, writes the
// standalone `antifox-tool` CLI self-installer script into `<dataDir>/bin`, and
// prepends `<dataDir>/bin` and `$HOME/.local/bin` to the process PATH so any
// tool installed on demand is immediately available to Bash, shell_open, and
// child processes across container restarts.
func ensureKaliBinDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = filepath.Join(os.TempDir(), "antifox-data")
	}
	binDir := filepath.Join(dataDir, "bin")
	toolsDir := filepath.Join(dataDir, "tools")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(toolsDir, 0o755)

	// Write the standalone `antifox-tool` CLI helper into binDir so agents and
	// operators can also run `antifox-tool status|install|update <tool>` in Bash.
	cliPath := filepath.Join(binDir, "antifox-tool")
	_ = os.WriteFile(cliPath, []byte(antifoxToolCLIScript), 0o755)

	// Prepend binDir and ~/.local/bin to PATH idempotently.
	curPath := os.Getenv("PATH")
	parts := filepath.SplitList(curPath)
	has := func(dir string) bool {
		for _, p := range parts {
			if filepath.Clean(p) == filepath.Clean(dir) {
				return true
			}
		}
		return false
	}
	var prepend []string
	if absBin, err := filepath.Abs(binDir); err == nil && !has(absBin) {
		prepend = append(prepend, absBin)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		localBin := filepath.Join(home, ".local", "bin")
		if !has(localBin) {
			prepend = append(prepend, localBin)
		}
	}
	if len(prepend) > 0 {
		newPath := strings.Join(prepend, string(os.PathListSeparator))
		if curPath != "" {
			newPath += string(os.PathListSeparator) + curPath
		}
		_ = os.Setenv("PATH", newPath)
	}
	return binDir
}

// seedKaliToolCatalog seeds the curated Kali & ProjectDiscovery CLI tool catalog
// into the PostgreSQL `tools` table as `kind="shell"` rows (system=false, so
// operators can edit, disable, or delete them in the UI) ONCE per version flag.
// Uses ON CONFLICT (key) DO NOTHING so existing custom tools are never clobbered.
func (s *Server) seedKaliToolCatalog() {
	if s.m == nil || s.m.pg == nil {
		return
	}
	ensureKaliBinDir(s.dataDir)
	if v, _, _ := s.m.pg.GetSetting(settingKaliCatalogVersion); v == "true" {
		return
	}
	defaultAgents := []string{"worker", "pentest", "auto"}
	seeded := 0
	for _, sp := range KaliToolCatalog() {
		if exist, _ := s.m.pg.GetTool(sp.Key); exist != nil {
			continue
		}
		execMeta, _ := json.Marshal(map[string]any{
			"binary":           sp.Binary,
			"category":         sp.Category,
			"install_method":   sp.InstallMethod,
			"github_repo":      sp.GitHubRepo,
			"git_entry":        sp.GitEntryScript,
			"git_interpreter":  sp.GitInterpreter,
			"apt_pkg":          sp.AptPkg,
			"pip_pkg":          sp.PipPkg,
			"post_install_cmd": sp.PostInstallCmd,
		})
		err := s.m.pg.CreateCustomTool(&db.Tool{
			Key:         sp.Key,
			Description: sp.Description,
			Schema:      json.RawMessage("{}"),
			Agents:      defaultAgents,
			Enabled:     true,
			Kind:        "shell",
			Exec:        execMeta,
			Deferred:    false,
		})
		if err != nil {
			log.Printf("[kali-tools] seed %s failed: %v", sp.Key, err)
			continue
		}
		seeded++
	}
	_ = s.m.pg.SetSetting(settingKaliCatalogVersion, "true")
	if seeded > 0 {
		log.Printf("[kali-tools] Seeded %d Kali/ProjectDiscovery CLI tools into tools catalog", seeded)
	}
}

// resolveShellRowBinary determines the executable binary name for a `kind="shell"`
// tool row (checking `exec.binary`, then `LookupKaliSpec`, then falling back to `row.Key`).
func resolveShellRowBinary(row *db.Tool) string {
	if row == nil {
		return ""
	}
	if len(row.Exec) > 0 {
		var m struct {
			Binary string `json:"binary"`
		}
		if json.Unmarshal(row.Exec, &m) == nil && strings.TrimSpace(m.Binary) != "" {
			return strings.TrimSpace(m.Binary)
		}
	}
	if sp, ok := LookupKaliSpec(row.Key); ok && sp.Binary != "" {
		return sp.Binary
	}
	return row.Key
}

// formatShellHint formats one `kind="shell"` row for injection into Bash's tool
// description, annotating whether its binary is currently installed on PATH or
// can be installed on demand via `ensure_kali_tool`.
func formatShellHint(row *db.Tool) string {
	bin := resolveShellRowBinary(row)
	if _, err := exec.LookPath(bin); err == nil {
		return fmt.Sprintf("- %s [installed]: %s", row.Key, row.Description)
	}
	return fmt.Sprintf("- %s [not installed — call ensure_kali_tool(tool=%q, action=\"install\")]: %s", row.Key, row.Key, row.Description)
}

// CheckKaliToolStatus inspects whether a catalog tool is installed and optionally
// probes its version string.
func CheckKaliToolStatus(ctx context.Context, sp KaliToolSpec, probeVersion bool) KaliToolStatus {
	st := KaliToolStatus{
		Key:           sp.Key,
		Binary:        sp.Binary,
		Category:      sp.Category,
		InstallMethod: sp.InstallMethod,
		GitHubRepo:    sp.GitHubRepo,
	}
	p, err := exec.LookPath(sp.Binary)
	if err != nil {
		st.Installed = false
		st.Message = "not found in PATH (use ensure_kali_tool or `antifox-tool install " + sp.Key + "` to install latest version)"
		return st
	}
	st.Installed = true
	st.Path = p
	if probeVersion && len(sp.VersionArgs) > 0 {
		vctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		out, _ := exec.CommandContext(vctx, p, sp.VersionArgs...).CombinedOutput()
		line := strings.TrimSpace(string(out))
		if idx := strings.IndexByte(line, '\n'); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if len(line) > 120 {
			line = strings.ToValidUTF8(line[:120], "")
		}
		st.Version = line
	}
	return st
}

// InstallOrUpdateKaliTool installs or updates a single Kali/ProjectDiscovery tool
// into `<dataDir>/bin` (always pulling the latest upstream release/commit rather
// than relying on an outdated embedded binary).
// action: "status" | "install" | "update"
func InstallOrUpdateKaliTool(ctx context.Context, dataDir, name, action string) (KaliToolStatus, error) {
	binDir := ensureKaliBinDir(dataDir)
	toolsDir := filepath.Join(filepath.Dir(binDir), "tools")

	sp, ok := LookupKaliSpec(name)
	if !ok {
		return KaliToolStatus{}, fmt.Errorf("unknown Kali tool %q (supported: %s)", name, strings.Join(kaliToolKeys(), ", "))
	}

	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "install"
	}
	if action == "status" {
		return CheckKaliToolStatus(ctx, sp, true), nil
	}
	if action != "install" && action != "update" {
		return KaliToolStatus{}, fmt.Errorf("invalid action %q: must be status, install, or update", action)
	}

	// If action == "install" and binary is already present on PATH, return immediately.
	if action == "install" {
		if st := CheckKaliToolStatus(ctx, sp, true); st.Installed {
			st.Message = "already installed"
			return st, nil
		}
	}

	var installErr error
	switch sp.InstallMethod {
	case "github_release":
		installErr = installFromGitHubRelease(ctx, sp, binDir)
		if installErr != nil && sp.AptPkg != "" {
			installErr = installViaApt(ctx, sp.AptPkg)
		}
	case "git_script":
		installErr = installFromGitRepo(ctx, sp, toolsDir, binDir)
		if installErr != nil && sp.PipPkg != "" {
			installErr = installViaPip(ctx, sp.PipPkg, sp.Binary, binDir)
		}
		if installErr != nil && sp.AptPkg != "" {
			installErr = installViaApt(ctx, sp.AptPkg)
		}
	case "pip":
		installErr = installViaPip(ctx, sp.PipPkg, sp.Binary, binDir)
		if installErr != nil && sp.AptPkg != "" {
			installErr = installViaApt(ctx, sp.AptPkg)
		}
	case "apt":
		installErr = installViaApt(ctx, sp.AptPkg)
		if installErr != nil && sp.GitHubRepo != "" && sp.GitEntryScript != "" {
			installErr = installFromGitRepo(ctx, sp, toolsDir, binDir)
		}
	default:
		installErr = fmt.Errorf("unsupported install_method %q", sp.InstallMethod)
	}

	if installErr != nil {
		st := CheckKaliToolStatus(ctx, sp, false)
		st.Message = "install failed: " + installErr.Error()
		return st, installErr
	}

	// Run post-install hook (e.g. `nuclei -ut -silent` for latest nuclei-templates).
	if strings.TrimSpace(sp.PostInstallCmd) != "" {
		pctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_ = exec.CommandContext(pctx, "sh", "-c", sp.PostInstallCmd).Run()
		cancel()
	}

	st := CheckKaliToolStatus(ctx, sp, true)
	if !st.Installed {
		return st, fmt.Errorf("installation finished for %s, but binary %q is still not found in PATH", sp.Key, sp.Binary)
	}
	if action == "update" {
		st.Message = "updated to latest upstream release"
	} else {
		st.Message = "installed latest upstream release"
	}
	return st, nil
}

// installFromGitHubRelease queries GitHub's `releases/latest` API for `sp.GitHubRepo`,
// picks the asset matching `runtime.GOOS` and `runtime.GOARCH`, downloads the
// archive (.zip or .tar.gz), and extracts `sp.Binary` into `binDir`.
func installFromGitHubRelease(ctx context.Context, sp KaliToolSpec, binDir string) error {
	if sp.GitHubRepo == "" {
		return fmt.Errorf("missing github_repo for %s", sp.Key)
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", sp.GitHubRepo)
	rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "AntiFox-ToolInstaller/2.0")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch release metadata (%s): %w", sp.GitHubRepo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github api %s HTTP %d: %s", sp.GitHubRepo, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fmt.Errorf("decode release metadata: %w", err)
	}

	assetName, assetURL := selectReleaseAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if assetURL == "" {
		return fmt.Errorf("no matching release asset found in %s (%s) for %s/%s", sp.GitHubRepo, rel.TagName, runtime.GOOS, runtime.GOARCH)
	}

	dlReq, err := http.NewRequestWithContext(rctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}
	dlReq.Header.Set("User-Agent", "AntiFox-ToolInstaller/2.0")
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s HTTP %d", assetName, dlResp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(dlResp.Body, 250<<20)) // 250 MB ceiling
	if err != nil {
		return fmt.Errorf("read asset %s: %w", assetName, err)
	}

	dstPath := filepath.Join(binDir, sp.Binary)
	if runtime.GOOS == "windows" && !strings.HasSuffix(dstPath, ".exe") {
		dstPath += ".exe"
	}

	lowerName := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		return extractBinaryFromZip(data, sp.Binary, dstPath)
	case strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz"):
		return extractBinaryFromTarGz(data, sp.Binary, dstPath)
	default:
		// Raw binary asset
		return writeExecutableAtomic(dstPath, data)
	}
}

// selectReleaseAsset picks the best archive asset for the given goos/goarch.
func selectReleaseAsset(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}, goos, goarch string) (string, string) {
	osAliases := map[string][]string{
		"linux":   {"linux", "unknown-linux-musl", "unknown-linux-gnu"},
		"darwin":  {"darwin", "macos", "mac", "apple-darwin"},
		"windows": {"windows", "win64", "pc-windows"},
	}[goos]
	if len(osAliases) == 0 {
		osAliases = []string{goos}
	}

	archAliases := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64", "64bit"},
		"arm64": {"arm64", "aarch64"},
	}[goarch]
	if len(archAliases) == 0 {
		archAliases = []string{goarch}
	}

	bestScore := -1
	var bestName, bestURL string
	for _, a := range assets {
		ln := strings.ToLower(a.Name)
		// Skip checksums, signatures, SBOMs, and source/deb/rpm packages when looking for portable archives.
		if strings.HasSuffix(ln, ".txt") || strings.HasSuffix(ln, ".sha256") ||
			strings.HasSuffix(ln, ".sig") || strings.HasSuffix(ln, ".sbom") ||
			strings.HasSuffix(ln, ".deb") || strings.HasSuffix(ln, ".rpm") ||
			strings.HasSuffix(ln, ".apk") || strings.Contains(ln, "checksums") {
			continue
		}
		osMatch := false
		for _, oa := range osAliases {
			if strings.Contains(ln, oa) {
				osMatch = true
				break
			}
		}
		archMatch := false
		for _, aa := range archAliases {
			if strings.Contains(ln, aa) {
				archMatch = true
				break
			}
		}
		if !osMatch || !archMatch {
			continue
		}
		score := 1
		if strings.HasSuffix(ln, ".zip") || strings.HasSuffix(ln, ".tar.gz") || strings.HasSuffix(ln, ".tgz") {
			score += 2
		}
		if strings.Contains(ln, "musl") {
			score += 1 // prefer statically-linked musl builds on Linux when available
		}
		if score > bestScore {
			bestScore = score
			bestName = a.Name
			bestURL = a.BrowserDownloadURL
		}
	}
	return bestName, bestURL
}

func extractBinaryFromZip(data []byte, wantBin, dstPath string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	wantLower := strings.ToLower(wantBin)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		if base == wantLower || base == wantLower+".exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			binBytes, err := io.ReadAll(io.LimitReader(rc, 250<<20))
			rc.Close()
			if err != nil {
				return err
			}
			return writeExecutableAtomic(dstPath, binBytes)
		}
	}
	return fmt.Errorf("binary %q not found inside zip archive", wantBin)
}

func extractBinaryFromTarGz(data []byte, wantBin, dstPath string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	wantLower := strings.ToLower(wantBin)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := strings.ToLower(filepath.Base(hdr.Name))
		if base == wantLower || base == wantLower+".exe" {
			binBytes, err := io.ReadAll(io.LimitReader(tr, 250<<20))
			if err != nil {
				return err
			}
			return writeExecutableAtomic(dstPath, binBytes)
		}
	}
	return fmt.Errorf("binary %q not found inside tar.gz archive", wantBin)
}

func writeExecutableAtomic(dstPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}

// installFromGitRepo clones or updates (`git pull --ff-only`) the upstream
// repository under `<toolsDir>/<key>` and creates an executable wrapper in
// `<binDir>/<binary>` so script-based tools (sqlmap, jwt_tool, testssl.sh, nikto)
// are always up-to-date without requiring root privileges.
func installFromGitRepo(ctx context.Context, sp KaliToolSpec, toolsDir, binDir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed on PATH")
	}
	repoDir := filepath.Join(toolsDir, sp.Key)
	repoURL := "https://github.com/" + sp.GitHubRepo + ".git"
	gctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		cmd := exec.CommandContext(gctx, "git", "-C", repoDir, "pull", "--ff-only")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull %s: %v (%s)", sp.GitHubRepo, err, strings.TrimSpace(string(out)))
		}
	} else {
		_ = os.RemoveAll(repoDir)
		cmd := exec.CommandContext(gctx, "git", "clone", "--depth", "1", repoURL, repoDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone %s: %v (%s)", sp.GitHubRepo, err, strings.TrimSpace(string(out)))
		}
	}

	// Install Python requirements if requirements.txt exists and interpreter is python3.
	reqFile := filepath.Join(repoDir, "requirements.txt")
	if sp.GitInterpreter == "python3" {
		if _, err := os.Stat(reqFile); err == nil {
			if py := detectPython(); py != "" {
				_ = exec.CommandContext(gctx, py, "-m", "pip", "install", "--quiet", "--user", "-r", reqFile).Run()
				_ = exec.CommandContext(gctx, py, "-m", "pip", "install", "--quiet", "--break-system-packages", "-r", reqFile).Run()
			}
		}
	}

	entry := filepath.Join(repoDir, sp.GitEntryScript)
	_ = os.Chmod(entry, 0o755)
	interp := strings.TrimSpace(sp.GitInterpreter)
	if interp == "" {
		interp = "sh"
	}
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %s %q \"$@\"\n", interp, entry)
	return writeExecutableAtomic(filepath.Join(binDir, sp.Binary), []byte(wrapper))
}

// installViaPip installs/upgrades a Python CLI package via pip and links it into binDir.
func installViaPip(ctx context.Context, pkg, binary, binDir string) error {
	py := detectPython()
	if py == "" {
		return fmt.Errorf("python3 interpreter not found")
	}
	pctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Try --break-system-packages (Debian Bookworm / PEP 668 containers), then --user, then plain pip.
	attempts := [][]string{
		{"-m", "pip", "install", "--upgrade", "--break-system-packages", pkg},
		{"-m", "pip", "install", "--upgrade", "--user", pkg},
		{"-m", "pip", "install", "--upgrade", pkg},
	}
	var lastOut []byte
	var lastErr error
	for _, args := range attempts {
		out, err := exec.CommandContext(pctx, py, args...).CombinedOutput()
		if err == nil {
			lastErr = nil
			break
		}
		lastOut = out
		lastErr = err
	}
	if lastErr != nil {
		return fmt.Errorf("pip install %s: %v (%s)", pkg, lastErr, strings.TrimSpace(string(lastOut)))
	}
	// If the binary is still not directly in binDir, create a python -m wrapper when possible.
	if _, err := exec.LookPath(binary); err != nil {
		wrapper := fmt.Sprintf("#!/bin/sh\nexec %q -m %s \"$@\"\n", py, pkg)
		_ = writeExecutableAtomic(filepath.Join(binDir, binary), []byte(wrapper))
	}
	return nil
}

// installViaApt installs or upgrades a Debian/Kali package via apt-get when running
// as root (or passwordless sudo) inside a container or VM.
func installViaApt(ctx context.Context, pkg string) error {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("apt-get not available on this host")
	}
	actx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	shCmd := fmt.Sprintf("apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s", pkg)
	if os.Geteuid() != 0 {
		if _, err := exec.LookPath("sudo"); err == nil {
			shCmd = "sudo -n sh -c '" + shCmd + "'"
		} else {
			return fmt.Errorf("apt-get requires root privileges (euid=%d)", os.Geteuid())
		}
	}
	out, err := exec.CommandContext(actx, "sh", "-c", shCmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get install %s: %v (%s)", pkg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func kaliToolKeys() []string {
	cat := KaliToolCatalog()
	keys := make([]string, len(cat))
	for i, sp := range cat {
		keys[i] = sp.Key
	}
	return keys
}

// toolEnsureKaliTool builds the host tool `ensure_kali_tool` so worker, pentest,
// and auto agents can check, install, or update any Kali/ProjectDiscovery tool
// on demand at runtime without failing when a binary is absent or outdated.
func (s *Server) toolEnsureKaliTool() actool.CoreTool {
	return wrTool(
		"ensure_kali_tool",
		"Check status, install on demand, or update to the latest upstream release for any tool in the Kali & ProjectDiscovery CLI catalog ("+strings.Join(kaliToolKeys(), ", ")+"). Installs into `<dataDir>/bin` (which is on PATH and persists across restarts) so tools are never stuck at an old embedded version.",
		objSchema(map[string]any{
			"tool":   strParam("Tool name (e.g. `httpx`, `nuclei`, `subfinder`, `dnsx`, `naabu`, `katana`, `ffuf`, `sqlmap`, `dalfox`, `jwt_tool`, `testssl`, `arjun`), category (`recon`, `web_discovery`, `vuln_scan`, `exploitation`, `auth_crypto`), or `all`."),
			"action": strParam("`status` (check if installed and version), `install` (install latest if missing; default), or `update` (force upgrade to latest upstream release/templates)."),
		}, "tool"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			var req struct {
				Tool   string `json:"tool"`
				Action string `json:"action"`
			}
			if err := json.Unmarshal(in, &req); err != nil {
				return actool.Errorf("invalid input: " + err.Error()), nil
			}
			target := strings.ToLower(strings.TrimSpace(req.Tool))
			action := strings.ToLower(strings.TrimSpace(req.Action))
			if action == "" {
				action = "install"
			}
			if target == "" {
				return actool.Errorf("tool is required (e.g. \"httpx\", \"nuclei\", \"recon\", or \"all\")"), nil
			}

			// Batch mode: "all" or category name
			var specs []KaliToolSpec
			for _, sp := range KaliToolCatalog() {
				if target == "all" || target == sp.Category {
					specs = append(specs, sp)
				}
			}
			if len(specs) > 0 {
				results := make([]KaliToolStatus, 0, len(specs))
				for _, sp := range specs {
					if action == "status" {
						results = append(results, CheckKaliToolStatus(ctx, sp, false))
						continue
					}
					st, _ := InstallOrUpdateKaliTool(ctx, s.dataDir, sp.Key, action)
					results = append(results, st)
				}
				return jsonResult(map[string]any{
					"action":  action,
					"target":  target,
					"results": results,
				})
			}

			st, err := InstallOrUpdateKaliTool(ctx, s.dataDir, target, action)
			if err != nil {
				b, _ := json.Marshal(st)
				return actool.Errorf(fmt.Sprintf("%v\n%s", err, string(b))), nil
			}
			return jsonResult(st)
		},
	)
}

// GET /api/tools/kali — returns the Kali & ProjectDiscovery catalog with live install status.
func (s *Server) pgListKaliTools(w http.ResponseWriter, r *http.Request) {
	ensureKaliBinDir(s.dataDir)
	probeVer := r.URL.Query().Get("version") == "true"
	cat := KaliToolCatalog()
	out := make([]KaliToolStatus, 0, len(cat))
	for _, sp := range cat {
		out = append(out, CheckKaliToolStatus(r.Context(), sp, probeVer))
	}
	writeJSON(w, 200, map[string]any{
		"bin_dir": filepath.Join(s.dataDir, "bin"),
		"tools":   out,
	})
}

// POST /api/tools/kali/install — installs or updates one or more Kali catalog tools.
func (s *Server) pgInstallKaliTool(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeErr(w, 403, "Only administrators can install or update host CLI tools")
		return
	}
	var req struct {
		Tool   string `json:"tool"`
		Action string `json:"action"` // install | update | status
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	target := strings.ToLower(strings.TrimSpace(req.Tool))
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "install"
	}
	if target == "" {
		writeErr(w, 400, "tool is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var specs []KaliToolSpec
	for _, sp := range KaliToolCatalog() {
		if target == "all" || target == sp.Category {
			specs = append(specs, sp)
		}
	}
	if len(specs) > 0 {
		results := make([]KaliToolStatus, 0, len(specs))
		for _, sp := range specs {
			st, _ := InstallOrUpdateKaliTool(ctx, s.dataDir, sp.Key, action)
			results = append(results, st)
		}
		writeJSON(w, 200, map[string]any{"results": results})
		return
	}

	st, err := InstallOrUpdateKaliTool(ctx, s.dataDir, target, action)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error(), "status": st})
		return
	}
	writeJSON(w, 200, st)
}

// antifoxToolCLIScript is materialized to `<dataDir>/bin/antifox-tool` so any
// Bash command or container user can run `antifox-tool status|install|update <tool>`
// directly from the shell to fetch the latest upstream release.
const antifoxToolCLIScript = `#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$(dirname "$BIN_DIR")/tools"
mkdir -p "$BIN_DIR" "$TOOLS_DIR"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

usage() {
  cat <<EOF
Usage: antifox-tool <status|install|update> [tool_name|--all]

Supported tools (always fetches latest upstream release/commit):
  Recon:         subfinder, dnsx, naabu, httpx, nmap, whatweb, wafw00f
  Web Discovery: katana, ffuf, gobuster, feroxbuster, arjun
  Vuln Scan:     nuclei, nikto, dalfox, testssl, sslscan
  Exploitation:  sqlmap, interactsh_client, jwt_tool, hydra
EOF
}

install_gh_release() {
  local repo="$1" bin="$2"
  echo "[antifox-tool] Fetching latest release for $repo ($OS/$ARCH)..."
  local url
  url="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
    | grep -Eo '"browser_download_url":\s*"[^"]+"' \
    | cut -d'"' -f4 \
    | grep -iE "${OS}|unknown-linux" \
    | grep -iE "${ARCH}|x86_64|aarch64|64bit" \
    | grep -viE '\.(txt|sha256|sig|sbom|deb|rpm|apk)$' \
    | head -n 1)"
  if [ -z "$url" ]; then
    echo "[antifox-tool] ERROR: No release asset found for $repo ($OS/$ARCH)" >&2
    return 1
  fi
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' RETURN
  local archive="$tmpdir/$(basename "$url")"
  curl -fsSL "$url" -o "$archive"
  case "$archive" in
    *.zip)    unzip -q "$archive" -d "$tmpdir" ;;
    *.tar.gz|*.tgz) tar -xzf "$archive" -C "$tmpdir" ;;
    *)        cp "$archive" "$tmpdir/$bin" ;;
  esac
  local found
  found="$(find "$tmpdir" -type f -name "$bin" | head -n 1)"
  if [ -z "$found" ]; then
    echo "[antifox-tool] ERROR: Binary $bin not found in archive" >&2
    return 1
  fi
  install -m 0755 "$found" "$BIN_DIR/$bin"
  echo "[antifox-tool] Installed $bin -> $BIN_DIR/$bin"
}

install_git_tool() {
  local key="$1" repo="$2" entry="$3" interp="$4" bin="$5"
  local dest="$TOOLS_DIR/$key"
  if [ -d "$dest/.git" ]; then
    git -C "$dest" pull --ff-only
  else
    rm -rf "$dest"
    git clone --depth 1 "https://github.com/${repo}.git" "$dest"
  fi
  if [ "$interp" = "python3" ] && [ -f "$dest/requirements.txt" ]; then
    python3 -m pip install --quiet --break-system-packages -r "$dest/requirements.txt" 2>/dev/null || \
      python3 -m pip install --quiet --user -r "$dest/requirements.txt" 2>/dev/null || true
  fi
  cat > "$BIN_DIR/$bin" <<WRAP
#!/bin/sh
exec $interp "$dest/$entry" "\$@"
WRAP
  chmod 0755 "$BIN_DIR/$bin"
  echo "[antifox-tool] Installed $bin -> $BIN_DIR/$bin (from $repo)"
}

install_one() {
  local t="$1" mode="$2"
  local bin="$t"
  [ "$t" = "interactsh_client" ] && bin="interactsh-client"
  if [ "$mode" = "install" ] && command -v "$bin" >/dev/null 2>&1; then
    echo "[antifox-tool] $t ($bin) already installed at $(command -v "$bin")"
    return 0
  fi
  case "$t" in
    subfinder)         install_gh_release "projectdiscovery/subfinder" "subfinder" ;;
    dnsx)              install_gh_release "projectdiscovery/dnsx" "dnsx" ;;
    naabu)             install_gh_release "projectdiscovery/naabu" "naabu" ;;
    httpx)             install_gh_release "projectdiscovery/httpx" "httpx" ;;
    katana)            install_gh_release "projectdiscovery/katana" "katana" ;;
    nuclei)            install_gh_release "projectdiscovery/nuclei" "nuclei" && "$BIN_DIR/nuclei" -ut -silent || true ;;
    interactsh_client) install_gh_release "projectdiscovery/interactsh" "interactsh-client" ;;
    ffuf)              install_gh_release "ffuf/ffuf" "ffuf" ;;
    gobuster)          install_gh_release "OJ/gobuster" "gobuster" ;;
    feroxbuster)       install_gh_release "epi052/feroxbuster" "feroxbuster" ;;
    dalfox)            install_gh_release "hahwul/dalfox" "dalfox" ;;
    sqlmap)            install_git_tool "sqlmap" "sqlmapproject/sqlmap" "sqlmap.py" "python3" "sqlmap" ;;
    jwt_tool)          install_git_tool "jwt_tool" "ticarpi/jwt_tool" "jwt_tool.py" "python3" "jwt_tool" ;;
    testssl)           install_git_tool "testssl" "drwetter/testssl.sh" "testssl.sh" "bash" "testssl" ;;
    nikto)             install_git_tool "nikto" "sullo/nikto" "program/nikto.pl" "perl" "nikto" ;;
    whatweb)           install_git_tool "whatweb" "urbanadventurer/WhatWeb" "whatweb" "ruby" "whatweb" ;;
    arjun|wafw00f)
      python3 -m pip install --upgrade --break-system-packages "$t" 2>/dev/null || \
        python3 -m pip install --upgrade --user "$t"
      ;;
    nmap|sslscan|hydra)
      apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$t"
      ;;
    *)
      echo "[antifox-tool] Unknown tool: $t" >&2
      return 1
      ;;
  esac
}

ALL_TOOLS=(subfinder dnsx naabu httpx nmap whatweb wafw00f katana ffuf gobuster feroxbuster arjun nuclei nikto dalfox testssl sslscan sqlmap interactsh_client jwt_tool hydra)

cmd="${1:-status}"
shift || true

case "$cmd" in
  status)
    for t in "${ALL_TOOLS[@]}"; do
      bin="$t"
      [ "$t" = "interactsh_client" ] && bin="interactsh-client"
      if p="$(command -v "$bin" 2>/dev/null)"; then
        printf "  [installed] %-20s -> %s\n" "$t" "$p"
      else
        printf "  [missing]   %-20s (run: antifox-tool install %s)\n" "$t" "$t"
      fi
    done
    ;;
  install|update)
    if [ $# -eq 0 ]; then
      usage
      exit 1
    fi
    for arg in "$@"; do
      if [ "$arg" = "--all" ] || [ "$arg" = "all" ]; then
        for t in "${ALL_TOOLS[@]}"; do
          install_one "$t" "$cmd" || true
        done
      else
        install_one "$arg" "$cmd"
      fi
    done
    ;;
  *)
    usage
    exit 1
    ;;
esac
`
