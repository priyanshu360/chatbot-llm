KIND_CLUSTER ?= kind
NAMESPACE ?= llm-logger
CHAT_IMG ?= chatbot-llm/chat-api
INGEST_IMG ?= chatbot-llm/ingestion-api
FRONTEND_IMG ?= chatbot-llm/frontend

.PHONY: all build build-chat build-ingestion build-frontend kind-load \
        kind-load-all deploy redeploy kind-create docker-up docker-down \
        e2e test grafana-port-forward help

all: kind-load-all deploy

# --- Docker builds ---

build-chat:
	docker build -f infra/docker/Dockerfile.chat-api -t $(CHAT_IMG):latest .

build-ingestion:
	docker build -f infra/docker/Dockerfile.ingestion-api -t $(INGEST_IMG):latest .

build-frontend:
	docker build -f infra/docker/Dockerfile.frontend -t $(FRONTEND_IMG):latest .

build: build-chat build-ingestion build-frontend

# --- KinD image loading ---

kind-load-chat:
	kind load docker-image $(CHAT_IMG):latest --name $(KIND_CLUSTER)

kind-load-ingestion:
	kind load docker-image $(INGEST_IMG):latest --name $(KIND_CLUSTER)

kind-load-frontend:
	kind load docker-image $(FRONTEND_IMG):latest --name $(KIND_CLUSTER)

kind-load-all: kind-load-chat kind-load-ingestion kind-load-frontend

kind-load: kind-load-all

# --- KinD cluster ---

kind-create:
	kind create cluster --name $(KIND_CLUSTER)

kind-delete:
	kind delete cluster --name $(KIND_CLUSTER)

# --- K8s deploy ---

deploy:
	kubectl apply -k infra/k8s/

redeploy: build kind-load-all
	kubectl rollout restart deployment -n $(NAMESPACE) -l app
	kubectl wait --namespace $(NAMESPACE) --for=condition=ready pod -l app --timeout=120s

# --- Docker compose (local dev) ---

docker-up:
	docker compose -f infra/docker-compose.yml --env-file=.env up -d

docker-down:
	docker compose -f infra/docker-compose.yml down

docker-up-build:
	docker compose -f infra/docker-compose.yml --env-file=.env up -d --build

# --- Grafana ---

grafana-port-forward:
	kubectl port-forward -n $(NAMESPACE) svc/grafana 3000:3000

# --- E2E ---

e2e:
	@echo "=== Health ===" && curl -s localhost:4000/api/health && \
	echo && \
	echo "=== Providers ===" && curl -s localhost:4000/api/providers

test:
	cd services/chat-api && go test ./...
	cd services/ingestion-api && go test ./...

# --- Help ---

help:
	@echo "Targets:"
	@echo "  build              Build all Docker images"
	@echo "  build-chat         Build chat-api image"
	@echo "  build-ingestion    Build ingestion-api image"
	@echo "  build-frontend     Build frontend image"
	@echo "  kind-load          Load all images into KinD"
	@echo "  kind-create        Create KinD cluster"
	@echo "  kind-delete        Delete KinD cluster"
	@echo "  deploy             Apply kustomize to K8s"
	@echo "  redeploy           Build + kind-load + rollout restart"
	@echo "  docker-up          Start local docker-compose"
	@echo "  docker-down        Stop docker-compose"
	@echo "  docker-up-build    Rebuild and start docker-compose"
	@echo "  e2e                Quick health + providers check"
	@echo "  test               Run Go unit tests"
	@echo "  grafana-port-forward  Port-forward Grafana to localhost:3000"
	@echo ""
	@echo "Variables:"
	@echo "  KIND_CLUSTER=$(KIND_CLUSTER)    NAMESPACE=$(NAMESPACE)"
