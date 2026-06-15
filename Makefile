BINARY := donetick
FRONTEND_DIR := ../donetick-frontend
DEV_DATA_DIR := ./tmp/donetick-dev
DEV_PORT := 2022

.PHONY: build run dev prod local local-backend local-frontend

build:
	go build -o $(BINARY) .

run: build
	DT_ENV=selfhosted ./$(BINARY)

prod:
	docker compose -f docker-compose.dev.yaml up --build

source:
	docker compose up

# Tab 1: make local-backend   (builds binary, snapshots prod DB, runs backend)
# Tab 2: make local-frontend  (runs Vite dev server with HMR)
# Data in $(DEV_DATA_DIR) is discarded when the backend tab exits.

local-backend:
	@mkdir -p $(DEV_DATA_DIR)/uploads ; \
	echo "Building dev binary..." && go build -o $(DEV_DATA_DIR)/$(BINARY) . ; \
	if [ -f ./data/donetick.db ]; then \
	  cp ./data/donetick.db $(DEV_DATA_DIR)/donetick.db && echo "Snapshotted prod DB → $(DEV_DATA_DIR)/donetick.db" ; \
	else \
	  echo "No prod DB found — starting with empty DB" ; \
	fi ; \
	trap 'echo "Cleaning up $(DEV_DATA_DIR)..."; rm -rf $(DEV_DATA_DIR)' EXIT ; \
	DT_ENV=selfhosted \
	DT_SQLITE_PATH=$(DEV_DATA_DIR)/donetick.db \
	DT_STORAGE_BASE_PATH=$(DEV_DATA_DIR)/uploads \
	DT_SERVER_PORT=$(DEV_PORT) \
	DT_SERVER_SERVE_FRONTEND=false \
	$(DEV_DATA_DIR)/$(BINARY)

local-frontend:
	cd $(FRONTEND_DIR) && VITE_APP_API_URL=http://localhost:$(DEV_PORT) npm run dev
