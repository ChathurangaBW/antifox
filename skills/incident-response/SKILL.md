---
name: incident-response
description: Professional skills and methodology for security incident response
version: 1.0.0
---

# Security incident response

## Overview

This skill provides methods, tools and best practices for security incident response.

## Response process

### 1. Preparation phase

**Preparatory work:**
- Establish a Response Team
- Develop a Response Plan
- Preparation tools and resources
- Establishment of communication channels

### 2. Identification phase

**Identified event:**
- Security alert.
- Anomalous detection
- Log Analysis
- User Report

### 3. Repression phase

**Constraint measures:**
- Isolation of affected systems
- Disable Account
- Interrupt network connections
- Backup evidence

### 4. Clearance phase

**Threat clearance:**
- Remove malware
- Fix the Vulnerability
- Reset the Credentials
- Clean the back door.

### 5. Recovery phase

**Recovery system:**
- Restore backup
- Validation system integrity
- Surveillance system
- Gradual restoration of services

### 6. Wrap-up phase

**Lessons learned:**
- Event report
- Lessons learned
- Improvement measures
- Update Process

## Using the Tools

### Log Analysis

**Using Splunk:**
```bash
# Search Log
index=security event_type="failed_login"

# Statistical analysis
index=security | stats count by src_ip

# Time series analysis
index=security | timechart count by event_type
```

**Using ELK:**
```bash
# ElasticsearchQuery
GET /logs/_search
{
  "query": {
    "match": {
      "event_type": "malware"
    }
  }
}
```

### Evidence tools

**Using Volatility:**
```bash
# Analyze memory mirrors
volatility -f memory.dump imageinfo

# List process
volatility -f memory.dump --profile=Win7SP1x64 pslist

# Memory of the extraction process
volatility -f memory.dump --profile=Win7SP1x64 memdump -p 1234 -D output/
```

**Using Autopsy:**
```bash
# StartAutopsy
# Create Case
# Add Evidence
# Analysis of data
```

### Network analysis

**Using Wiresark:**
```bash
# Capture flows
wireshark -i eth0

# AnalysisPCAPFile
wireshark -r capture.pcap

# Filter Flows
# Show Filters: ip.addr == 192.168.1.100
# Capture Filter: host 192.168.1.100
```

**Using tcpdump:**
```bash
# Capture flows
tcpdump -i eth0 -w capture.pcap

# Analysis of traffic
tcpdump -r capture.pcap -A
```

## Event Type

### Malicious software

**Response step:**
1. Isolation of affected systems
2. Collection of samples
3. Analysis of malware
4. Clearing the threat.
5. Fix the Vulnerability

**Tools:**
- VirusTotal
- Cuckoo Sandbox
- YARARule

### Data leak.

**Response step:**
1. Confirm leak range.
2. Stop the leak.
3. Assess the Impact
4. Notification to interested parties
5. Fix the Vulnerability

**Inspection items:**
- Quantities of leaked data
- Affected users
- Discover the channel.
- Data sensitivity

### Denial of Service

**Response step:**
1. Type of attack confirmed
2. Enable protective measures
3. Filter malign flow
4. Surveillance System Status
5. Restricted services

**Preventive Measures:**
- DDoSProtection services
- Flow cleaning.
- Flow-restrictive measures
- CDNPrevention

### Visits not authorized

**Response step:**
1. Disable affected accounts
2. Reset the Credentials
3. Check access log
4. Assessment of data access
5. Fix the Vulnerability

**Inspection items:**
- Access times
- Visit Contents
- Visited Sources
- Data Modification

## List of responses

### Preparation phase
- [ ] Establish a Response Team
- [ ] Develop a Response Plan
- [ ] Prepare the Tools
- [ ] Establishment of communication channels

### Identification phase
- [ ] Confirming event
- [ ] Collection of information
- [ ] Assess the Impact
- [ ] Record time lines

### Repression phase
- [ ] Isolation system.
- [ ] Disable Account
- [ ] Interrupt connection
- [ ] Backup evidence

### Clearance phase
- [ ] Remove Threat
- [ ] Fix the Vulnerability
- [ ] Reset the Credentials
- [ ] Validate Clear

### Recovery phase
- [ ] Restart the system.
- [ ] Validate integrity
- [ ] Surveillance system
- [ ] Restoring services

### Wrap-up phase
- [ ] Preparation of the report
- [ ] Lessons learned
- [ ] Improvement measures
- [ ] Update Process

## Best Practices

### 1. Ready.

- Establish a Response Team
- Develop a Response Plan
- Periodic rehearsals
- Prepare the Tools

### 2. Response

- Quick Response
- Systemized processing
- Record all operations
- Protection of evidence

### 3. Communication

- Internal communication
- External notifications
- State Update
- Ex post facto reports

### 4. Improvements

- Event analysis
- Process improvements
- Tools Update
- Training upgrades

## Important Notes

- Quick Response
- Protection of evidence
- Record the Actions
- Comply with Applicable Laws and Regulations