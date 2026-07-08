# ============================
# Variables
# ============================

FRONTEND_DIR := frontend
BACKEND_DIR := backend

BOOTSTRAP_DIR := infra/bootstrap
MAIN_DIR := infra/main

TF_ENV_FILE := ../env/dev.tfvars

LAMBDA_BINARY := bootstrap
LAMBDA_ZIP := lambda.zip

FRONTEND_BUCKET := train-status-app-dev-frontend-assets

LAMBDA_ARTIFACT_BUCKET := train-status-app-dev-lambda-artifacts
LAMBDA_ARTIFACT_KEY := lambda/bootstrap.zip

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

frontend-lint:
	cd $(FRONTEND_DIR) && npm run lint

frontend-dev:
	cd $(FRONTEND_DIR) && npm run dev

frontend-build:
	cd $(FRONTEND_DIR) && npm run build

frontend-upload:
	aws s3 sync \
		$(FRONTEND_DIR)/dist/ \
		s3://$(FRONTEND_BUCKET) \
		--delete

frontend-invalidate:
	cd $(MAIN_DIR) && \
	aws cloudfront create-invalidation \
		--distribution-id "$$(terraform output -raw cloudfront_distribution_id)" \
		--paths "/*"		

frontend-deploy:
	$(MAKE) frontend-build
	$(MAKE) frontend-upload
	$(MAKE) frontend-invalidate
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

backend-upload:
	aws s3 cp \
		$(BACKEND_DIR)/$(LAMBDA_ZIP) \
		s3://$(LAMBDA_ARTIFACT_BUCKET)/$(LAMBDA_ARTIFACT_KEY)

backend-deploy:
	$(MAKE) backend-build
	$(MAKE) backend-package
	$(MAKE) backend-upload

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
	cd $(BOOTSTRAP_DIR) && terraform plan \
	-var-file=$(TF_ENV_FILE)

tf-bootstrap-apply:
	cd $(BOOTSTRAP_DIR) && terraform apply \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

tf-bootstrap-destroy:
	cd $(BOOTSTRAP_DIR) && terraform destroy \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

# ============================
# Terraform Main
# ============================

tf-main-init:
	cd $(MAIN_DIR) && terraform init

tf-main-fmt:
	cd $(MAIN_DIR) && terraform fmt -recursive

tf-main-validate:
	cd $(MAIN_DIR) && terraform validate

tf-main-plan:
	cd $(MAIN_DIR) && terraform plan \
	-var-file=$(TF_ENV_FILE)

tf-main-apply:
	cd $(MAIN_DIR) && terraform apply \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

tf-main-destroy:
	cd $(MAIN_DIR) && terraform destroy \
	-auto-approve \
	-var-file=$(TF_ENV_FILE)

# ============================
# Clean
# ============================

clean: backend-clean