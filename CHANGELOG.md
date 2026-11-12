# Update Log - Changelog

This project's version update summary. The format follows [Keep a Changelog](https://keepachangelog.com/), and versions follow [SemVer](https://semver.org/).

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
