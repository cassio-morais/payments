# Deployment Architecture Guide

This document describes recommended production architectures for the payments API.

## Recommended: TLS Termination at Load Balancer

**This is the standard, production-ready approach.**

```
┌─────────────────────────────────────────────────────────────┐
│ Internet                                                     │
│   ↓ HTTPS (TLS 1.3)                                        │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ Load Balancer / API Gateway                                 │
│                                                              │
│ • AWS ALB/NLB (with ACM certificates)                       │
│ • nginx/Traefik (with Let's Encrypt)                        │
│ • API Gateway (Kong, Apigee, AWS API Gateway)               │
│                                                              │
│ Handles:                                                     │
│ ✓ TLS termination                                           │
│ ✓ Certificate management (auto-renewal)                     │
│ ✓ Rate limiting (optional)                                  │
│ ✓ DDoS protection                                           │
│ ✓ Health checks                                             │
└─────────────────────────────────────────────────────────────┘
         │
         ▼ HTTP (within VPC - secure)
┌─────────────────────────────────────────────────────────────┐
│ Private VPC / Kubernetes Cluster                            │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ API Instance │  │ API Instance │  │ API Instance │     │
│  │   :8080      │  │   :8080      │  │   :8080      │     │
│  │              │  │              │  │              │     │
│  │ TLS: false   │  │ TLS: false   │  │ TLS: false   │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Why This Approach?

**Pros:**
- ✅ **Simpler operations** - No certificate management in app
- ✅ **Auto-renewal** - Let's Encrypt, ACM handle rotation
- ✅ **Better performance** - Hardware TLS acceleration
- ✅ **Load balancing** - Built-in with TLS termination
- ✅ **DDoS protection** - Cloudflare, AWS Shield
- ✅ **Centralized control** - One place for TLS config
- ✅ **Zero downtime** - Certificate updates without app restart

**Configuration:**
```bash
# Application runs on HTTP in private VPC
PAYMENTS_SERVER_PORT=8080
PAYMENTS_SERVER_TLS_ENABLED=false
```

---

## AWS Example

### Using Application Load Balancer (ALB)

```yaml
# terraform/alb.tf
resource "aws_lb" "payments_api" {
  name               = "payments-api-lb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets           = var.public_subnet_ids

  enable_http2 = true
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.payments_api.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"  # TLS 1.3
  certificate_arn   = aws_acm_certificate.payments_api.arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

resource "aws_lb_target_group" "api" {
  name     = "payments-api-tg"
  port     = 8080  # App runs on HTTP
  protocol = "HTTP"
  vpc_id   = var.vpc_id

  health_check {
    path                = "/health/ready"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
  }
}

resource "aws_acm_certificate" "payments_api" {
  domain_name       = "api.example.com"
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}
```

### Security Group Configuration

```yaml
# ALB Security Group - Public facing
resource "aws_security_group" "alb" {
  vpc_id = var.vpc_id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]  # Public HTTPS
  }

  egress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]  # Only to VPC
  }
}

# API Security Group - Private
resource "aws_security_group" "api" {
  vpc_id = var.vpc_id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]  # Only from ALB
  }
}
```

---

## Kubernetes Example

### Using Ingress Controller (nginx)

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: payments-api
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-protocols: "TLSv1.3"
spec:
  tls:
  - hosts:
    - api.example.com
    secretName: payments-api-tls  # Managed by cert-manager
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: payments-api
            port:
              number: 8080  # App runs on HTTP

---
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: payments-api
spec:
  selector:
    app: payments-api
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP  # Not exposed outside cluster

---
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: api
        image: payments-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: PAYMENTS_SERVER_PORT
          value: "8080"
        - name: PAYMENTS_SERVER_TLS_ENABLED
          value: "false"  # TLS handled by ingress
```

---

## Alternative: App-Level TLS

**Use this ONLY if:**
- Direct internet exposure (unusual)
- Compliance requires end-to-end encryption
- Zero-trust architecture with mTLS
- No load balancer available

```
┌─────────────────────────────────────────────────────────────┐
│ Internet                                                     │
│   ↓ HTTPS (TLS 1.3)                                        │
└─────────────────────────────────────────────────────────────┘
         │
         ▼ HTTPS (end-to-end encryption)
┌─────────────────────────────────────────────────────────────┐
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ API Instance │  │ API Instance │  │ API Instance │     │
│  │   :443       │  │   :443       │  │   :443       │     │
│  │              │  │              │  │              │     │
│  │ TLS: true    │  │ TLS: true    │  │ TLS: true    │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
```bash
PAYMENTS_SERVER_PORT=443
PAYMENTS_SERVER_TLS_ENABLED=true
PAYMENTS_SERVER_TLS_CERT_FILE=/etc/payments/certs/cert.pem
PAYMENTS_SERVER_TLS_KEY_FILE=/etc/payments/certs/key.pem
```

**Drawbacks:**
- ❌ Certificate management burden (manual renewal)
- ❌ Need to restart app for cert updates
- ❌ More complex deployment
- ❌ No hardware TLS acceleration
- ❌ Each instance needs certificates

---

## Service Mesh (Advanced)

For microservices with mTLS requirements:

```
┌─────────────────────────────────────────────────────────────┐
│ Istio / Linkerd Service Mesh                                │
│                                                              │
│ • Automatic mTLS between services                           │
│ • Certificate rotation (every 24h)                          │
│ • Zero-trust network                                        │
│                                                              │
│  ┌──────────────┐  mTLS  ┌──────────────┐  mTLS           │
│  │   Payments   │◄──────►│   Accounts   │◄──────►...      │
│  │   Service    │        │   Service    │                  │
│  └──────────────┘        └──────────────┘                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
```yaml
# App doesn't handle TLS - sidecar proxy does
PAYMENTS_SERVER_PORT=8080
PAYMENTS_SERVER_TLS_ENABLED=false
```

---

## nginx Reverse Proxy Example

### Simple nginx Configuration

```nginx
# /etc/nginx/sites-available/payments-api
upstream payments_api {
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    # TLS Configuration
    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;
    ssl_protocols TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers 'TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256:TLS_CHACHA20_POLY1305_SHA256';

    # Security Headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;

    # Proxy to application
    location / {
        proxy_pass http://payments_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$server_name$request_uri;
}
```

### Let's Encrypt Auto-Renewal

```bash
# Install certbot
sudo apt-get install certbot python3-certbot-nginx

# Generate certificate
sudo certbot --nginx -d api.example.com

# Auto-renewal (already configured by certbot)
sudo systemctl status certbot.timer
```

---

## Decision Matrix

| Scenario | Recommended Approach | App TLS Setting |
|----------|---------------------|-----------------|
| AWS deployment | ALB with ACM | `TLS_ENABLED=false` |
| Kubernetes | Ingress + cert-manager | `TLS_ENABLED=false` |
| Docker Compose (small) | nginx/Traefik proxy | `TLS_ENABLED=false` |
| Service mesh (Istio) | Sidecar mTLS | `TLS_ENABLED=false` |
| Single server | nginx reverse proxy | `TLS_ENABLED=false` |
| Direct exposure (rare) | App-level TLS | `TLS_ENABLED=true` |
| Zero-trust required | Service mesh mTLS | `TLS_ENABLED=false` |

---

## Summary

**Default Recommendation:** Let your load balancer/gateway handle TLS.

**Application Configuration:**
```bash
# Production (behind load balancer)
PAYMENTS_SERVER_PORT=8080
PAYMENTS_SERVER_TLS_ENABLED=false

# Development (local testing)
PAYMENTS_SERVER_PORT=8080
PAYMENTS_SERVER_TLS_ENABLED=false
```

**Infrastructure handles:**
- TLS termination
- Certificate management
- Auto-renewal
- Load balancing
- DDoS protection
- Rate limiting (optional)

**Application focuses on:**
- Business logic
- Authentication (JWT)
- Authorization
- Data validation
- Database operations

This separation of concerns is the industry standard for modern cloud-native applications.
