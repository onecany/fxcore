// Package openapi3 内嵌 OpenAPI 3.1 规范文档（API设计.md §2 基建），
// 供 /api/v1/openapi3.json 挂载与 swagger UI 加载。
//
// 重新生成（swag v2，需 go install github.com/swaggo/swag/v2/cmd/swag@latest）：
//
//	swag init -g cmd/server/main.go -o docs --outputTypes json,yaml --parseDependency --parseInternal --v3.1
//	cp docs/swagger.json internal/api/v1/openapi3/swagger.json
//
// 仓库根 docs/ 只放文档（swagger.json + swagger.yaml），本包是 embed 副本；
// 前端类型生成：npm --prefix web run gen:api（openapi-typescript 消费本文件）。
package openapi3

import _ "embed"

// Spec OpenAPI 3.1 规范原文（挂载于 /swagger/openapi3.json）。
//
//go:embed swagger.json
var Spec []byte
