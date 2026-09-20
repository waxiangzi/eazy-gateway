.PHONY: build build-web run dev docker-build clean

BINARY_NAME=eazy-gateway
BUILD_DIR=build

build-web:
	cd web && npm run build

build: build-web
	rm -rf cmd/eazy-gateway/dist
	cp -r web/dist cmd/eazy-gateway/dist
	go build -tags embed -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/eazy-gateway/
	rm -rf cmd/eazy-gateway/dist

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

dev:
	@echo "Use 'just dev' for hot-reload development (requires just)"
	@exit 1

docker-build:
	docker build -t $(BINARY_NAME) .

clean:
	rm -rf $(BUILD_DIR)/
	rm -rf web/node_modules/
	rm -rf web/dist/
	rm -rf cmd/eazy-gateway/dist/
	rm -rf tmp/
	rm -f .air.toml
