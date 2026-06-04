.PHONY: build build-web run docker-build clean

BINARY_NAME=tun-console
BUILD_DIR=build

build-web:
	cd web && npm run build

build: build-web
	rm -rf cmd/tun-console/dist
	cp -r web/dist cmd/tun-console/dist
	go build -tags embed -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/tun-console/
	rm -rf cmd/tun-console/dist

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

docker-build:
	docker build -t $(BINARY_NAME) .

clean:
	rm -rf $(BUILD_DIR)/
	rm -rf web/node_modules/
	rm -rf web/dist/
	rm -rf cmd/tun-console/dist/
