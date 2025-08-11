# XPod to ECS Converter Examples

## CLI Usage

Build and run the converter:

```bash
# Build the CLI tool
go build -o bin/xpod-to-ecs ./cmd/xpod-to-ecs

# Convert a sample XPod spec to ECS task definition
./bin/xpod-to-ecs \
  -input examples/sample-xpod.yaml \
  -parameter-store-prefix "/webapp" \
  -log-region "ap-northeast-1" \
  -output task-definition.json
```

## XPod YAML Format

```yaml
family: web-application
cpu: "512"
memory: "1024"
networkMode: awsvpc
requiresCompatibilities:
  - FARGATE
executionRoleArn: arn:aws:iam::123456789012:role/ecsTaskExecutionRole
taskRoleArn: arn:aws:iam::123456789012:role/ecsTaskRole
containers:
- name: nginx
  image: nginx:1.21-alpine
  ports:
  - containerPort: 80
    protocol: TCP
  resources:
    limits:
      cpu: "256m"
      memory: "512Mi"
    requests:
      memory: "256Mi"
  env:
  - name: ENVIRONMENT
    value: production
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: database-credentials
        key: password
  - name: API_CONFIG
    valueFrom:
      configMapKeyRef:
        name: api-configuration
        key: config.json
  volumeMounts:
  - name: static-files
    mountPath: /usr/share/nginx/html
    readOnly: true
volumes:
- name: static-files
  hostPath:
    path: /opt/static-content
tags:
  Environment: production
  Team: platform
```

## Parameter Store Setup

Before running the converted task, make sure to set up Parameter Store parameters:

```bash
# Store secrets (for production namespace)
aws ssm put-parameter \
  --name "/webapp/production/secrets/database-credentials/password" \
  --value "your-db-password" \
  --type "SecureString"

# Store config maps (for production namespace)
aws ssm put-parameter \
  --name "/webapp/production/configmaps/api-configuration/config.json" \
  --value '{"api_url": "https://api.example.com"}' \
  --type "String"
```

## Deployment

Deploy the generated ECS task definition:

```bash
# Register the task definition
aws ecs register-task-definition --cli-input-json file://task-definition.json

# Create or update a service
aws ecs create-service \
  --cluster my-cluster \
  --service-name web-application \
  --task-definition web-application:1 \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-12345],securityGroups=[sg-12345],assignPublicIp=ENABLED}"
```