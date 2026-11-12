---
name: cloud-security-audit
description: Professional skills and methodology for cloud safety audits
version: 1.0.0
---

# Cloud safety audit

## Overview

This skill provides the methods, tools, and best practices for cloud security auditing, covering mainstream cloud platforms such as AWS, Azure, and GCP.

## Audit coverage

### 1. Identity and access management

**Inspection items:**
- IAMPolicy Configuration
- User Permissions
- Role permissions
- Access Key Management

### 2. Network Security

**Inspection items:**
- Security Group Configuration
- NetworkACL
- VPCConfiguration
- Flow Encryption

### 3. Data Security

**Inspection items:**
- Data Encryption
- Key Management
- Backup Policy
- Data disaggregation

### 4. Compliance

**Inspection items:**
- Compliance framework
- Audit log
- Security alert.
- Event response

## AWSSecurity Audit

### IAMAudit

**Inspection of IAM strategy:**
```bash
# List allIAMUser
aws iam list-users

# List allIAMPolicy
aws iam list-policies

# Check User Permissions
aws iam list-user-policies --user-name username
aws iam list-attached-user-policies --user-name username

# Check Role Permissions
aws iam list-role-policies --role-name rolename
```

**Common problems:**
- Excessive Permissions
- Unused access keys
- The password policy is weak.
- MFANot enabled

### S3Security Audit

**Inspection of S3 storage drums:**
```bash
# List all storage barrels
aws s3 ls

# Check the storage drum policy
aws s3api get-bucket-policy --bucket bucketname

# Check storage drumsACL
aws s3api get-bucket-acl --bucket bucketname

# Check storage drum encryption
aws s3api get-bucket-encryption --bucket bucketname
```

**Common problems:**
- Public access
- Synchronising folder failed: %s: %s
- Version control not enabled
- Log logs not enabled

### Audit of the Security Group

**Inspection Security Unit:**
```bash
# List all security teams.
aws ec2 describe-security-groups

# Check Open Port
aws ec2 describe-security-groups --group-ids sg-xxx
```

**Common problems:**
- 0.0.0.0/0Open
- Unnecessary port open
- The rules are too loose.

### CloudTrailAudit

**Inspection of audit log:**
```bash
# List All Tracks
aws cloudtrail describe-trails

# Check log file integrity
aws cloudtrail get-trail-status --name trailname
```

## AzureSecurity Audit

### Subscriptions and Resources Group

**Checked subscription:**
```bash
# List all subscriptions
az account list

# Inspection Resource Group
az group list
```

### Cyber Security Unit

**Inspection of NSG:**
```bash
# List allNSG
az network nsg list

# CheckNSGRules
az network nsg rule list --nsg-name nsgname --resource-group rgname
```

### Store Account

**Inspection of storage account:**
```bash
# List all stored accounts
az storage account list

# Check access policy
az storage account show --name accountname --resource-group rgname
```

## GCPSecurity Audit

### Projects and organization

**Inspection items:**
```bash
# List all items
gcloud projects list

# CheckIAMPolicy
gcloud projects get-iam-policy project-id
```

### Calculating Engines

**Examples of inspections:**
```bash
# List all examples
gcloud compute instances list

# Check firewall rules
gcloud compute firewall-rules list
```

### Storage

**Inspection of storage drums:**
```bash
# List all storage barrels
gsutil ls

# Check storage drum privileges
gsutil iam get gs://bucketname
```

## Automation tools

### Scout Suite

```bash
# AWSAudit
scout aws

# AzureAudit
scout azure

# GCPAudit
scout gcp
```

### Prowler

```bash
# AWSSecurity audit
prowler -c check11,check12,check13

# Complete audit
prowler
```

### CloudSploit

```bash
# ScanAWSAccount
cloudsploit scan aws

# ScanAzureSubscriptions
cloudsploit scan azure
```

### Pacu

```bash
# AWSInfiltration test frame
pacu
```

## Audit list

### IAMSecurity
- [ ] Check User Permissions
- [ ] Check Role Permissions
- [ ] Check access keys
- [ ] Check password policy
- [ ] Check MFA usage

### Network Security
- [ ] Inspection of security team/NSG rules
- [ ] Check VPC Configuration
- [ ] Check the networkACL
- [ ] Check traffic encryption

### Data Security
- [ ] Check Data Encryption
- [ ] Check Key Management
- [ ] Check backup policy
- [ ] Check data disaggregation

### Compliance
- [ ] Inspection of audit logs
- [ ] Check the surveillance alarms.
- [ ] Check incident response
- [ ] Check compliance framework

## Common Security Issues

### 1. Excessive Permissions

**Question:**
- IAMThe strategy is too loose.
- User has administrator privileges
- It's too much of a role.

**Repairs:**
- Principle of Least Privilege
- Periodic review of competence
- Simulate using IAM strategy

### 2. Open Resources

**Question:**
- S3Storage drums are public
- Security team open.0.0.0.0/0
- Open access to databases

**Repairs:**
- Restricted access
- Use private networks
- Enable access control

### 3. Unencrypted Data

**Question:**
- Store unencrypted
- Transfer unencrypted
- Unable to manage key

**Repairs:**
- Enable encryption
- UsingTLS/SSL
- Use key management services

### 4. Log Missing

**Question:**
- Audit log not activated
- Log not kept
- Log not monitored

**Repairs:**
- EnableCloudTrail/Azure Monitor
- Set log retention policy
- Configure surveillance alarms

## Best Practices

### 1. Least Privilege

- Grant only necessary permissions
- Periodic review of competence
- Simulate using IAM strategy

### 2. Multiple layers of protection.

- Network layer protection
- Application layer protection
- Data layer protection

### 3. Surveillance and alarm.

- Enable audit log
- Configure surveillance alarms
- Create event response process

### 4. Compliance

- Compliance framework adhered to
- Periodic security audits
- Documentation security policy

## Important Notes

- Audit only in an authorized environment
- Avoiding effects on the production environment
- Pay attention to the differences between cloud platforms.
- Regular security audits conducted