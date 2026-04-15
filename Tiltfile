# -*- mode: Python -*-


docker_build(
    "ghcr.io/marmotdata/marmot",
    ".",
    dockerfile="Dockerfile",
    ignore=[
        ".git",
        ".github",
        "*.md",
        "LICENSE",
        "test/",
        "web/docs/",
        "testenv/",
    ],
)


k8s_yaml("charts/marmot/crds/runs.marmotdata.io_runs.yaml")


k8s_yaml(blob("""
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              value: marmot
            - name: POSTGRES_PASSWORD
              value: marmot
            - name: POSTGRES_DB
              value: marmot
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "marmot"]
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
"""))



helm_values = encode_yaml({
    "image": {
        "repository": "ghcr.io/marmotdata/marmot",
        "pullPolicy": "Never",
    },
    "replicaCount": 1,
    "autoscaling": {"enabled": False},
    "resources": {
        "limits":   {"cpu": "1000m", "memory": "512Mi"},
        "requests": {"cpu": "100m",  "memory": "128Mi"},
    },
    "postgresql": {"enabled": False},
    "cnpg":       {"enabled": False},
    "config": {
        "server": {
            "port": 8080,
            "host": "0.0.0.0",
            "autoGenerateEncryptionKey": True,
            "allow_unencrypted": True,
        },
        "database": {
            "host": "postgres",
            "port": 5432,
            "user": "marmot",
            "name": "marmot",
            "sslmode": "disable",
            "max_conns": 20,
            "idle_conns": 5,
            "conn_lifetime": 5,
        },
        "logging": {
            "level": "debug",
            "format": "console",
        },
        "auth": {
            "anonymous": {
                "enabled": True,
                "role": "admin",
            },
        },
        "pipelines": {
            "max_workers": 5,
            "scheduler_interval": 30,
            "lease_expiry": 300,
            "claim_expiry": 30,
        },
        "operator": {
            "enabled": True,
            "namespace": "default",
        },
    },
    "operator": {
        "enabled": True,
        "replicas": 1,
        "leaderElect": False,
        "resources": {
            "limits":   {"cpu": "500m", "memory": "256Mi"},
            "requests": {"cpu": "50m",  "memory": "64Mi"},
        },
    },
    "env": {
        "MARMOT_DATABASE_PASSWORD": "marmot",
    },
})

k8s_yaml(local("helm template marmot charts/marmot -f -", stdin=helm_values))


k8s_resource("postgres", port_forwards=["5432:5432"], labels=["infra"])
k8s_resource("marmot", port_forwards=["8080:8080"], labels=["marmot"], resource_deps=["postgres"])
k8s_resource("marmot-operator", labels=["marmot"], resource_deps=["marmot"])
