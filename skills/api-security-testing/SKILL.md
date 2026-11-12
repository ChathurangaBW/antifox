---
name: api-security-testing
description: APIProfessional skills and methodology for safety testing
version: 1.0.0
---

# APISecurity Testing

## Overview

This skill provides methods, tools and best practices for API security testing.

## Test Scope

### 1. Accreditation and delegation of authority

**Test item:**
- TokenValidation
- TokenExpiry processing
- Permission Control
- Role permission validation

### 2. Input Validation

**Test item:**
- Parameter Type Validation
- Data length limit
- Special Character Processing
- SQLInject protection.
- XSSPrevention

### 3. Business Logic

**Test item:**
- Workflow Validation
- State transition
- Concurrency Control
- Operational rules

### 4. Error Handling

**Test item:**
- Error Message Disclosure
- Stack Trace
- Exposure to sensitive information

## Test Method

### 1. APIFound

**Identified API endpoint:**
```bash
# Scan with Directory
gobuster dir -u https://target.com -w api-wordlist.txt

# UsingBurp SuitePassive Scan
# Bruce application, ObserveAPICall

# AnalysisJavaScriptFile
# FindAPIEnd Definition
```

### 2. Authentication test

**Token test:**
```http
# Testing an invalid token
GET /api/user
Authorization: Bearer invalid_token

# Test expiredToken
GET /api/user
Authorization: Bearer expired_token

# None of the testsToken
GET /api/user
```

**JWT test:**
```bash
# Usingjwt_tool
python jwt_tool.py <JWT_TOKEN>

# Test algorithm confusion
python jwt_tool.py <JWT_TOKEN> -X a

# Test key violence crack
python jwt_tool.py <JWT_TOKEN> -C -d wordlist.txt
```

### 3. Authorized Testing

**Horizontal competence:**
```http
# UserAAccess UsersBResources
GET /api/user/123
Authorization: Bearer user_a_token

# Should return.403
```

**Vertical permissions:**
```http
# General User Access Administrator Interface
GET /api/admin/users
Authorization: Bearer user_token

# Should return.403
```

### 4. Enter Validation Test

**SQL injection:**
```http
POST /api/search
{
  "query": "test' OR '1'='1"
}
```

**Command injection:**
```http
POST /api/execute
{
  "command": "ping; id"
}
```

**XXE:**
```http
POST /api/parse
Content-Type: application/xml

<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<foo>&xxe;</foo>
```

### 5. Speed limit test

**Test rate limit:**
```python
import requests

for i in range(1000):
    response = requests.get('https://target.com/api/endpoint')
    print(f"Request {i}: {response.status_code}")
```

## Using the Tools

### Postman

**Create test collection:**
1. Import API documents
2. Set authentication
3. Create Test Example
4. Run automation tests

### Burp Suite

**API scan:**
1. Configure API peer
2. Set authentication
3. Run Active Scan
4. Analysis Results

### OWASP ZAP

```bash
# APIScan
zap-cli quick-scan --self-contained \
  --start-options '-config api.disablekey=true' \
  http://target.com/api
```

### REST-Attacker

```bash
# ScanOpenAPINorms
rest-attacker scan openapi.yaml
```

## Common Vulnerabilities

### 1. Authentication Bypass

**Token validation defect:**
- Weak Token Generation
- TokenPredictability
- TokenDoes not verify the token signature

### 2. Privilege Escalation

**IDOR:**
- Direct Object Reference
- No resource ownership verified

### 3. Information Disclosure

**Error message:**
- Detailed error information
- Stack Trace
- Sensitive data

### 4. Inject the hole.

**Common injection:**
- SQLInjection
- NoSQLInjection
- Command Injection
- XXE

### 5. Business Logic

**Logical defect:**
- Price Manipulation
- Rate Limit Bypass
- State Changes

## Test Checklist

### Authentication test
- [ ] TokenValidation
- [ ] TokenExpiry processing
- [ ] Weak Token Test
- [ ] TokenReplay the attack.

### Authorized Testing
- [ ] Horizontal permission test
- [ ] Vertical Permission Test
- [ ] Role permission validation
- [ ] Resource access control

### Input Validation
- [ ] SQLInjection test.
- [ ] XSS Testing
- [ ] Command injection test.
- [ ] XXE Testing
- [ ] Parameter contamination

### Business Logic
- [ ] Workflow Validation
- [ ] State transition
- [ ] Concurrency Control
- [ ] Operational rules

### Error Handling
- [ ] Error Message Disclosure
- [ ] Stack Trace
- [ ] Exposure to sensitive information

## Preventive Measures

### Recommended Approach

1. **Authentication**
   - Use StrengthToken
   - Realizing Token Refresh
   - Authenticate Token signature

2. **Delegation of authority**
   - Role-based access control
   - Validation of resource ownership
   - Principle of Least Privilege

3. **Input Validation**
   - Parameter Type Validation
   - Data length limit
   - Allowlist Validation

4. **Error Handling**
   - Unified Error Response
   - Do not divulge details.
   - Log the Errors

5. **Speed limit**
   - Achieved API flow
   - Preventing violent breakdowns
   - Surveillance exception request.

## Important Notes

- Perform only in an authorized testing environment
- Avoid impact on API.
- Watch out for different API versions.
- Watch the frequency of requests during the test.