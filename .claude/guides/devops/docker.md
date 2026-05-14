# Docker Best Practices and Standards

This document contains Docker rules for DevOps tasks. Each rule has a unique identifier (DK-XXXX) for reference in code reviews and task execution.

> **Related**: For Kubernetes rules, see [kubernetes.md](./kubernetes.md)
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми Docker задачами

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| DK-001 | Use multi-stage builds for production | ✅ | [Multi-Stage Builds](#dk-001-multi-stage-builds-for-production) |
| DK-002 | Run as non-root user | ✅ | [Non-Root User](#dk-002-run-as-non-root-user) |
| DK-003 | Cache dependencies layer separately | | [Layer Caching](#dk-003-cache-dependencies-layer-separately) |
| DK-004 | Use specific image tags, not :latest | ✅ | [Image Tags](#dk-004-use-specific-image-tags-not-latest) |
| DK-005 | Minimize layers, combine RUN commands | | [Layer Optimization](#dk-005-minimize-layers-combine-run-commands) |
| DK-006 | Use .dockerignore | ✅ | [Dockerignore](#dk-006-use-dockerignore) |
| DK-007 | Set HEALTHCHECK | | [Health Checks](#dk-007-set-healthcheck) |
| DK-008 | Use CGO_ENABLED=0 for Go | | [Go Static Build](#dk-008-use-cgo_enabled0-for-go) |
| DK-009 | Use scratch or distroless for production | ✅ | [Base Image Selection](#dk-009-use-scratch-or-distroless-for-production) |
| DK-010 | Never expose secrets in ARG/ENV | ✅ | [Secret Management](#dk-010-never-expose-secrets-in-argenv) |
| DK-011 | Set resource limits | | [Resource Limits](#dk-011-set-resource-limits) |

---

## 1. Dockerfile Rules

### DK-001: Multi-Stage Builds for Production

**Severity:** CRITICAL

All production images MUST use multi-stage builds.

**Why:** Reduces image size by 50-80%, minimizes attack surface.

```dockerfile
# DK-001: Multi-stage build example
# syntax=docker/dockerfile:1

# Builder stage
FROM golang:1.24-alpine AS builder
WORKDIR /build

# DK-003: Cache dependencies separately
COPY go.mod go.sum ./
RUN go mod download

# Copy and build
COPY . .
# DK-008: Static build for scratch
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/server \
    ./cmd/server

# DK-009: Production stage with scratch
FROM scratch AS production

# Copy SSL certs for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# DK-002: Non-root user
COPY --from=builder /etc/passwd /etc/passwd
USER 65534:65534

COPY --from=builder /app/server /server

EXPOSE 8080

# DK-007: Health check
HEALTHCHECK --interval=30s --timeout=3s \
  CMD ["/server", "healthcheck"] || exit 1

ENTRYPOINT ["/server"]
```

```dockerfile
# DK-001: VIOLATION - Single stage build
FROM golang:1.24
COPY . .
RUN go build -o /app
ENTRYPOINT ["/app"]
# Image size: 800MB+ instead of <20MB
```

---

### DK-002: Run as Non-Root User

**Severity:** CRITICAL

Containers MUST run as non-root user for security.

```dockerfile
# DK-002: GOOD - Create and use non-root user
FROM python:3.11-slim

# Create user
RUN groupadd -r appgroup && \
    useradd -r -g appgroup -u 1000 appuser

WORKDIR /app

# Copy with correct ownership
COPY --chown=appuser:appuser . /app

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt

# DK-002: Switch to non-root
USER appuser

CMD ["python", "app.py"]
```

```dockerfile
# DK-002: For scratch images - use nobody
FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
USER 65534:65534  # nobody user
```

```dockerfile
# DK-002: VIOLATION - Running as root
FROM python:3.11-slim
COPY . /app
# No USER directive = runs as root!
CMD ["python", "app.py"]
```

---

### DK-003: Cache Dependencies Layer Separately

Dependencies change less frequently than source code. Separate them for better caching.

```dockerfile
# DK-003: GOOD - Dependencies cached separately
FROM golang:1.24-alpine

# 1. Base image (rarely changes)
# 2. System dependencies (rarely change)
RUN apk add --no-cache git ca-certificates

# 3. DK-003: Application dependencies (change sometimes)
COPY go.mod go.sum ./
RUN go mod download

# 4. Source code (changes often)
COPY . .

# 5. Build (runs after code changes)
RUN go build -o /app/server
```

```dockerfile
# DK-003: BuildKit cache mounts for faster builds
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    go build -o /app/server
```

```dockerfile
# DK-003: VIOLATION - No layer caching
FROM golang:1.24-alpine
COPY . .  # Every change invalidates go mod download
RUN go mod download && go build -o /app
```

---

### DK-004: Use Specific Image Tags, Not :latest

**Severity:** CRITICAL

Always use semantic versioning for reproducible builds.

```dockerfile
# DK-004: GOOD - Specific version
FROM golang:1.24.1-alpine3.19
FROM node:20.11.1-alpine3.19
FROM postgres:16.2-alpine3.19
```

```dockerfile
# DK-004: VIOLATION - Using :latest
FROM golang:latest      # Unpredictable builds
FROM postgres:latest    # May break unexpectedly
FROM node               # Defaults to :latest
```

**Exception:** Build stages can use more general tags:
```dockerfile
# DK-004: Acceptable for builder (not shipped)
FROM golang:1.24-alpine AS builder
# ...
# DK-004: Production MUST be specific
FROM gcr.io/distroless/static-debian12:nonroot
```

---

### DK-005: Minimize Layers, Combine RUN Commands

Fewer layers = smaller image, faster pulls.

```dockerfile
# DK-005: GOOD - Combined RUN commands
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git && \
    rm -rf /var/lib/apt/lists/* && \
    apt-get clean
```

```dockerfile
# DK-005: VIOLATION - Multiple layers
RUN apt-get update
RUN apt-get install -y ca-certificates
RUN apt-get install -y curl
RUN apt-get install -y git
RUN rm -rf /var/lib/apt/lists/*
# 5 layers instead of 1
```

---

### DK-006: Use .dockerignore

**Severity:** CRITICAL

Always create `.dockerignore` to exclude unnecessary files from build context.

```
# DK-006: .dockerignore example

# Version control
.git
.gitignore

# Documentation
*.md
docs/

# CI/CD
.github/
.gitlab-ci.yml

# IDE
.vscode/
.idea/

# Dependencies (rebuilt in container)
node_modules/
vendor/

# Build artifacts
dist/
build/
*.log
*.test

# Environment files (SECURITY!)
.env
.env.*
!.env.example

# Test files
*_test.go
__tests__/
coverage/
```

**Why:**
- Reduces build context size (faster builds)
- Prevents secret leakage (.env files)
- Excludes test files from production image

---

### DK-007: Set HEALTHCHECK

Enable container orchestration and self-healing.

```dockerfile
# DK-007: GOOD - Health check in Dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
```

```dockerfile
# DK-007: For Go binaries without wget
HEALTHCHECK --interval=30s --timeout=3s \
  CMD ["/app", "healthcheck"] || exit 1
```

**Note:** Kubernetes ignores Docker HEALTHCHECK - use K8s probes instead (see K8-002).

---

### DK-008: Use CGO_ENABLED=0 for Go

Required for static binaries that run in scratch/distroless.

```dockerfile
# DK-008: GOOD - Static Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/server \
    ./cmd/server
```

```dockerfile
# DK-008: VIOLATION - CGO enabled (won't work in scratch)
RUN go build -o /app/server ./cmd/server
# Error: binary requires glibc, not in scratch
```

**Flags explained:**
- `CGO_ENABLED=0`: Disable C bindings (static binary)
- `-ldflags="-s -w"`: Strip debug info (~30% smaller)

---

### DK-009: Use scratch or distroless for Production

**Severity:** CRITICAL

Minimal base images for maximum security.

| Image Type | Size | Shell | Security | Use Case |
|-----------|------|-------|----------|----------|
| **scratch** | ~0 MB | No | Highest | Go static binaries |
| **distroless** | ~2-20 MB | No* | Very High | Java, Python, Node.js |
| **alpine** | ~5 MB | Yes | High | General purpose, debugging |
| **debian:slim** | ~25 MB | Yes | Medium | Compatibility needs |

\*distroless has debug variants with shell

```dockerfile
# DK-009: GOOD - Go with scratch
FROM scratch
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

```dockerfile
# DK-009: GOOD - Node.js with distroless
FROM gcr.io/distroless/nodejs20-debian12
COPY --from=builder /build /app
WORKDIR /app
CMD ["server.js"]
```

```dockerfile
# DK-009: VIOLATION - Full image in production
FROM golang:1.24  # 800MB+ with compilers, shells, etc.
```

---

### DK-010: Never Expose Secrets in ARG/ENV

**Severity:** CRITICAL

ARG and ENV values are saved in image layers and visible in history!

```dockerfile
# DK-010: VIOLATION - Secrets in ARG/ENV
ARG DATABASE_PASSWORD  # NEVER DO THIS
ENV DB_PASS=$DATABASE_PASSWORD  # Visible in docker history!
```

```dockerfile
# DK-010: GOOD - BuildKit secrets
# syntax=docker/dockerfile:1

FROM alpine

RUN --mount=type=secret,id=db_password \
    DB_PASSWORD=$(cat /run/secrets/db_password) && \
    # Use secret here - NOT saved in layer
    configure_app --password="$DB_PASSWORD"
```

**Build command:**
```bash
# DK-010: Pass secret at build time
docker build --secret id=db_password,src=./secrets/db_password.txt .
```

```dockerfile
# DK-010: SSH keys for private repos
RUN --mount=type=ssh \
    git clone git@github.com:private/repo.git
```

```bash
# DK-010: SSH agent forwarding
docker build --ssh default .
```

---

### DK-011: Set Resource Limits

Prevent containers from consuming all host resources.

```yaml
# DK-011: Docker Compose resource limits
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
```

```bash
# DK-011: Docker run with limits
docker run \
  --memory=512m \
  --cpus=0.5 \
  --pids-limit=100 \
  myapp:v1.0.0
```

**Without limits:** Container can use all host resources → OOM kills other containers.

---

## 2. Docker Compose Patterns

### DK-012: Modern Structure (Compose v2+)

**Note:** Version field is no longer required with Compose v2.

```yaml
# DK-007: Health checks in compose
# DK-010: Secrets management
services:
  web:
    build:
      context: .
      dockerfile: Dockerfile
      target: production
    restart: unless-stopped
    depends_on:
      db:
        condition: service_healthy
    networks:
      - frontend
      - backend
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  db:
    # DK-004: Specific version
    image: postgres:16.2-alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - backend
    volumes:
      - postgres_data:/var/lib/postgresql/data
    # DK-010: Secrets, not ENV
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password

networks:
  frontend:
  backend:
    internal: true  # No external access

volumes:
  postgres_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

### DK-013: Environment Management

```yaml
services:
  app:
    env_file:
      - .env
      - .env.${ENVIRONMENT:-development}
    environment:
      - LOG_LEVEL=${LOG_LEVEL:-info}
      - SERVICE_NAME=myapp
```

**Precedence (highest to lowest):**
1. Shell environment variables
2. `--env-file` flag
3. Default `.env` file
4. `env_file` in compose
5. `environment` in compose

---

## 3. Security Standards

### DK-014: Image Scanning (Required in CI/CD)

```bash
# Trivy - recommended scanner
trivy image --severity HIGH,CRITICAL --exit-code 1 myapp:latest

# Generate SBOM
trivy image --format spdx-json -o sbom.json myapp:latest
```

```yaml
# GitHub Actions integration
- name: Scan Docker image
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.IMAGE }}
    format: 'sarif'
    exit-code: '1'
    severity: 'CRITICAL,HIGH'
```

### DK-015: Runtime Security

```bash
# DK-002, DK-011: Secure runtime
docker run \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid \
  --security-opt=no-new-privileges:true \
  --cap-drop ALL \
  --cap-add NET_BIND_SERVICE \
  --user 1000:1000 \
  --memory=512m \
  --cpus=0.5 \
  --pids-limit=100 \
  --restart=on-failure:3 \
  myapp:v1.0.0
```

### DK-016: Network Segmentation

```yaml
networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true  # No external access

services:
  nginx:
    networks: [frontend]
  app:
    networks: [frontend, backend]
  db:
    networks: [backend]  # Only accessible from app
```

---

## 4. DK-017: Common Gotchas

| Problem | Rule | Consequence | Solution |
|---------|------|-------------|----------|
| Using `:latest` tag | DK-004 | Unpredictable builds | Semantic versioning: `v1.2.3` |
| Running as root | DK-002 | Privilege escalation risk | `USER 1000:1000` |
| No `.dockerignore` | DK-006 | Large context, secret leakage | Create `.dockerignore` |
| No resource limits | DK-011 | OOM kills other containers | `--memory`, `--cpus` |
| Hardcoded secrets | DK-010 | Secrets in image layers | BuildKit secrets |
| No health checks | DK-007 | No automatic recovery | `HEALTHCHECK` or K8s probes |
| Large base images | DK-009 | More vulnerabilities | scratch, distroless, alpine |
| Single-stage build | DK-001 | Huge production image | Multi-stage builds |
| Docker socket mount | - | Root-equivalent access | **NEVER** mount `/var/run/docker.sock` |
| `--privileged` flag | - | All capabilities | Use `--cap-add` for specific caps |

---

## 5. Quick Template

### DK-018: Go Dockerfile (Production-Ready)

```dockerfile
# DK-001, DK-003, DK-008, DK-009: Complete example
# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /build

# DK-003: Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# DK-008: Static build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

# DK-009: Minimal production image
FROM scratch

# SSL certs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# DK-002: Non-root
COPY --from=builder /etc/passwd /etc/passwd
USER 65534:65534

COPY --from=builder /app /app

# DK-007: Health check
HEALTHCHECK --interval=30s CMD ["/app", "health"] || exit 1

ENTRYPOINT ["/app"]
```

---

## References

- [Docker Best Practices](https://docs.docker.com/build/building/best-practices/)
- [OWASP Docker Security](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)
- [BuildKit Secrets](https://docs.docker.com/build/building/secrets/)
- [Trivy Scanner](https://trivy.dev/)
- [Hadolint](https://github.com/hadolint/hadolint) - Dockerfile linter
