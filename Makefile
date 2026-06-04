.PHONY: build build-web run docker-build clean

BINARY_NAME=tun-console
BUILD_DIR=build

build-web:
	cd web && npm install && npm run build

build: build-web
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/tun-console/

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

docker-build:
	docker build -t $(BINARY_NAME) .

clean:
	rm -rf $(BUILD_DIR)/
	rm -rf web/node_modules/
	rm -rf web/dist/
