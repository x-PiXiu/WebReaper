.PHONY: build-server dev-server dev-web build-web test clean

# 构建后端 API 服务（纯 API，不含前端——前端由 Nginx 独立部署）
build-server:
	go build -o bin/webreaper ./cmd/server

# 开发模式跑后端 API（:8082）
dev-server:
	go run ./cmd/server

# 开发模式跑前端（:5173，/api 经 vite proxy 代理到后端 8082，免 CORS）
dev-web:
	cd web && npm run dev

# 构建前端产物（输出到 web/dist，供 Nginx 托管）
build-web:
	cd web && npm run build

# 跑全部测试
test:
	go test ./...

clean:
	rm -rf bin/ web/dist/
