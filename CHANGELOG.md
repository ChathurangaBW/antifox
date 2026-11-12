# Update Log - Changelog

This project's version update summary. The format follows [Keep a Changelog](https://keepachangelog.com/), and versions follow [SemVer](https://semver.org/).

## v2.1.1

- **Docker Build Fix (`non-free` Debian Bookworm package removal)**: Removed Debian `non-free` `nikto` from the container `apt-get install` list and preinstalled `libnet-ssleay-perl`, `libxml-writer-perl`, `libio-socket-ssl-perl`, and `libjson-perl` from Debian `main` so container builds succeed reliably on `python:3.12-slim-bookworm`.
- **Up-to-Date Upstream Nikto Self-Installer**: Switched `nikto`'s primary `InstallMethod` in `KaliToolCatalog` from `apt` (2.1.5) to `git_script` (`sullo/nikto` 2.6+), retaining `apt` as fallback, so `ensure_kali_tool` and `antifox-tool install nikto` deploy current upstream Nikto checks cleanly.

## v2.1.0

- **Dynamic Kali & ProjectDiscovery Tool Catalog**: Seeded 21 curated security CLI tools (`subfinder`, `dnsx`, `naabu`, `httpx`, `nmap`, `whatweb`, `wafw00f`, `katana`, `ffuf`, `gobuster`, `feroxbuster`, `arjun`, `nuclei`, `nikto`, `dalfox`, `testssl`, `sslscan`, `sqlmap`, `interactsh_client`, `jwt_tool`, `hydra`) into the PostgreSQL `tools` catalog (`kind="shell"`), with live `[installed]` / `[not installed]` status hints in `Bash` and pre-installed reconnaissance/verification packages in the Docker image.
- **On-Demand Latest-Release Self-Installer**: Added `ensure_kali_tool`, `/api/tools/kali`, `/api/tools/kali/install`, and the persistent `<dataDir>/bin/antifox-tool` CLI helper so missing or outdated tools install and upgrade directly from upstream GitHub Releases, Git repositories, `pip`, or `apt` rather than freezing old binaries inside releases.
- **Deterministic Scanner Ingestion & Asset Auto-Enrichment**: Added `scan_file` and `scan_jsonl` parameters to `insert_assets` for direct JSON/JSONL ingestion from `subfinder`, `dnsx`, `naabu`, `httpx`, `katana`, and `ffuf`; wired automatic DNS resolution (`ResolveDomain`) and HTTP site probing (`ProbeSite`) on asset upsert; added the `tech` compatibility alias and endpoint IP fallback.
- **Finding Accuracy, Traffic Correlation & Guard Fixes**: Preserved 4-level severity (`critical` no longer downgraded to `high`); fixed finding deduplication collisions across 12+ character vulnerability class names; automatically populated `finding_traffic` links when reproduction steps resolve to MITM packets; disambiguated `ResolveSteps` by host and expected status code; and extended `guard` attribution to interactive `shell_open`/`shell_send` sessions.
- **Global English Prompts, Startup Schema Sync & Updater**: Enforced English across agent prompts and settlement wrap-ups, replaced garbled tool/prompt strings, added UTF-8 safe truncation, refreshed built-in tool schemas on startup for existing databases, and updated `selfupdate.Repo` to `ChathurangaBW/antifox`.

## v2.0.4

- **Severity normalization restored**: `Serious`, `High risk`, `Low risk.`, and `Information` map to their correct canonical levels again. The English conversion left the alias keys capitalized while the input was lower-cased, so those aliases never matched and findings were downgraded.
- **Legacy title cleanup restored**: Chinese severity prefixes, full-width parentheses, and `U+3000` in target names are normalized again, so data created before the conversion still produces clean report titles.
- **Log levels fixed**: `failed`, `drop`, `reject`, `refused`, `retry`, and `disabled` raise the intended level; the lower-cased message was previously compared against capitalized keywords.
- **Ellipsis restored**: truncated output, the API-key hint, and traffic snippet markers use `U+2026` again instead of three ASCII dots.
- **Tests**: regression coverage added for severity aliases, title prefixes, name normalization, and log levels; the `db` test package compiles again after a pre-existing `CreateTask` signature mismatch.

## v2.0.3

- **Custom LLM session headers**: Each LLM configuration can set an HTTP header whose value is automatically the current session ID, supporting gateways such as OpenCode that use sessions for prompt caching or sticky routing.
- **Global outbound proxy**: Target traffic from every agent can use one outbound proxy (HTTP/HTTPS/SOCKS5), hiding the source IP or using a jump host; includes a usability test that displays the proxy egress IP.
- **Per-task egress selection**: When creating a task, choose whether it uses the global proxy or connects directly.
- **Confirmation enriches immediately**: Manually confirmed vulnerabilities are automatically enriched into structured data and are ready for delivery.
- **Report management**: New reports list only confirmed vulnerabilities by default (including status and CVSS); the body reuses enriched data.
- **Automatic evidence captions**: Evidence screenshot captions are generated from reproduction-step text and remain editable.
- **Vulnerability detail refinements**: Status dropdown, section headings, and one-click copy.
- **Traffic task filtering fix**: Task filtering now combines the task activity window, so repeated tests of the same target no longer mix traffic.
- Changed the system configuration page to a responsive two-column layout.

## v2.0.2

- **One-click update**: Check for and install new GitHub releases from the system configuration page (SHA256 verification, atomic replacement, and automatic rollback on failure) without affecting data.
- Release packages include the `start.sh` daemon script and a systemd template.

## v2.0.1

- Automatically link real traffic packets when reproduction steps are saved.

## v2.0.0

- Evidence-driven delivery: save reproduction steps in place, organize discovery pages by exploit-chain steps, link a real traffic packet to every step, and generate manually graded trilingual reports.
