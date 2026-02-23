# Production Deployment Security Guide

This guide documents the required security configuration for deploying the payments API to production.

## Table of Contents
- [Pre-Deployment Security Checklist](#pre-deployment-security-checklist)
- [Required Configuration](#required-configuration)
- [Secrets Management](#secrets-management)
- [TLS/HTTPS Setup](#tlshttps-setup)
- [Startup Validation](#startup-validation)
- [Monitoring & Alerting](#monitoring--alerting)

---

## Pre-Deployment Security Checklist

Before deploying to production, verify ALL items are complete:

### Critical (Application will FAIL to start if missing)
- [ ] `PAYMENTS_DATABASE_PASSWORD` set from secrets manager
- [ ] `PAYMENTS_AUTH_JWT_SECRET` set (32+ characters)
- [ ] `PAYMENTS_SERVER_TLS_ENABLED=true`
- [ ] Valid TLS certificates configured (`cert_file`, `key_file`)
- [ ] `ENV=production` or `ENV=prod` environment variable set

### High Priority (Required for security)
- [ ] Database SSL/TLS enabled (`ssl_mode=require`)
- [ ] Redis password set (if Redis auth enabled)
- [ ] CORS origins whitelisted (no wildcards like `*`)
- [ ] TLS 1.3 minimum version (`tls.min_version=1.3`)
- [ ] Strong JWT secret (48+ bytes, generated with `openssl rand -base64 48`)
- [ ] All secrets rotated from development/staging

### Recommended
- [ ] Monitoring alerts configured
- [ ] Log aggregation enabled
- [ ] Rate limiting tuned for your traffic
- [ ] Database connection pool sized appropriately
- [ ] Health check endpoints tested
- [ ] Backup and disaster recovery plan in place

---

## Required Configuration

### Minimum Production Configuration

```bash
# Environment
ENV=production

# Server
PAYMENTS_SERVER_PORT=443
PAYMENTS_SERVER_TLS_ENABLED=true
PAYMENTS_SERVER_TLS_CERT_FILE=/etc/payments/certs/cert.pem
PAYMENTS_SERVER_TLS_KEY_FILE=/etc/payments/certs/key.pem
PAYMENTS_SERVER_TLS_MIN_VERSION=1.3

# Database
PAYMENTS_DATABASE_HOST=prod-db.internal
PAYMENTS_DATABASE_PASSWORD=<FROM_SECRETS_MANAGER>
PAYMENTS_DATABASE_SSL_MODE=require

# Redis
PAYMENTS_REDIS_HOST=prod-redis.internal
PAYMENTS_REDIS_PASSWORD=<FROM_SECRETS_MANAGER>

# Authentication
PAYMENTS_AUTH_JWT_SECRET=<FROM_SECRETS_MANAGER>

# CORS (NO WILDCARDS)
PAYMENTS_SERVER_CORS_ALLOWED_ORIGINS=https://app.example.com
PAYMENTS_SERVER_CORS_ALLOW_CREDENTIALS=true
```

---

## Secrets Management

### AWS Deployment

**Using AWS Secrets Manager:**

```bash
# Store secrets
aws secretsmanager create-secret \
  --name payments/prod/database-password \
  --secret-string "$(openssl rand -base64 32)"

aws secretsmanager create-secret \
  --name payments/prod/jwt-secret \
  --secret-string "$(openssl rand -base64 48)"

# Retrieve at runtime (in startup script)
export PAYMENTS_DATABASE_PASSWORD=$(aws secretsmanager get-secret-value \
  --secret-id payments/prod/database-password \
  --query SecretString \
  --output text)

export PAYMENTS_AUTH_JWT_SECRET=$(aws secretsmanager get-secret-value \
  --secret-id payments/prod/jwt-secret \
  --query SecretString \
  --output text)
```

**Using AWS Parameter Store:**

```bash
# Store secrets
aws ssm put-parameter \
  --name /payments/prod/database-password \
  --value "$(openssl rand -base64 32)" \
  --type SecureString

aws ssm put-parameter \
  --name /payments/prod/jwt-secret \
  --value "$(openssl rand -base64 48)" \
  --type SecureString

# Retrieve at runtime
export PAYMENTS_DATABASE_PASSWORD=$(aws ssm get-parameter \
  --name /payments/prod/database-password \
  --with-decryption \
  --query Parameter.Value \
  --output text)
```

### Kubernetes Deployment

**Using Kubernetes Secrets:**

```bash
# Create secrets
kubectl create secret generic payments-db-password \
  --from-literal=password="$(openssl rand -base64 32)"

kubectl create secret generic payments-jwt-secret \
  --from-literal=secret="$(openssl rand -base64 48)"
```

**Deployment manifest:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
spec:
  template:
    spec:
      containers:
      - name: api
        image: payments-api:latest
        env:
        - name: ENV
          value: "production"
        - name: PAYMENTS_DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: payments-db-password
              key: password
        - name: PAYMENTS_AUTH_JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: payments-jwt-secret
              key: secret
        - name: PAYMENTS_SERVER_TLS_ENABLED
          value: "true"
        volumeMounts:
        - name: tls-certs
          mountPath: /etc/payments/certs
          readOnly: true
      volumes:
      - name: tls-certs
        secret:
          secretName: payments-tls-certs
```

**Using External Secrets Operator (Recommended):**

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: payments-secrets
spec:
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: payments-secrets
  data:
  - secretKey: database-password
    remoteRef:
      key: payments/prod/database-password
  - secretKey: jwt-secret
    remoteRef:
      key: payments/prod/jwt-secret
```

### Docker Compose Deployment

```yaml
version: '3.8'
services:
  api:
    image: payments-api:latest
    environment:
      ENV: production
      PAYMENTS_SERVER_TLS_ENABLED: "true"
      PAYMENTS_SERVER_TLS_CERT_FILE: /run/secrets/tls_cert
      PAYMENTS_SERVER_TLS_KEY_FILE: /run/secrets/tls_key
    secrets:
      - db_password
      - jwt_secret
      - tls_cert
      - tls_key
    environment:
      PAYMENTS_DATABASE_PASSWORD_FILE: /run/secrets/db_password
      PAYMENTS_AUTH_JWT_SECRET_FILE: /run/secrets/jwt_secret

secrets:
  db_password:
    external: true
  jwt_secret:
    external: true
  tls_cert:
    file: ./certs/cert.pem
  tls_key:
    file: ./certs/key.pem
```

---

## TLS/HTTPS Setup

### Certificate Acquisition

**Option 1: Let's Encrypt (Recommended for public domains)**

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificate
sudo certbot certonly --standalone -d api.example.com

# Certificates will be at:
# /etc/letsencrypt/live/api.example.com/fullchain.pem
# /etc/letsencrypt/live/api.example.com/privkey.pem
```

**Option 2: AWS Certificate Manager + Load Balancer**

Use ACM for certificate management and terminate TLS at the load balancer. Configure the load balancer to:
- Listen on port 443 (HTTPS)
- Use TLS 1.3 minimum
- Forward to application on port 8080 (HTTP backend OK if in VPC)

**Option 3: Self-Signed (Development/Internal only)**

```bash
./scripts/generate-dev-certs.sh
```

**⚠️ WARNING**: Never use self-signed certificates for public production APIs.

### Certificate Permissions

```bash
# Set correct permissions
chmod 644 /path/to/cert.pem
chmod 600 /path/to/key.pem
chown payments:payments /path/to/*.pem
```

### Certificate Rotation

Automate certificate renewal:

```bash
# Cron job for Let's Encrypt renewal
0 0 1 * * certbot renew --quiet && systemctl reload payments-api
```

---

## Startup Validation

The application automatically validates configuration on startup when `ENV=production`.

### Validation Checks

The following checks will **fail startup** if not met:

1. **Database Password Required**
   ```
   Error: database.password required in production
   ```

2. **JWT Secret Required**
   ```
   Error: auth.jwt_secret required in production
   ```

3. **JWT Secret Length**
   ```
   Error: auth.jwt_secret must be at least 32 characters
   ```

4. **TLS Enabled**
   ```
   Error: server.tls.enabled must be true in production
   ```

5. **TLS Certificate Files Exist**
   ```
   Error: server.tls.cert_file not found: /path/to/cert.pem
   Error: server.tls.key_file not found: /path/to/key.pem
   ```

### Testing Production Configuration

```bash
# Dry-run validation (won't start server)
ENV=production \
PAYMENTS_DATABASE_PASSWORD=test \
PAYMENTS_AUTH_JWT_SECRET=$(openssl rand -base64 48) \
PAYMENTS_SERVER_TLS_ENABLED=true \
PAYMENTS_SERVER_TLS_CERT_FILE=./certs/cert.pem \
PAYMENTS_SERVER_TLS_KEY_FILE=./certs/key.pem \
./bin/api --validate-only
```

### Expected Startup Logs

Successful production startup:

```
{"level":"info","time":"2025-01-15T10:00:00Z","message":"Starting HTTPS server","addr":":443","tls_version":"1.3"}
{"level":"info","time":"2025-01-15T10:00:00Z","message":"All services initialized"}
```

Failed startup (missing config):

```
{"level":"fatal","time":"2025-01-15T10:00:00Z","error":"invalid config: database.password required in production"}
```

---

## Monitoring & Alerting

### Health Checks

Configure health check endpoints in your load balancer/orchestrator:

- **Liveness**: `GET /health/live` - Returns 200 if application is running
- **Readiness**: `GET /health/ready` - Returns 200 if DB and Redis are reachable

**Kubernetes Example:**

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
    scheme: HTTP
  initialDelaySeconds: 10
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
    scheme: HTTP
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Metrics Endpoint

Prometheus metrics available at: `GET /internal/metrics` (requires authentication)

**Important Metrics to Monitor:**

- `payments_http_requests_total` - Request rate and status codes
- `payments_http_request_duration_seconds` - Request latency
- `payments_payment_errors_total` - Payment processing errors
- `payments_worker_messages_processed_total` - Background worker throughput

### Recommended Alerts

```yaml
# Alert if error rate > 5%
- alert: HighErrorRate
  expr: rate(payments_http_requests_total{status=~"5.."}[5m]) / rate(payments_http_requests_total[5m]) > 0.05

# Alert if p95 latency > 1s
- alert: HighLatency
  expr: histogram_quantile(0.95, rate(payments_http_request_duration_seconds_bucket[5m])) > 1.0

# Alert if payment errors spike
- alert: PaymentErrorSpike
  expr: rate(payments_payment_errors_total[5m]) > 10
```

### Security Monitoring

Monitor for:
- Repeated 401/403 responses (potential auth attacks)
- 429 responses (rate limit hits)
- Unusual traffic patterns
- Failed database connections
- Certificate expiration (alert 30 days before)

---

## Additional Security Recommendations

### 1. Network Security
- Deploy in private VPC/subnet
- Use security groups to restrict access
- Only expose HTTPS port (443) publicly
- Use AWS PrivateLink or VPC peering for internal services

### 2. Database Security
- Use IAM database authentication if available
- Enable encryption at rest
- Regular automated backups
- Point-in-time recovery enabled

### 3. Application Security
- Run as non-root user
- Use read-only file systems where possible
- Enable audit logging
- Regular security scanning of Docker images

### 4. Compliance
- PCI-DSS: Required for processing payments
- GDPR: If handling EU user data
- SOC 2: For enterprise customers
- Regular penetration testing

---

## Troubleshooting

### "Config validation failed" on startup

Check that all required environment variables are set:
```bash
env | grep PAYMENTS_
```

### "Certificate not found" error

Verify certificate files exist and are readable:
```bash
ls -la /etc/payments/certs/
```

### TLS handshake failures

Verify TLS configuration:
```bash
openssl s_client -connect api.example.com:443 -tls1_3
```

### Authentication failing

Verify JWT secret is consistent across all instances:
```bash
echo $PAYMENTS_AUTH_JWT_SECRET | wc -c  # Should be > 32
```

---

## Support

For security issues, contact: security@example.com

For deployment support: devops@example.com
