---
name: secure-code-review
description: Professional skills and methodology for security code review
version: 1.0.0
---

# Security Code Review

## Overview

This skill provides methods, tools and best practices for the security code review.

## Scope of the review

### 1. Input Validation

**Inspection items:**
- User Input Authentication
- Parameter Authentication
- Data Filter
- Border inspection

### 2. Output Encoding

**Inspection items:**
- XSSPrevention
- Output Encoding
- Content security strategy
- Response Header Settings

### 3. Authentication and Authorization

**Inspection items:**
- Authentication Mechanism
- Session Management
- Permission Control
- Password Process

### 4. Encryption and Keys

**Inspection items:**
- Data Encryption
- Key Management
- Hashi Algorithm
- Random Number Generation

## Review methodology

### 1. Static Analysis

**Using SAST tools:**
```bash
# SonarQube
sonar-scanner

# Checkmarx
# UsingWebInterface

# Fortify
sourceanalyzer -b project build.sh
sourceanalyzer -b project -scan

# Semgrep
semgrep --config=auto .
```

### 2. Manual review

**Review list:**
- [ ] Input Validation
- [ ] Output Encoding
- [ ] SQLInjection
- [ ] XSSVulnerability
- [ ] Authentication and Authorization
- [ ] Encryption Use
- [ ] Error Handling
- [ ] Log Logs

### 3. Code Mode Identification

**Hazard function:**
```python
# PythonDangerous function
eval()
exec()
pickle.loads()
os.system()
subprocess.call()
```

```java
// JavaDangerous function
Runtime.exec()
ProcessBuilder()
Class.forName()
```

```php
// PHPDangerous function
eval()
exec()
system()
passthru()
```

## Common Leak Mode

### SQLInjection

**Hazard code:**
```java
String query = "SELECT * FROM users WHERE id = " + userId;
Statement stmt = connection.createStatement();
ResultSet rs = stmt.executeQuery(query);
```

**Safe code:**
```java
String query = "SELECT * FROM users WHERE id = ?";
PreparedStatement stmt = connection.prepareStatement(query);
stmt.setInt(1, userId);
ResultSet rs = stmt.executeQuery();
```

### XSSVulnerability

**Hazard code:**
```javascript
document.innerHTML = userInput;
element.innerHTML = "<div>" + userInput + "</div>";
```

**Safe code:**
```javascript
element.textContent = userInput;
element.setAttribute("data-value", userInput);
// Or use the encoder library
element.innerHTML = escapeHtml(userInput);
```

### Command Injection

**Hazard code:**
```python
import os
os.system("ping " + user_input)
```

**Safe code:**
```python
import subprocess
subprocess.run(["ping", "-c", "1", validated_input])
```

### Path Through

**Hazard code:**
```java
String filePath = "/uploads/" + fileName;
File file = new File(filePath);
```

**Safe code:**
```java
String basePath = "/uploads/";
String fileName = Paths.get(fileName).getFileName().toString();
String filePath = basePath + fileName;
File file = new File(filePath);
if (!file.getCanonicalPath().startsWith(basePath)) {
    throw new SecurityException("Invalid path");
}
```

### Hard-coded Credentials

**Hazard code:**
```java
String apiKey = "1234567890abcdef";
String password = "admin123";
```

**Safe code:**
```java
String apiKey = System.getenv("API_KEY");
String password = keyStore.getPassword("db_password");
```

## Using the Tools

### SonarQube

```bash
# StartSonarQube
docker run -d -p 9000:9000 sonarqube

# Run Scan
sonar-scanner \
  -Dsonar.projectKey=myproject \
  -Dsonar.sources=. \
  -Dsonar.host.url=http://localhost:9000
```

### Semgrep

```bash
# Install
pip install semgrep

# Run Scan
semgrep --config=auto .

# Use rules
semgrep --config=p/security-audit .
```

### CodeQL

```bash
# Create Database
codeql database create database --language=java --source-root=.

# Run Query
codeql database analyze database security-and-quality.qls --format=sarif-latest
```

## Review of the List

### Input Validation
- [ ] All user input is authenticated
- [ ] Validate with White List
- [ ] Validate data type and scope
- [ ] Handle Special Characters

### Output Encoding
- [ ] HTMLOutput Encoding
- [ ] URLEncoding
- [ ] JavaScriptEncoding
- [ ] SQLParametricisation

### Authentication and Authorization
- [ ] Strong password policy
- [ ] Secure Session Management
- [ ] Permission Check
- [ ] Multifactor authentication

### Encryption
- [ ] Use strong encryption algorithms
- [ ] Key Security Storage
- [ ] Transfer Encryption
- [ ] Store Encryption

### Error Handling
- [ ] Do not divulge sensitive information
- [ ] Unified Error Response
- [ ] Log the Errors
- [ ] Anomalous treatment

## Best Practices

### 1. Safety Code

- FollowOWASP Top 10
- Guidance on the use of safety codes
- Code review process
- Security training

### 2. Automation tools

- Integrated SAST Tool
- CI/CDSecurity checks
- Automation scan
- Analysis of results

### 3. Code review process

- Peer review
- Security expert review
- Periodic review
- Recording problems

## Important Notes

- Combined tools and manual reviews
- Focus on business logic vulnerabilities
- Regular updates of the rules of the tool
- Create a safe coding culture