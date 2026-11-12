# Update Log - Changelog

This project's version update summary. The format follows [Keep a Changelog](https://keepachangelog.com/), and versions follow [SemVer](https://semver.org/).

## v2.0.3

- **Custom LLM session headers**: Each LLM configuration can set an HTTP header whose value is automatically the current session ID, supporting gateways such as OpenCode that use sessions for prompt caching or sticky routing.
- **Global outbound proxy**: Target traffic from every agent can use one outbound proxy (HTTP/HTTPS/SOCKS5), hiding the source IP or using a jump host; includes a usability test that displays the proxy egress IP.
- **Per-task egress selection**: When creating a task, choose whether it uses the global proxy or connects directly.
- **Confirmation enriches immediately**: Manually confirmed vulnerabilities are automatically enriched into structured data and are ready for delivery.
- **Report management**: New reports list only confirmed vulnerabilities by default (including status and CVSS); the body reuses enriched data.
- **Automatic evidence captions**: Evidence screenshot captions are generated from reproduction-step text and remain editable.
- **Vulnerability detail refinements**: Status dropdown, section headings, and one-click copy.
- **Flow Job Screening Repair**:The task-screening superimposed task activity window and multiple tests of the same target do not lead to traffic.
- Change the system configuration page to two columns to fit the layout.

## v2.0.2

- **One Key Update**:Check and install from the system configuration page GitHub New versionSHA256 Verify / Atom replacement / Failed to roll back automatically) and data is not affected.
- Publication of products attached `start.sh` Protect Scripts with systemd template.

## v2.0.1

- Auto-link real flow packages when the step is dropped.

## v2.0.0

- Evidence-driven delivery: recurrence process in-situ storage, discovery pages step by step, real flow packages per step; manual trilingual reporting.
