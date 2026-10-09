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

# 都営以外の事業者のデータ（backend/assets/loder.go の extraFiles と合わせる）と、S3 での置き場所
EXTRA_FILES := railway.json station.json train_type.json station_timetable.gob train_timetable.gob destination_station.json
EXTRA_PREFIX := assets-extra

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

# ファイル名にハッシュが付く assets/ は長期キャッシュし、index.html などは毎回再検証させる。
# Cache-Control を確実に付けるため cp で全件上げ直し、最後の sync で不要になったファイルを消す
frontend-upload:
	aws s3 cp \
		$(FRONTEND_DIR)/dist/assets/ \
		s3://$(FRONTEND_BUCKET)/assets/ \
		--recursive \
		--cache-control "public, max-age=31536000, immutable"
	aws s3 cp \
		$(FRONTEND_DIR)/dist/ \
		s3://$(FRONTEND_BUCKET) \
		--recursive \
		--exclude "assets/*" \
		--cache-control "no-cache"
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

backend-generate:
	cd $(BACKEND_DIR) && go generate ./assets

# assets/extra（都営以外の事業者のデータ）は go:embed で埋め込む。ライセンス上リポジトリに置けないので、
# 非公開の S3（Lambda アーティファクト用のバケット）に置き、どの版を使うかを assets/extra.version に書いてコミットする。
# 本番のバイナリが都営だけにならないよう、ビルドの前にすべてそろっているかを確かめる（CI のテストはデータ無しで動く）
backend-build:
	cd $(BACKEND_DIR) && \
	for f in $(EXTRA_FILES); do \
		test -f assets/extra/$$f || { echo "assets/extra/$$f is missing: run make backend-extra-download" >&2; exit 1; }; \
	done && \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
	go build -o $(LAMBDA_BINARY) ./cmd/api

# 手元の assets/extra（scripts/update_assets.sh で作ったもの）を S3 に置き、assets/extra.version をその版にする。
# 版は「取得日-中身のハッシュ」。同じ中身なら同じ版になる。実行後に assets/extra.version をコミットする
backend-extra-upload:
	cd $(BACKEND_DIR) && \
	for f in $(EXTRA_FILES); do test -f assets/extra/$$f || { echo "assets/extra/$$f is missing" >&2; exit 1; }; done && \
	hash=$$(cd assets/extra && cat $(EXTRA_FILES) | sha256sum | cut -c1-12) && \
	version=$$(date -r assets/extra/station.json +%Y-%m-%d)-$$hash && \
	for f in $(EXTRA_FILES); do \
		aws s3 cp --only-show-errors assets/extra/$$f s3://$(LAMBDA_ARTIFACT_BUCKET)/$(EXTRA_PREFIX)/$$version/$$f || exit 1; \
	done && \
	echo $$version > assets/extra.version && \
	echo "uploaded $$version (commit backend/assets/extra.version)"

# assets/extra.version の版を S3 から assets/extra に取ってくる（手元の assets/extra は上書きされる）。
# 中身のハッシュが版と合わなければ失敗する
backend-extra-download:
	cd $(BACKEND_DIR) && \
	version=$$(cat assets/extra.version) && \
	for f in $(EXTRA_FILES); do \
		aws s3 cp --only-show-errors s3://$(LAMBDA_ARTIFACT_BUCKET)/$(EXTRA_PREFIX)/$$version/$$f assets/extra/$$f || exit 1; \
	done && \
	hash=$$(cd assets/extra && cat $(EXTRA_FILES) | sha256sum | cut -c1-12) && \
	test "$${version##*-}" = "$$hash" || { echo "assets/extra does not match $$version" >&2; exit 1; }

backend-package:
	cd $(BACKEND_DIR) && \
	rm -f $(LAMBDA_ZIP) && \
	zip $(LAMBDA_ZIP) $(LAMBDA_BINARY)

backend-upload:
	aws s3 cp \
		$(BACKEND_DIR)/$(LAMBDA_ZIP) \
		s3://$(LAMBDA_ARTIFACT_BUCKET)/$(LAMBDA_ARTIFACT_KEY)

backend-deploy:
	$(MAKE) backend-extra-download
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