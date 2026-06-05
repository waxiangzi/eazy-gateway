# tun-console 构建与任务自动化
# 使用 `just <recipe>` 运行，详见 https://github.com/casey/just

# --- 变量 ---
binary_name := "tun-console"
build_dir   := "build"

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

# 构建 Docker 镜像
docker-build:
    docker build -t '{{binary_name}}' .

# 清理构建产物
clean:
    rm -rf '{{build_dir}}/'
    rm -rf web/node_modules/
    rm -rf web/dist/
    rm -rf cmd/tun-console/dist/

# 显示所有可用配方
default:
    @just --list
