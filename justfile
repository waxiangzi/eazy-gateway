# tun-console 构建与任务自动化
# 使用 `just <recipe>` 运行，详见 https://github.com/casey/just

# --- 变量 ---
binary_name := "tun-console"
build_dir   := "build"
go_mod_dir  := "cmd/tun-console"

# --- 配方 ---

# 构建前端（Vue SPA）
build-web:
    cd web && npm run build

# 构建完整项目（前端 + Go 二进制）
build: build-web
    rm -rf cmd/tun-console/dist
    cp -r web/dist cmd/tun-console/dist
    go build -tags embed -o '{{build_dir}}/{{binary_name}}' ./cmd/tun-console/
    rm -rf cmd/tun-console/dist

# 构建并运行
run: build
    ./'{{build_dir}}/{{binary_name}}'

# 开发模式：Go 后端自动编译重启
dev:
    #!/usr/bin/env bash
    set -euo pipefail

    # 查找 air 二进制文件（可能不在 GOPATH，例如 mise 环境）
    AIR_BIN=""
    if command -v air &>/dev/null; then
        AIR_BIN="$(command -v air)"
    elif [ -x "$(go env GOPATH)/bin/air" ]; then
        AIR_BIN="$(go env GOPATH)/bin/air"
    fi
    if [ -z "$AIR_BIN" ]; then
        echo "Installing air (Go hot reload tool)..."
        go install github.com/air-verse/air@latest
        # 安装后重新查找
        if command -v air &>/dev/null; then
            AIR_BIN="$(command -v air)"
        elif [ -x "$(go env GOPATH)/bin/air" ]; then
            AIR_BIN="$(go env GOPATH)/bin/air"
        else
            echo "ERROR: air was installed but could not be found in PATH."
            echo "Check your Go installation and PATH settings."
            exit 1
        fi
    fi

    # 确保前端 dist 存在以便 Go noembed 模式能提供服务
    if [ ! -d web/dist ] && [ ! -d cmd/tun-console/dist ]; then
        just build-web
    fi

    # 生成 air 配置文件
    mkdir -p tmp
    printf '%s\n' \
        'root = "."' \
        'testdata_dir = "testdata"' \
        'tmp_dir = "tmp"' \
        '' \
        '[build]' \
        '  args_bin = []' \
        '  bin = "./tmp/main"' \
        '  cmd = "go build -o ./tmp/main ./cmd/tun-console/"' \
        '  delay = 1000' \
        '  exclude_dir = ["assets", "tmp", "vendor", "web/node_modules", "web/dist", "build"]' \
        '  exclude_file = []' \
        '  exclude_regex = ["_test.go"]' \
        '  exclude_unchanged = false' \
        '  follow_symlink = false' \
        '  full_bin = ""' \
        '  include_dir = []' \
        '  include_ext = ["go"]' \
        '  kill_delay = "0s"' \
        '  log = "build-errors.log"' \
        '  send_interrupt = false' \
        '  stop_on_root = false' \
        '' \
        '[color]' \
        '  app = ""' \
        '  build = "yellow"' \
        '  main = "magenta"' \
        '  runner = "green"' \
        '  watcher = "cyan"' \
        '' \
        '[log]' \
        '  time = false' \
        '' \
        '[misc]' \
        '  clean_on_exit = false' \
        '' \
        '[screen]' \
        '  clear_on_rebuild = false' \
        > .air.toml

    echo "Starting dev server (Go hot-reload via air)..."
    "$AIR_BIN" -c .air.toml &
    AIR_PID=$!

    trap 'kill $AIR_PID 2>/dev/null; wait $AIR_PID 2>/dev/null; exit' INT TERM
    wait "$AIR_PID"

# 构建 Docker 镜像
docker-build:
    docker build -t '{{binary_name}}' .

# 清理构建产物（含 air tmp、node_modules）
clean:
    rm -rf '{{build_dir}}/'
    rm -rf web/node_modules/
    rm -rf web/dist/
    rm -rf cmd/tun-console/dist/
    rm -rf tmp/
    rm -f .air.toml

# 显示所有可用配方
default:
    @just --list
