---
name: container-security-testing
description: Professional skills and methodology for container safety testing
version: 1.0.0
---

# Container safety test

## Overview

This skill provides methods, tools and best practices for the safety testing of containers and covers packaging techniques such as Docker, Kubernetes.

## Test Scope

### 1. Image Security

**Inspection items:**
- Basic mirror leaks
- Reliance on bag holes
- Mirror Configuration
- Sensitive information

### 2. Runtime Security

**Inspection items:**
- Packaging Permissions
- Resource constraints
- Network Isolation
- File System

### 3. Orchestration Security

**Inspection items:**
- KubernetesConfiguration
- Service accounts
- RBAC
- Network Policy

## DockerSecurity Testing

### Mirror Scan

**Using Trivy:**
```bash
# Scan the image
trivy image nginx:latest

# Scan Local Mirrors
trivy image --input nginx.tar

# Show only high-risk holes
trivy image --severity HIGH,CRITICAL nginx:latest
```

**Using Claire:**
```bash
# StartClair
docker run -d --name clair clair:latest

# Scan the image
clair-scanner --ip 192.168.1.100 nginx:latest
```

**Using Docker Bench:**
```bash
# RunDockerSecurity benchmarking test
docker run --rm --net host --pid host --userns host --cap-add audit_control \
  -e DOCKER_CONTENT_TRUST=$DOCKER_CONTENT_TRUST \
  -v /etc:/etc:ro \
  -v /usr/bin/containerd:/usr/bin/containerd:ro \
  -v /usr/bin/runc:/usr/bin/runc:ro \
  -v /usr/lib/systemd:/usr/lib/systemd:ro \
  -v /var/lib:/var/lib:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  --label docker_bench_security \
  docker/docker-bench-security
```

### Container Configuration Check

**Inspection Dockerfile:**
```dockerfile
# Examples of security issues
FROM ubuntu:latest  # UsinglatestLabel
RUN apt-get update && apt-get install -y curl  # No Version Specified
COPY . /app  # Could contain sensitive files
ENV PASSWORD=secret  # Hard Encoding Password
USER root  # UsingrootUser
```

**Safety best practices:**
```dockerfile
# Use specific version
FROM ubuntu:20.04

# Specified package version
RUN apt-get update && apt-get install -y curl=7.68.0-1ubuntu2.7

# Use Non-UserootUser
RUN useradd -m appuser
USER appuser

# Minimize Mirrors
FROM alpine:3.15

# Multi-stage construction
FROM golang:1.18 AS builder
WORKDIR /app
COPY . .
RUN go build -o app

FROM alpine:3.15
COPY --from=builder /app/app /app
```

### Check when running

**Permission to inspect containers:**
```bash
# Check privileged containers
docker ps --filter "label=privileged=true"

# Check the mount host directory
docker inspect container_name | grep -A 10 Mounts

# Check the container network
docker network inspect network_name
```

**Inspection resource constraints:**
```bash
# Check memory limits
docker stats container_name

# CheckCPULimits
docker inspect container_name | grep -i cpu
```

## KubernetesSecurity Testing

### Configure Check

**Using kube-bench:**
```bash
# Runkube-bench
kube-bench run

# Check specific benchmarks
kube-bench run --targets master,node,etcd
```

**Using kube-hunter:**
```bash
# Runkube-hunter
kube-hunter --remote target-ip

# Active Mode
kube-hunter --active
```

### PodSecurity

**Inspection of Pod security strategy:**
```yaml
# It's not safe.PodConfigure
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      privileged: true  # Precise Mode
      runAsUser: 0  # rootUser
```

**Security configuration:**
```yaml
apiVersion: v1
kind: Pod
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 2000
  containers:
  - name: app
    image: nginx
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
        add:
        - NET_BIND_SERVICE
```

### RBACCheck

**Checking role privileges:**
```bash
# List all roles
kubectl get roles --all-namespaces

# Check role binding
kubectl get rolebindings --all-namespaces

# Examination of cluster roles
kubectl get clusterroles

# Check User Permissions
kubectl auth can-i --list --as=system:serviceaccount:default:sa-name
```

**Common problems:**
- Excessive Permissions
- Unused roles
- Unused service accounts

### Network Policy

**Inspection of web strategy:**
```bash
# List all network strategies
kubectl get networkpolicies --all-namespaces

# Check network policy configuration
kubectl describe networkpolicy policy-name -n namespace
```

**Examples of web strategy:**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

## Using the Tools

### Falco

**Operating security monitoring:**
```bash
# InstallFalco
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm install falco falcosecurity/falco

# Inspection rules
falco -r /etc/falco/rules.d/
```

### Aqua Security

```bash
# Scan the image
aqua image scan nginx:latest

# ScanKubernetesCluster
aqua k8s scan
```

### Snyk

```bash
# ScanDockerfile
snyk test --docker nginx:latest

# ScanKubernetesConfigure
snyk iac test k8s/
```

## Test Checklist

### Image Security
- [ ] Scan for base mirror leaks
- [ ] Scanning is dependent on the bag bug.
- [ ] Check Dockerfile configuration
- [ ] Check sensitive information leaks.

### Runtime Security
- [ ] Check container privileges
- [ ] Checking resource constraints
- [ ] Check network isolation.
- [ ] Check file system mount

### Orchestration Security
- [ ] Check Kubernetes configuration
- [ ] Check RBAC Configuration
- [ ] Check network policy
- [ ] Check Pod security strategy

## Common Security Issues

### 1. Mirror breach.

**Question:**
- Basic mirrors contain holes
- Reliance packs contain holes
- Not updated in time

**Repairs:**
- Scan mirrors on a regular basis
- Update base mirrors in a timely manner
- Use minimum mirror

### 2. Excessive Permissions

**Question:**
- Packagings run in root
- Precise Mode
- Mount Sensitive Directory

**Repairs:**
- Use non-root users
- Disable privileged mode
- Restricted document system access

### 3. Security Misconfiguration

**Question:**
- Default configuration is not safe
- Network strategy missing
- RBACSecurity Misconfiguration

**Repairs:**
- Follow security best practices
- Implementation of the web strategy
- Correct ConfigurationRBAC

### 4. Sensible information leaks

**Question:**
- Mirror contains keys
- Exposure to environmental variables
- Config file leak

**Repairs:**
- Use key management
- Avoid hard encoding
- Use Secret objects

## Best Practices

### 1. Image Security

- Use official base mirrors
- Regularly update mirrors
- Scan for mirror leaks
- Minimize mirror size

### 2. Runtime Security

- Use non-root users
- Restricted container rights
- Implementation of resource constraints
- Enable safe context

### 3. Orchestration Security

- Configure Network Policy
- ImplementationRBAC
- Use Pod security policy
- Enable audit log

## Important Notes

- Test only in an authorized environment
- Avoiding effects on the production environment
- Take note of the differences in the platforms of the different containers.
- Security scans conducted on a regular basis