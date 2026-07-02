# ============================
# Variables
# ============================

FRONTEND_DIR := frontend
BACKEND_DIR := backend

BOOTSTRAP_DIR := infra/bootstrap
INFRA_DIR := infra/main
TF_ENV_FILE := ../env/dev.tfvars

LAMBDA_BINARY := bootstrap
LAMBDA_ZIP := lambda.zip

# ============================
# Docker
# ============================

up:
	docker compose up --build

down:
	docker compose down

restart:
	docker compose down
	docker compose up --build

logs:
	docker compose logs -f

# ============================
# Docker Shell
# ============================

frontend-shell:
	docker exec -it train-status-frontend sh

backend-shell:
	docker exec -it train-status-backend sh


# ============================
# Frontend
# ============================

frontend-dev:
	cd $(FRONTEND_DIR) && npm run dev

frontend-build:
	cd $(FRONTEND_DIR) && npm run build

frontend-lint:
	cd $(FRONTEND_DIR) && npm run lint

# ============================
# Backend
# ============================

backend-run:
	cd $(BACKEND_DIR) && go run ./cmd/api

backend-test:
	cd $(BACKEND_DIR) && go test ./...

backend-vet:
	cd $(BACKEND_DIR) && go vet ./...

backend-build:
	cd $(BACKEND_DIR) && \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
	go build -o $(LAMBDA_BINARY) ./cmd/api

backend-package:
	cd $(BACKEND_DIR) && \
	rm -f $(LAMBDA_ZIP) && \
	zip $(LAMBDA_ZIP) $(LAMBDA_BINARY)

backend-clean:
	rm -f $(BACKEND_DIR)/$(LAMBDA_BINARY)
	rm -f $(BACKEND_DIR)/$(LAMBDA_ZIP)

# ============================
# Terraform Bootstrap
# ============================

tf-bootstrap-init:
	cd $(BOOTSTRAP_DIR) && terraform init

tf-bootstrap-fmt:
	cd $(BOOTSTRAP_DIR) && terraform fmt -recursive

tf-bootstrap-validate:
	cd $(BOOTSTRAP_DIR) && terraform validate

tf-bootstrap-plan:
	cd $(BOOTSTRAP_DIR) && terraform plan

tf-bootstrap-apply:
	cd $(BOOTSTRAP_DIR) && terraform apply -auto-approve

tf-bootstrap-destroy:
	cd $(BOOTSTRAP_DIR) && terraform destroy -auto-approve

# ============================
# Terraform Main
# ============================

tf-init:
	cd $(INFRA_DIR) && terraform init

tf-fmt:
	cd $(INFRA_DIR) && terraform fmt -recursive

tf-validate:
	cd $(INFRA_DIR) && terraform validate

tf-plan:
	cd $(INFRA_DIR) && terraform plan \
	-var-file=$(TF_ENV_FILE)

tf-apply:
	cd $(INFRA_DIR) && terraform apply \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

tf-destroy:
	cd $(INFRA_DIR) && terraform destroy \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

# ============================
# Clean
# ============================

clean: backend-clean