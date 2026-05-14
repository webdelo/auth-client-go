# Kubernetes Best Practices and Standards

This document contains Kubernetes rules for DevOps tasks. Each rule has a unique identifier (K8-XXXX) for reference in code reviews and task execution.

> **Related**: For Docker rules, see [docker.md](./docker.md)
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми Kubernetes задачами
**Текущая версия K8s**: 1.35 (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| K8-001 | Set resource requests and limits | ✅ | [Resource Management](#k8-001-set-resource-requests-and-limits) |
| K8-002 | Use readiness and liveness probes | ✅ | [Health Probes](#k8-002-use-readiness-and-liveness-probes) |
| K8-003 | Run as non-root (securityContext) | ✅ | [Security Context](#k8-003-run-as-non-root-securitycontext) |
| K8-004 | Use ConfigMaps for config, Secrets for secrets | | [Configuration](#k8-004-use-configmaps-for-config-secrets-for-secrets) |
| K8-005 | Set PodDisruptionBudget for HA | | [Pod Disruption Budget](#k8-005-set-poddisruptionbudget-for-ha) |
| K8-006 | Use namespaces for isolation | | [Namespaces](#k8-006-use-namespaces-for-isolation) |
| K8-007 | Set terminationGracePeriodSeconds | | [Graceful Shutdown](#k8-007-set-terminationgraceperiodseconds) |
| K8-008 | Use HorizontalPodAutoscaler | | [Autoscaling](#k8-008-use-horizontalpodautoscaler) |
| K8-009 | Implement graceful shutdown | ✅ | [Graceful Shutdown](#k8-009-implement-graceful-shutdown) |
| K8-010 | Use Network Policies | | [Network Policies](#k8-010-use-network-policies) |
| K8-011 | Apply Pod Security Standards | ✅ | [Pod Security](#k8-011-apply-pod-security-standards) |

---

## 1. Resource Management

### K8-001: Set Resource Requests and Limits

**Severity:** CRITICAL

ALWAYS set resource requests and limits for all containers.

| Type | Purpose | Enforcement |
|------|---------|-------------|
| **Requests** | Minimum for scheduling | Scheduler placement |
| **Limits** | Maximum allowed | Kubelet via cgroups |

```yaml
# K8-001: GOOD - Proper resource configuration
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: app
    image: myapp:v1.0.0
    resources:
      requests:
        memory: "256Mi"  # K8-001: Use Mi, not m!
        cpu: "250m"      # 0.25 CPU cores
      limits:
        memory: "512Mi"
        cpu: "500m"
```

**CRITICAL WARNING**: `400m` memory = 0.4 BYTES, NOT 400 MiB! Always use `400Mi`.

```yaml
# K8-001: VIOLATION - No resource limits
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:v1.0.0
    # No resources defined = can consume all node resources!
```

#### Quality of Service (QoS) Classes

| QoS Class | Definition | Eviction Priority | Use Case |
|-----------|------------|-------------------|----------|
| **Guaranteed** | requests == limits | Lowest (last) | Production critical |
| **Burstable** | requests < limits | Medium | General apps |
| **BestEffort** | No requests/limits | Highest (first) | Non-critical batch |

```yaml
# K8-001: Guaranteed QoS (RECOMMENDED for production)
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "512Mi"  # Same as request
    cpu: "500m"      # Same as request
```

#### Resource Behavior

| Resource | Type | Exceed Behavior |
|----------|------|-----------------|
| CPU | Compressible | Throttling (NOT evicted) |
| Memory | Incompressible | OOM Kill → Pod restart |

**Recommendation**: Set memory request == limit (incompressible resource).

---

### K8-008: Use HorizontalPodAutoscaler

Enable automatic scaling based on metrics.

```yaml
# K8-008: HPA configuration
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: myapp-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300  # Prevents flapping
```

**Gotchas**:
- DON'T use memory as primary metric (slow, destructive)
- Requires Metrics Server
- Custom metrics (request rate) better than CPU/memory

---

## 2. Health & Observability

### K8-002: Use Readiness and Liveness Probes

**Severity:** CRITICAL

| Probe | Purpose | Failure Action |
|-------|---------|----------------|
| **Liveness** | Is container running? | Restart container |
| **Readiness** | Ready for traffic? | Remove from Service |
| **Startup** | Has it started? | Disable liveness/readiness |

```yaml
# K8-002: GOOD - Complete probe configuration
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: app
    image: myapp:v1.0.0

    # K8-002: Startup probe for slow-starting apps
    startupProbe:
      httpGet:
        path: /healthz
        port: 8080
      failureThreshold: 30
      periodSeconds: 10

    # K8-002: Liveness probe
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 0
      periodSeconds: 10
      timeoutSeconds: 1
      failureThreshold: 3

    # K8-002: Readiness probe
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
      timeoutSeconds: 1
      failureThreshold: 3
```

**Best Practices**:
- Separate endpoints: `/healthz` (liveness), `/ready` (readiness)
- Lightweight checks (non-blocking)
- Use startup probe for slow apps

**ВАЖНО**: Kubernetes ignores Docker HEALTHCHECK! Use K8s probes.

```yaml
# K8-002: VIOLATION - No probes
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:v1.0.0
        # No probes = no self-healing, traffic to unhealthy pods
```

---

## 3. Security

### K8-003: Run as Non-Root (securityContext)

**Severity:** CRITICAL

Containers MUST run as non-root user.

```yaml
# K8-003: GOOD - Restricted Pod security context
apiVersion: v1
kind: Pod
metadata:
  name: secure-pod
spec:
  # K8-003: Pod-level security context
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault

  containers:
  - name: app
    image: myapp:v1.0.0
    # K8-003: Container-level security context
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
```

**Restricted Requirements**:
- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`
- Drop ALL capabilities
- Seccomp profile: RuntimeDefault

```yaml
# K8-003: VIOLATION - Running as root
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:v1.0.0
    # No securityContext = runs as root!
```

---

### K8-011: Apply Pod Security Standards

**Severity:** CRITICAL

| Level | Restrictions | Use Case |
|-------|-------------|----------|
| **Privileged** | None | System pods only |
| **Baseline** | Minimal | Dev environments |
| **Restricted** | Hardened | Production (REQUIRED) |

```yaml
# K8-011: Enforce at Namespace level
apiVersion: v1
kind: Namespace
metadata:
  name: production
  labels:
    # K8-011: Pod Security Standards labels
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

---

### K8-010: Use Network Policies

Implement network segmentation with default deny.

```yaml
# K8-010: Default deny all traffic
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

```yaml
# K8-010: Allow specific traffic
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-to-backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: frontend
    ports:
    - protocol: TCP
      port: 8080
```

```yaml
# K8-010: Allow DNS (Required!)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
```

**NOTE**: Requires CNI with NetworkPolicy support (Calico, Cilium, Weave Net).

---

## 4. Configuration

### K8-004: Use ConfigMaps for Config, Secrets for Secrets

Separate configuration from code. Never hardcode.

```yaml
# K8-004: ConfigMap for non-sensitive config
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  LOG_LEVEL: "info"
  FEATURE_FLAGS: "new-ui=true,beta=false"
```

```yaml
# K8-004: Secret for sensitive data
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
type: Opaque
data:
  DATABASE_PASSWORD: cGFzc3dvcmQxMjM=  # base64
```

```yaml
# K8-004: Usage in Pod
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:v1.0.0
    envFrom:
    - configMapRef:
        name: app-config
    - secretRef:
        name: app-secrets
```

**ВАЖНО**: Enable encryption at rest in etcd for Secrets!

#### K8-012: External Secrets Operator (Recommended for production)

```yaml
# K8-004: External Secrets from Vault
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vault-secret
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: app-secret
  data:
  - secretKey: password
    remoteRef:
      key: secret/data/myapp
      property: password
```

**Supported Backends**: HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager.

---

## 5. High Availability

### K8-005: Set PodDisruptionBudget for HA

Protect availability during voluntary disruptions.

```yaml
# K8-005: PodDisruptionBudget
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: myapp-pdb
spec:
  minAvailable: 2  # OR maxUnavailable: 1
  selector:
    matchLabels:
      app: myapp
```

**Best Practices**:
- Never set `minAvailable` == replica count
- Use percentage-based for flexibility
- Combine with topology spread

```yaml
# K8-005: VIOLATION - PDB equals replicas
apiVersion: policy/v1
kind: PodDisruptionBudget
spec:
  minAvailable: 3  # If replicas: 3, cluster cannot drain nodes!
```

---

### K8-006: Use Namespaces for Isolation

Organize workloads by namespace.

```yaml
# K8-006: Namespace structure
# kube-system      # K8s system
# monitoring       # Prometheus, Grafana
# logging          # EFK stack
# ingress-nginx    # Ingress controller
# cert-manager     # Cert management
# production       # Prod workloads
# staging          # Staging
# development      # Dev
```

```yaml
# K8-006: ResourceQuota per namespace
apiVersion: v1
kind: ResourceQuota
metadata:
  name: production-quota
  namespace: production
spec:
  hard:
    requests.cpu: "100"
    requests.memory: "200Gi"
    limits.cpu: "200"
    limits.memory: "400Gi"
    pods: "100"
    services: "50"
```

```yaml
# K8-006: LimitRange defaults
apiVersion: v1
kind: LimitRange
metadata:
  name: production-limits
  namespace: production
spec:
  limits:
  - type: Container
    default:
      cpu: "500m"
      memory: "512Mi"
    defaultRequest:
      cpu: "100m"
      memory: "128Mi"
```

---

## 6. Graceful Shutdown

### K8-007: Set terminationGracePeriodSeconds

Allow time for graceful shutdown.

```yaml
# K8-007: terminationGracePeriodSeconds
apiVersion: v1
kind: Pod
spec:
  terminationGracePeriodSeconds: 60  # Default is 30
  containers:
  - name: app
    image: myapp:v1.0.0
```

**Timeline**:
1. Pod receives SIGTERM
2. Removed from Service endpoints
3. preStop hook runs (if defined)
4. Application has `terminationGracePeriodSeconds` to shutdown
5. SIGKILL sent after grace period

---

### K8-009: Implement Graceful Shutdown

**Severity:** CRITICAL

Application MUST handle SIGTERM properly.

```yaml
# K8-009: preStop hook for graceful shutdown
apiVersion: v1
kind: Pod
spec:
  terminationGracePeriodSeconds: 60
  containers:
  - name: app
    image: myapp:v1.0.0
    lifecycle:
      preStop:
        exec:
          command: ["/bin/sh", "-c", "sleep 5"]  # Allow LB to update
```

**Go Implementation**:
```go
// K8-009: Graceful shutdown in Go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()

server := &http.Server{Addr: ":8080"}
go server.ListenAndServe()

<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```

---

## 7. Deployment Strategies

### K8-013: Rolling Update (Default)

```yaml
# K8-001, K8-002, K8-003, K8-009: Complete Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 4
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1        # Max pods above desired
      maxUnavailable: 0  # Zero downtime
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
        version: v1.2.0
    spec:
      terminationGracePeriodSeconds: 60
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
      - name: app
        image: myapp:v1.2.0
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]
```

**Rollback**:
```bash
kubectl rollout undo deployment/myapp
kubectl rollout undo deployment/myapp --to-revision=2
```

### K8-014: Blue-Green Deployment

```yaml
# Switch via Service selector
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
    version: green  # Switch to 'blue' for rollback
  ports:
  - port: 80
    targetPort: 8080
```

### K8-015: Canary Deployment (Argo Rollouts)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: myapp-rollout
spec:
  replicas: 10
  strategy:
    canary:
      steps:
      - setWeight: 10   # 10% traffic
      - pause: {duration: 1h}
      - setWeight: 50   # 50% traffic
      - pause: {duration: 1h}
      - setWeight: 100  # Full rollout
```

---

## 8. K8-016: Common Anti-Patterns

| Anti-Pattern | Rule | Problem | Solution |
|--------------|------|---------|----------|
| Using `latest` tag | - | Unpredictable | Semantic versioning |
| No resource limits | K8-001 | Resource contention, OOM | Set requests & limits |
| Naked Pods | - | No rescheduling | Use Deployments |
| Hardcoded config | K8-004 | Inflexible | ConfigMaps/Secrets |
| Data in Pods | - | Data loss | PersistentVolumes |
| Missing probes | K8-002 | No self-healing | Implement all probes |
| No PDB | K8-005 | Downtime during maintenance | Create PDBs |
| Running as root | K8-003 | Security risk | `runAsNonRoot: true` |
| Single replica | - | No HA | Min 2+ replicas |
| No graceful shutdown | K8-009 | Connection drops | Handle SIGTERM |

---

## 9. Quick Template

### K8-017: Production-Ready Deployment

```yaml
# K8-001, K8-002, K8-003, K8-005, K8-007, K8-009: Full example
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    app.kubernetes.io/name: myapp
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      # K8-007: Grace period
      terminationGracePeriodSeconds: 60

      # K8-003: Security context
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault

      containers:
      - name: app
        image: myapp:v1.0.0
        ports:
        - containerPort: 8080

        # K8-001: Resources
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"

        # K8-002: Probes
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          periodSeconds: 5

        # K8-003: Container security
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]

        # K8-009: Graceful shutdown hook
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 5"]
---
# K8-005: PodDisruptionBudget
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: myapp-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: myapp
```

---

## References

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [Argo Rollouts](https://argoproj.github.io/rollouts/) - Progressive delivery
- [External Secrets Operator](https://external-secrets.io/) - Secret management
