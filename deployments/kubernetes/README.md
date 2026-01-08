# TexFlow Kubernetes Deployment

This directory contains Kubernetes manifests for deploying TexFlow to a Kubernetes cluster.

## Directory Structure

```
kubernetes/
├── base/                    # Base configurations
│   ├── namespace.yaml       # Namespace definition
│   ├── configmap.yaml       # Non-sensitive configuration
│   ├── secrets.yaml         # Sensitive credentials (update for production!)
│   └── ingress.yaml         # Ingress and network policies
├── infrastructure/          # Infrastructure services
│   ├── mongodb.yaml         # MongoDB StatefulSet
│   ├── redis.yaml           # Redis Deployment
│   ├── minio.yaml           # MinIO object storage
│   └── kong.yaml            # Kong API Gateway
├── services/                # Application services
│   ├── auth-service.yaml    # Authentication service
│   ├── project-service.yaml # Project management service
│   ├── websocket-service.yaml # WebSocket service
│   ├── collaboration-service.yaml # Collaboration service
│   └── compilation-service.yaml # LaTeX compilation service
├── monitoring/              # Monitoring stack
│   ├── prometheus.yaml      # Prometheus with RBAC
│   └── grafana.yaml         # Grafana dashboards
└── kustomization.yaml       # Kustomize configuration
```

## Prerequisites

- Kubernetes cluster (v1.25+)
- kubectl configured to access your cluster
- Docker images built and pushed to a container registry
- (Optional) NGINX Ingress Controller for external access
- (Optional) cert-manager for TLS certificates

## Quick Start

### 1. Build Docker Images

```bash
# Build all service images
make docker-build

# Push to your registry (set REGISTRY environment variable)
REGISTRY=your-registry.com make docker-push
```

### 2. Update Image References

Edit `kustomization.yaml` to point to your container registry:

```yaml
images:
  - name: texflow/auth-service
    newName: your-registry.com/texflow/auth-service
    newTag: latest
  # ... repeat for other services
```

### 3. Update Secrets

**Important**: Update `base/secrets.yaml` with production-ready credentials before deploying:

```yaml
stringData:
  MONGO_ROOT_PASSWORD: "your-secure-password"
  REDIS_PASSWORD: "your-secure-password"
  JWT_SECRET: "your-jwt-secret"
  # ... other secrets
```

For production, consider using:
- Kubernetes Secrets with encryption at rest
- External secret management (HashiCorp Vault, AWS Secrets Manager)
- Sealed Secrets

### 4. Deploy to Kubernetes

```bash
# Deploy all resources
make k8s-deploy

# Or using kubectl directly
kubectl apply -k deployments/kubernetes/

# Check deployment status
make k8s-status
```

### 5. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n texflow

# Check services
kubectl get services -n texflow

# View logs
make k8s-logs SERVICE=auth-service
```

## Configuration

### ConfigMap (Non-Sensitive)

`base/configmap.yaml` contains non-sensitive configuration:

| Key | Description | Default |
|-----|-------------|---------|
| MONGO_DATABASE | MongoDB database name | texflow |
| MINIO_BUCKET | MinIO bucket name | texflow |
| MINIO_USE_SSL | Enable SSL for MinIO | false |
| COMPILATION_TIMEOUT | LaTeX compilation timeout | 60s |
| COMPILATION_MEMORY_LIMIT | Memory limit for compilation | 2GB |
| LOG_LEVEL | Logging verbosity | info |

### Secrets

`base/secrets.yaml` contains sensitive credentials. Update these for production:

| Key | Description |
|-----|-------------|
| MONGO_ROOT_USERNAME | MongoDB admin username |
| MONGO_ROOT_PASSWORD | MongoDB admin password |
| REDIS_PASSWORD | Redis password |
| MINIO_ACCESS_KEY | MinIO access key |
| MINIO_SECRET_KEY | MinIO secret key |
| JWT_SECRET | JWT signing secret |

## Resource Allocation

Each service has defined resource requests and limits:

| Service | Memory Request | Memory Limit | CPU Request | CPU Limit |
|---------|---------------|--------------|-------------|-----------|
| Auth | 128Mi | 256Mi | 100m | 250m |
| Project | 128Mi | 256Mi | 100m | 250m |
| WebSocket | 128Mi | 256Mi | 100m | 250m |
| Collaboration | 128Mi | 256Mi | 100m | 250m |
| Compilation | 512Mi | 2Gi | 250m | 1000m |
| MongoDB | 512Mi | 1Gi | 250m | 500m |
| Redis | 128Mi | 256Mi | 100m | 200m |
| MinIO | 256Mi | 512Mi | 100m | 250m |

## Auto-Scaling

Each backend service has HorizontalPodAutoscaler configured:

- **Min Replicas**: 2
- **Max Replicas**: 10
- **CPU Target**: 70% utilization
- **Memory Target**: 80% utilization

## Storage

Persistent storage is configured for:

| Component | Storage Size | Access Mode |
|-----------|-------------|-------------|
| MongoDB | 10Gi | ReadWriteOnce |
| Redis | 5Gi | ReadWriteOnce |
| MinIO | 20Gi | ReadWriteOnce |
| Prometheus | 10Gi | ReadWriteOnce |
| Grafana | 5Gi | ReadWriteOnce |
| Kong DB | 5Gi | ReadWriteOnce |

## Ingress Configuration

The default ingress is configured for `texflow.local`. Update for your domain:

```yaml
spec:
  rules:
    - host: your-domain.com
      http:
        paths:
          - path: /api
            backend:
              service:
                name: kong
                port:
                  number: 8000
```

### TLS Configuration

Uncomment the TLS section in `base/ingress.yaml` for HTTPS:

```yaml
spec:
  tls:
    - hosts:
        - your-domain.com
      secretName: texflow-tls
```

## Monitoring

### Prometheus

Prometheus is configured to scrape metrics from all services. Access Prometheus:

```bash
kubectl port-forward -n texflow svc/prometheus 9090:9090
```

### Grafana

Grafana is pre-configured with Prometheus as a datasource. Access Grafana:

```bash
kubectl port-forward -n texflow svc/grafana 3000:3000
```

Default credentials: `admin/admin`

## Management Commands

```bash
# Deploy to Kubernetes
make k8s-deploy

# Delete from Kubernetes
make k8s-delete

# View deployment status
make k8s-status

# View service logs
make k8s-logs SERVICE=auth-service

# Restart a service
make k8s-restart SERVICE=auth-service

# Scale a service
make k8s-scale SERVICE=auth-service REPLICAS=5

# Port forward Kong
make k8s-port-forward

# Describe a resource
make k8s-describe RESOURCE=pod/auth-service-xxx
```

## Production Considerations

### Security

1. **Secrets Management**: Use a proper secrets management solution
2. **Network Policies**: Network policies are included to restrict traffic
3. **RBAC**: Configure appropriate RBAC for service accounts
4. **TLS**: Enable TLS for all external traffic

### High Availability

1. **Multiple Replicas**: Services default to 2 replicas
2. **Pod Disruption Budgets**: Consider adding PDBs
3. **Anti-Affinity Rules**: Consider adding pod anti-affinity

### Monitoring

1. **Alerts**: Configure Prometheus alerting rules
2. **Dashboards**: Create Grafana dashboards for your metrics
3. **Logging**: Consider centralized logging (ELK, Loki)

### Backup

1. **MongoDB**: Implement regular backups
2. **MinIO**: Configure backup for object storage
3. **Redis**: Configure RDB/AOF persistence

## Troubleshooting

### Pods not starting

```bash
# Check pod events
kubectl describe pod -n texflow <pod-name>

# Check logs
kubectl logs -n texflow <pod-name>
```

### Database connection issues

```bash
# Check if MongoDB is ready
kubectl get pods -n texflow -l app.kubernetes.io/name=mongodb

# Check MongoDB logs
kubectl logs -n texflow -l app.kubernetes.io/name=mongodb
```

### Service discovery issues

```bash
# Check services
kubectl get services -n texflow

# Test DNS resolution from a pod
kubectl exec -it -n texflow <pod-name> -- nslookup mongodb
```

## Customization with Kustomize

You can create overlays for different environments:

```
kubernetes/
├── base/
├── overlays/
│   ├── development/
│   │   └── kustomization.yaml
│   ├── staging/
│   │   └── kustomization.yaml
│   └── production/
│       └── kustomization.yaml
```

Example overlay `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../
patches:
  - patch.yaml
```

## Support

For issues and questions, please open an issue on GitHub.
