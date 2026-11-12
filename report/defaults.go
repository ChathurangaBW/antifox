package report

// Default deterministic scaffold content shared by all reports (brand-free,
// neutral), in three locales: zh-CN | zh-TW | en-US. The delivery conventions
// (personnel, PTES process, ~56-item test checklist, tools, standards) are
// fixed; the AI only writes the narrative prose. Company name / personnel are
// user-configured in Report Management; these are locale-aware fallbacks.
//
// Selector functions (methodologyIntro/methodology/tools/standards/checklist/
// defaultPersonnel) project the requested locale; build.go calls them with the
// report locale.

type kv struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// checklistItem is one row of the 2.3 test checklist (already localized).
type checklistItem struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
}

// ---------------------------------------------------------------------------
// 2.1 test personnel — neutral, brand-free fallbacks (no contact). Overridden by
// user-configured personnel in report settings.
// ---------------------------------------------------------------------------
func defaultPersonnel(loc string) []map[string]string {
	switch normLocale(loc) {
	case "zh-TW":
		return []map[string]string{
			{"name": "Lead Consultant / Security Engineer", "role": "Lead Consultant / Security Engineer", "contact": ""},
			{"name": "Red Team Assessment Group", "role": "Offensive Security Team", "contact": ""},
		}
	case "en-US":
		return []map[string]string{
			{"name": "Lead Consultant / Security Engineer", "role": "Lead Consultant / Security Engineer", "contact": ""},
			{"name": "Red Team Assessment Group", "role": "Offensive Security Team", "contact": ""},
		}
	default:
		return []map[string]string{
			{"name": "Lead Consultant / Security Engineer", "role": "Lead Consultant / Security Engineer", "contact": ""},
			{"name": "Red Team Assessment Group", "role": "Offensive Security Team", "contact": ""},
		}
	}
}

// ---------------------------------------------------------------------------
// 2.2 test workflow (PTES 8 phases)
// ---------------------------------------------------------------------------
func methodologyIntro(loc string) string {
	return pick(loc,
		"The penetration testing service workflow follows the internationally recognized Penetration Testing Execution Standard (PTES), ensuring that every engagement is delivered in a standardized, high-quality manner. The workflow comprises the following 8 phases:",
		"The penetration testing service workflow follows the internationally recognized Penetration Testing Execution Standard (PTES), ensuring that every engagement is delivered in a standardized, high-quality manner. The workflow comprises the following 8 phases:",
		"The penetration testing service workflow follows the internationally recognized Penetration Testing Execution Standard (PTES), ensuring that every engagement is delivered in a standardized, high-quality manner. The workflow comprises the following 8 phases:")
}

func methodology(loc string) []kv {
	switch normLocale(loc) {
	case "zh-TW":
		return []kv{
			{"Pre-engagement Interactions", "Project initiation and preparation: both parties agree on the testing scope and specific test items, and authorization confirmation, personnel, and tooling are finalized."},
			{"Intelligence Gathering", "Open-source information collection, asset mapping, and site structure analysis around the authorized assets, culminating in the penetration testing implementation plan."},
			{"Threat Modeling", "Analysis of business assets and processes, assessment of threat actors and their capabilities, and identification of the primary attack surface and risk paths."},
			{"Vulnerability Analysis", "Correlation analysis of scanner results combined with manual verification, reproduction in a controlled environment, and derivation of analytical conclusions."},
			{"Exploitation", "Based on the analysis results, the optimal attack path is selected to perform low-impact, precise verification and confirm the authenticity of each risk."},
			{"Post-Exploitation", "Within the authorized scope, assessment of lateral movement, privilege escalation, and reachability of sensitive data following successful exploitation."},
			{"Reporting", "Consolidation of vulnerability details, evidence, CVSS scoring, and risk ratings into a deliverable penetration test report."},
			{"Remediation Support", "Working with the client to complete remediation based on the report, and conducting retesting where required to confirm that vulnerabilities are fully closed."},
		}
	case "en-US":
		return []kv{
			{"Pre-engagement Interactions", "Project initiation and preparation: both parties agree on the testing scope and specific test items, and authorization confirmation, personnel, and tooling are finalized."},
			{"Intelligence Gathering", "Open-source information collection, asset mapping, and site structure analysis around the authorized assets, culminating in the penetration testing implementation plan."},
			{"Threat Modeling", "Analysis of business assets and processes, assessment of threat actors and their capabilities, and identification of the primary attack surface and risk paths."},
			{"Vulnerability Analysis", "Correlation analysis of scanner results combined with manual verification, reproduction in a controlled environment, and derivation of analytical conclusions."},
			{"Exploitation", "Based on the analysis results, the optimal attack path is selected to perform low-impact, precise verification and confirm the authenticity of each risk."},
			{"Post-Exploitation", "Within the authorized scope, assessment of lateral movement, privilege escalation, and reachability of sensitive data following successful exploitation."},
			{"Reporting", "Consolidation of vulnerability details, evidence, CVSS scoring, and risk ratings into a deliverable penetration test report."},
			{"Remediation Support", "Working with the client to complete remediation based on the report, and conducting retesting where required to confirm that vulnerabilities are fully closed."},
		}
	default:
		return []kv{
			{"Pre-engagement Interactions", "Project initiation and preparation: both parties agree on the testing scope and specific test items, and authorization confirmation, personnel, and tooling are finalized."},
			{"Intelligence Gathering", "Open-source information collection, asset mapping, and site structure analysis around the authorized assets, culminating in the penetration testing implementation plan."},
			{"Threat Modeling", "Analysis of business assets and processes, assessment of threat actors and their capabilities, and identification of the primary attack surface and risk paths."},
			{"Vulnerability Analysis", "Correlation analysis of scanner results combined with manual verification, reproduction in a controlled environment, and derivation of analytical conclusions."},
			{"Exploitation", "Based on the analysis results, the optimal attack path is selected to perform low-impact, precise verification and confirm the authenticity of each risk."},
			{"Post-Exploitation", "Within the authorized scope, assessment of lateral movement, privilege escalation, and reachability of sensitive data following successful exploitation."},
			{"Reporting", "Consolidation of vulnerability details, evidence, CVSS scoring, and risk ratings into a deliverable penetration test report."},
			{"Remediation Support", "Working with the client to complete remediation based on the report, and conducting retesting where required to confirm that vulnerabilities are fully closed."},
		}
	}
}

// ---------------------------------------------------------------------------
// 2.4 test tools
// ---------------------------------------------------------------------------
func tools(loc string) []kv {
	switch normLocale(loc) {
	case "zh-TW":
		return []kv{
			{"Burp Suite", "An integrated platform for web application security testing, serving as the primary proxy and packet-capture tool for this penetration test."},
			{"SQLMap", "An open-source tool for detecting and exploiting SQL injection vulnerabilities."},
			{"Nmap", "A network port scanning and service identification tool."},
			{"Nuclei", "A YAML-template-based vulnerability scanner used for bulk fingerprinting and vulnerability verification."},
			{"cURL / httpie", "Command-line tools for crafting HTTP requests and analyzing responses."},
			{"Wappalyzer / EHole", "Web application framework and fingerprint identification tools."},
			{"In-house POC Library", "An internally maintained library of vulnerability verification scripts, used together with a local vulnerability database index."},
		}
	case "en-US":
		return []kv{
			{"Burp Suite", "An integrated platform for web application security testing, serving as the primary proxy and packet-capture tool for this penetration test."},
			{"SQLMap", "An open-source tool for detecting and exploiting SQL injection vulnerabilities."},
			{"Nmap", "A network port scanning and service identification tool."},
			{"Nuclei", "A YAML-template-based vulnerability scanner used for bulk fingerprinting and vulnerability verification."},
			{"cURL / httpie", "Command-line tools for crafting HTTP requests and analyzing responses."},
			{"Wappalyzer / EHole", "Web application framework and fingerprint identification tools."},
			{"In-house POC Library", "An internally maintained library of vulnerability verification scripts, used together with a local vulnerability database index."},
		}
	default:
		return []kv{
			{"Burp Suite", "An integrated platform for web application security testing, serving as the primary proxy and packet-capture tool for this penetration test."},
			{"SQLMap", "An open-source tool for detecting and exploiting SQL injection vulnerabilities."},
			{"Nmap", "A network port scanning and service identification tool."},
			{"Nuclei", "A YAML-template-based vulnerability scanner used for bulk fingerprinting and vulnerability verification."},
			{"cURL / httpie", "Command-line tools for crafting HTTP requests and analyzing responses."},
			{"Wappalyzer / EHole", "Web application framework and fingerprint identification tools."},
			{"In-house POC Library", "An internally maintained library of vulnerability verification scripts, used together with a local vulnerability database index."},
		}
	}
}

// ---------------------------------------------------------------------------
// 2.5 reference standards
// ---------------------------------------------------------------------------
func standards(loc string) []string {
	switch normLocale(loc) {
	case "zh-TW":
		return []string{
			"GB/T 25069-2022 Information Security Technology — Terminology",
			"GB/T 22239-2019 Information Security Technology — Baseline for Classified Protection of Cybersecurity",
			"GB/T 28448-2019 Information Security Technology — Evaluation Requirements for Classified Protection of Cybersecurity",
			"GB/T 20271-2006 Information Security Technology — Common Security Techniques Requirement for Information Systems",
			"PTES (Penetration Testing Execution Standard)",
			"OWASP Top 10: 2021",
		}
	case "en-US":
		return []string{
			"GB/T 25069-2022 Information Security Technology — Terminology",
			"GB/T 22239-2019 Information Security Technology — Baseline for Classified Protection of Cybersecurity",
			"GB/T 28448-2019 Information Security Technology — Evaluation Requirements for Classified Protection of Cybersecurity",
			"GB/T 20271-2006 Information Security Technology — Common Security Techniques Requirement for Information Systems",
			"PTES (Penetration Testing Execution Standard)",
			"OWASP Top 10: 2021",
		}
	default:
		return []string{
			"GB/T 25069-2022 Information Security Technology — Terminology",
			"GB/T 22239-2019 Information Security Technology — Baseline for Classified Protection of Cybersecurity",
			"GB/T 28448-2019 Information Security Technology — Evaluation Requirements for Classified Protection of Cybersecurity",
			"GB/T 20271-2006 Information Security Technology — Common Security Techniques Requirement for Information Systems",
			"PTES (Penetration Testing Execution Standard)",
			"OWASP Top 10: 2021",
		}
	}
}

// ---------------------------------------------------------------------------
// 2.3 test checklist (~56 items). checklistRow holds all three locales; checklist(loc)
// projects the requested one. AI-returned checklist_hits are matched against the
// localized Name (see buildChecklist) -> matched rows render as failed (red bold).
// ---------------------------------------------------------------------------
type checklistRow struct {
	CatCN, CatTW, CatEN    string
	NameCN, NameTW, NameEN string
	DescCN, DescTW, DescEN string
}

func checklist(loc string) []checklistItem {
	out := make([]checklistItem, 0, len(checklistAll))
	for _, r := range checklistAll {
		switch normLocale(loc) {
		case "zh-TW":
			out = append(out, checklistItem{Category: r.CatEN, Name: r.NameEN, Desc: r.DescEN})
		case "en-US":
			out = append(out, checklistItem{Category: r.CatEN, Name: r.NameEN, Desc: r.DescEN})
		default:
			out = append(out, checklistItem{Category: r.CatEN, Name: r.NameEN, Desc: r.DescEN})
		}
	}
	return out
}

var checklistAll = []checklistRow{
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Brute Force", "Brute Force", "Brute Force", "Commonly found in login systems; large-scale attempts of login credentials against a known username.", "Commonly found in login systems; large-scale attempts of login credentials against a known username.", "Commonly found in login systems; large-scale attempts of login credentials against a known username."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "SMS Bombing", "SMS Bombing", "SMS Bombing", "Replaying the SMS verification-code request to flood a target with text messages (SMS bombing).", "Replaying the SMS verification-code request to flood a target with text messages (SMS bombing).", "Replaying the SMS verification-code request to flood a target with text messages (SMS bombing)."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Authentication Bypass", "Authentication Bypass", "Authentication Bypass", "The application lacks a valid token mechanism, allowing authentication information to be modified to bypass server-side authorization.", "The application lacks a valid token mechanism, allowing authentication information to be modified to bypass server-side authorization.", "The application lacks a valid token mechanism, allowing authentication information to be modified to bypass server-side authorization."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Malicious Registration", "Malicious Registration", "Malicious Registration", "The user registration function can be abused by an attacker to perform bulk registration.", "The user registration function can be abused by an attacker to perform bulk registration.", "The user registration function can be abused by an attacker to perform bulk registration."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Session Fixation", "Session Fixation", "Session Fixation", "The session ID does not change before and after login, creating a session-hijacking risk.", "The session ID does not change before and after login, creating a session-hijacking risk.", "The session ID does not change before and after login, creating a session-hijacking risk."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Plaintext Password Transmission", "Plaintext Password Transmission", "Plaintext Password Transmission", "Usernames or passwords are transmitted without encryption, making them susceptible to sniffing and interception.", "Usernames or passwords are transmitted without encryption, making them susceptible to sniffing and interception.", "Usernames or passwords are transmitted without encryption, making them susceptible to sniffing and interception."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Password Recovery Logic Flaw", "Password Recovery Logic Flaw", "Password Recovery Logic Flaw", "Logic flaws in the password recovery flow can be exploited to illegitimately reset a password.", "Logic flaws in the password recovery flow can be exploited to illegitimately reset a password.", "Logic flaws in the password recovery flow can be exploited to illegitimately reset a password."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "OAuth Authentication Flaw", "OAuth Authentication Flaw", "OAuth Authentication Flaw", "Incomplete OAuth authentication allows unauthorized login to other users' accounts.", "Incomplete OAuth authentication allows unauthorized login to other users' accounts.", "Incomplete OAuth authentication allows unauthorized login to other users' accounts."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Weak Password", "Weak Password", "Weak Password", "Discovery of weak passwords in the system through brute forcing, guessing, and similar techniques.", "Discovery of weak passwords in the system through brute forcing, guessing, and similar techniques.", "Discovery of weak passwords in the system through brute forcing, guessing, and similar techniques."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Broken Authentication & Session Management", "Broken Authentication & Session Management", "Broken Authentication & Session Management", "Vulnerabilities in the authentication and session management functions, including missing session timeout settings.", "Vulnerabilities in the authentication and session management functions, including missing session timeout settings.", "Vulnerabilities in the authentication and session management functions, including missing session timeout settings."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "CAPTCHA Functional Flaw", "CAPTCHA Functional Flaw", "CAPTCHA Functional Flaw", "The CAPTCHA does not expire and can be reused, enabling brute-force attacks.", "The CAPTCHA does not expire and can be reused, enabling brute-force attacks.", "The CAPTCHA does not expire and can be reused, enabling brute-force attacks."},
	{"Authentication & Session Management", "Authentication & Session Management", "Authentication & Session Management", "Username Enumeration", "Username Enumeration", "Username Enumeration", "Different accounts return different error messages, allowing enumeration of valid accounts in the system.", "Different accounts return different error messages, allowing enumeration of valid accounts in the system.", "Different accounts return different error messages, allowing enumeration of valid accounts in the system."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "CORS Cross-Origin Resource Sharing Vulnerability", "CORS Cross-Origin Resource Sharing Vulnerability", "CORS Cross-Origin Resource Sharing Vulnerability", "Improper CORS configuration can be exploited to steal users' sensitive data.", "Improper CORS configuration can be exploited to steal users' sensitive data.", "Improper CORS configuration can be exploited to steal users' sensitive data."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "Flash Cross-Domain Access Vulnerability", "Flash Cross-Domain Access Vulnerability", "Flash Cross-Domain Access Vulnerability", "An incorrect crossdomain.xml policy leads to cross-domain security issues.", "An incorrect crossdomain.xml policy leads to cross-domain security issues.", "An incorrect crossdomain.xml policy leads to cross-domain security issues."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "Outdated jQuery Version", "Outdated jQuery Version", "Outdated jQuery Version", "An outdated jQuery version introduces cross-site scripting and other known security vulnerabilities.", "An outdated jQuery version introduces cross-site scripting and other known security vulnerabilities.", "An outdated jQuery version introduces cross-site scripting and other known security vulnerabilities."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "JSONP Hijacking", "JSONP Hijacking", "JSONP Hijacking", "Cross-domain data transfer lacks authorization checks, allowing sensitive data to be read by malicious scripts.", "Cross-domain data transfer lacks authorization checks, allowing sensitive data to be read by malicious scripts.", "Cross-domain data transfer lacks authorization checks, allowing sensitive data to be read by malicious scripts."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "Cross-Site Request Forgery (CSRF)", "Cross-Site Request Forgery (CSRF)", "Cross-Site Request Forgery (CSRF)", "Forces an authenticated user to perform unintended actions within a web application.", "Forces an authenticated user to perform unintended actions within a web application.", "Forces an authenticated user to perform unintended actions within a web application."},
	{"Client-Side Security", "Client-Side Security", "Client-Side Security", "Cross-Site Scripting (XSS)", "Cross-Site Scripting (XSS)", "Cross-Site Scripting (XSS)", "Injects malicious scripts into web pages, executing malicious code when the page is viewed.", "Injects malicious scripts into web pages, executing malicious code when the page is viewed.", "Injects malicious scripts into web pages, executing malicious code when the page is viewed."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "Code Injection", "Code Injection", "Code Injection", "An attacker submits code as input, which the server processes as dynamic code.", "An attacker submits code as input, which the server processes as dynamic code.", "An attacker submits code as input, which the server processes as dynamic code."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "LDAP Injection", "LDAP Injection", "LDAP Injection", "Leverages LDAP query syntax to disclose sensitive user and host information.", "Leverages LDAP query syntax to disclose sensitive user and host information.", "Leverages LDAP query syntax to disclose sensitive user and host information."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "Command Injection", "Command Injection", "Command Injection", "Injects OS commands via an HTTP request to execute system commands on the server.", "Injects OS commands via an HTTP request to execute system commands on the server.", "Injects OS commands via an HTTP request to execute system commands on the server."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "XFF Injection", "XFF Injection", "XFF Injection", "Performs SQL injection by manipulating the X-Forwarded-For header.", "Performs SQL injection by manipulating the X-Forwarded-For header.", "Performs SQL injection by manipulating the X-Forwarded-For header."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "SSI Injection", "SSI Injection", "SSI Injection", "May allow injection of code into HTML pages and even remote code execution.", "May allow injection of code into HTML pages and even remote code execution.", "May allow injection of code into HTML pages and even remote code execution."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "SQL Injection", "SQL Injection", "SQL Injection", "By inserting SQL queries, an attacker can read or tamper with data, perform administrative operations, or execute system commands.", "By inserting SQL queries, an attacker can read or tamper with data, perform administrative operations, or execute system commands.", "By inserting SQL queries, an attacker can read or tamper with data, perform administrative operations, or execute system commands."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "XPath Injection", "XPath Injection", "XPath Injection", "Injects elements into an XPath query, potentially bypassing authentication.", "Injects elements into an XPath query, potentially bypassing authentication.", "Injects elements into an XPath query, potentially bypassing authentication."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "Link or Frame Injection", "Link or Frame Injection", "Link or Frame Injection", "Injects frame or iframe tags containing malicious content.", "Injects frame or iframe tags containing malicious content.", "Injects frame or iframe tags containing malicious content."},
	{"Malicious Input / Injection", "Malicious Input / Injection", "Malicious Input / Injection", "XML External Entity (XXE) Injection", "XML External Entity (XXE) Injection", "XML External Entity (XXE) Injection", "XML external entities can be abused to read system files, perform SSRF, and mount other attacks.", "XML external entities can be abused to read system files, perform SSRF, and mount other attacks.", "XML external entities can be abused to read system files, perform SSRF, and mount other attacks."},
	{"Access Control", "Access Control", "Access Control", "Local File Inclusion", "Local File Inclusion", "Local File Inclusion", "Includes and executes code on the target server to obtain sensitive files or privileges.", "Includes and executes code on the target server to obtain sensitive files or privileges.", "Includes and executes code on the target server to obtain sensitive files or privileges."},
	{"Access Control", "Access Control", "Access Control", "Vertical Privilege Escalation", "Vertical Privilege Escalation", "Vertical Privilege Escalation", "A low-privilege account can access or operate high-privilege functionality.", "A low-privilege account can access or operate high-privilege functionality.", "A low-privilege account can access or operate high-privilege functionality."},
	{"Access Control", "Access Control", "Access Control", "Unauthorized Access", "Unauthorized Access", "Unauthorized Access", "Internal resources or functions can be accessed directly without login or authorization.", "Internal resources or functions can be accessed directly without login or authorization.", "Internal resources or functions can be accessed directly without login or authorization."},
	{"Access Control", "Access Control", "Access Control", "Arbitrary File Upload", "Arbitrary File Upload", "Arbitrary File Upload", "Uploading an executable script file to gain the ability to run commands on the server.", "Uploading an executable script file to gain the ability to run commands on the server.", "Uploading an executable script file to gain the ability to run commands on the server."},
	{"Access Control", "Access Control", "Access Control", "Arbitrary File Download", "Arbitrary File Download", "Arbitrary File Download", "System files, configuration files, and similar can be browsed or downloaded arbitrarily.", "System files, configuration files, and similar can be browsed or downloaded arbitrarily.", "System files, configuration files, and similar can be browsed or downloaded arbitrarily."},
	{"Access Control", "Access Control", "Access Control", "Horizontal Privilege Escalation", "Horizontal Privilege Escalation", "Horizontal Privilege Escalation", "Users at the same privilege level can access or operate other users' resources.", "Users at the same privilege level can access or operate other users' resources.", "Users at the same privilege level can access or operate other users' resources."},
	{"Access Control", "Access Control", "Access Control", "Remote File Inclusion", "Remote File Inclusion", "Remote File Inclusion", "Crafting parameters to include remote code that is executed on the server.", "Crafting parameters to include remote code that is executed on the server.", "Crafting parameters to include remote code that is executed on the server."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Error Page Information Disclosure", "Error Page Information Disclosure", "Error Page Information Disclosure", "Error messages contain sensitive server or database information.", "Error messages contain sensitive server or database information.", "Error messages contain sensitive server or database information."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Test / Backup File Leak", "Test / Backup File Leak", "Test / Backup File Leak", "Leftover outdated files, backup pages, and test files are exposed.", "Leftover outdated files, backup pages, and test files are exposed.", "Leftover outdated files, backup pages, and test files are exposed."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", ".git Source Code Leak", ".git Source Code Leak", ".git Source Code Leak", "The .git directory was not removed at deployment, allowing source code to be recovered.", "The .git directory was not removed at deployment, allowing source code to be recovered.", "The .git directory was not removed at deployment, allowing source code to be recovered."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "GitHub Information Leak", "GitHub Information Leak", "GitHub Information Leak", "Sensitive code, keys, and similar are inadvertently made public on GitHub.", "Sensitive code, keys, and similar are inadvertently made public on GitHub.", "Sensitive code, keys, and similar are inadvertently made public on GitHub."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "IIS Short Filename Disclosure", "IIS Short Filename Disclosure", "IIS Short Filename Disclosure", "An IIS file enumeration vulnerability that allows enumeration of files in the root directory.", "An IIS file enumeration vulnerability that allows enumeration of files in the root directory.", "An IIS file enumeration vulnerability that allows enumeration of files in the root directory."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Absolute Path Disclosure", "Absolute Path Disclosure", "Absolute Path Disclosure", "The application discloses its absolute path on the host.", "The application discloses its absolute path on the host.", "The application discloses its absolute path on the host."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Directory Listing", "Directory Listing", "Directory Listing", "A website configuration flaw results in a browsable directory listing vulnerability.", "A website configuration flaw results in a browsable directory listing vulnerability.", "A website configuration flaw results in a browsable directory listing vulnerability."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Sensitive Information Disclosure", "Sensitive Information Disclosure", "Sensitive Information Disclosure", "Configuration files contain sensitive data such as passwords, keys, internal addresses, and employee identifiers.", "Configuration files contain sensitive data such as passwords, keys, internal addresses, and employee identifiers.", "Configuration files contain sensitive data such as passwords, keys, internal addresses, and employee identifiers."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Sensitive File Leak", "Sensitive File Leak", "Sensitive File Leak", "Program files, configuration files, logs, backups, and database files are exposed.", "Program files, configuration files, logs, backups, and database files are exposed.", "Program files, configuration files, logs, backups, and database files are exposed."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Default Page Leak", "Default Page Leak", "Default Page Leak", "Default-installed middleware and frameworks ship with sample pages and documentation.", "Default-installed middleware and frameworks ship with sample pages and documentation.", "Default-installed middleware and frameworks ship with sample pages and documentation."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Directory Traversal", "Directory Traversal", "Directory Traversal", "An attacker views the website's directory structure and files to mount further attacks.", "An attacker views the website's directory structure and files to mount further attacks.", "An attacker views the website's directory structure and files to mount further attacks."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Residual Intrusion Traces", "Residual Intrusion Traces", "Residual Intrusion Traces", "Traces of a prior intrusion remain in the application, such as WebShell files.", "Traces of a prior intrusion remain in the application, such as WebShell files.", "Traces of a prior intrusion remain in the application, such as WebShell files."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "SVN Source Code Leak", "SVN Source Code Leak", "SVN Source Code Leak", "Source code is obtained through an SVN source code disclosure vulnerability.", "Source code is obtained through an SVN source code disclosure vulnerability.", "Source code is obtained through an SVN source code disclosure vulnerability."},
	{"Information Disclosure", "Information Disclosure", "Information Disclosure", "Web Console Exposure", "Web Console Exposure", "Web Console Exposure", "The management console of a web container or database is exposed.", "The management console of a web container or database is exposed.", "The management console of a web container or database is exposed."},
	{"Security Misconfiguration", "Security Misconfiguration", "Security Misconfiguration", "Insecure HTTP Methods", "Insecure HTTP Methods", "Insecure HTTP Methods", "The server enables insecure methods such as TRACE, PUT, and DELETE.", "The server enables insecure methods such as TRACE, PUT, and DELETE.", "The server enables insecure methods such as TRACE, PUT, and DELETE."},
	{"Security Misconfiguration", "Security Misconfiguration", "Security Misconfiguration", "Server-Side Request Forgery (SSRF)", "Server-Side Request Forgery (SSRF)", "Server-Side Request Forgery (SSRF)", "The server makes requests on the attacker's behalf, allowing access to internal-network or local resources.", "The server makes requests on the attacker's behalf, allowing access to internal-network or local resources.", "The server makes requests on the attacker's behalf, allowing access to internal-network or local resources."},
	{"Security Misconfiguration", "Security Misconfiguration", "Security Misconfiguration", "Host Header Attack", "Host Header Attack", "Host Header Attack", "Malicious manipulation of the HTTP Host header to enable phishing, cache poisoning, and similar attacks.", "Malicious manipulation of the HTTP Host header to enable phishing, cache poisoning, and similar attacks.", "Malicious manipulation of the HTTP Host header to enable phishing, cache poisoning, and similar attacks."},
	{"Security Misconfiguration", "Security Misconfiguration", "Security Misconfiguration", "Slow HTTP DoS Attack", "Slow HTTP DoS Attack", "Slow HTTP DoS Attack", "Exhausts server resources through slow HTTP requests.", "Exhausts server resources through slow HTTP requests.", "Exhausts server resources through slow HTTP requests."},
	{"Security Misconfiguration", "Security Misconfiguration", "Security Misconfiguration", "Unnecessary Web Server Ports Open", "Unnecessary Web Server Ports Open", "Unnecessary Web Server Ports Open", "Default ports beyond the required business ports are left open.", "Default ports beyond the required business ports are left open.", "Default ports beyond the required business ports are left open."},
	{"Insecure Storage & Transport", "Insecure Storage & Transport", "Insecure Storage & Transport", "Insecure Storage", "Insecure Storage", "Insecure Storage", "Important information is not encrypted or is encrypted with insufficient strength.", "Important information is not encrypted or is encrypted with insufficient strength.", "Important information is not encrypted or is encrypted with insufficient strength."},
	{"Insecure Storage & Transport", "Insecure Storage & Transport", "Insecure Storage & Transport", "Sensitive Information in HTML", "Sensitive Information in HTML", "Sensitive Information in HTML", "HTML pages contain sensitive information.", "HTML pages contain sensitive information.", "HTML pages contain sensitive information."},
}
