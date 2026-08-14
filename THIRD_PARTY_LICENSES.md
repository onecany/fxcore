# Third-Party Licenses

本项目（FXcore）构建所引用的 Go 依赖及其开源许可证清单。
由 `go-licenses` 扫描生成，随 go.mod 变更后重新生成。

- 依赖总数：62
- 许可证分布：MIT × 34、BSD-3-Clause × 13、Apache-2.0 × 12、BSD-2-Clause × 2、MPL-2.0 × 1

## 许可证合规说明

- **依赖树零 GPL 家族**：全部 62 个依赖中没有任何 GPL/LGPL/AGPL 授权。
- **MPL-2.0（go-sql-driver/mysql）非 GPL 家族**：MPL-2.0 是文件级弱 copyleft，
  与 GPL 是互不相关的两类许可（且 MPL-2.0 与 GPL 兼容）。作为依赖**原样使用不修改**时
  义务为零——只有修改其源文件并分发时才需按 MPL-2.0 公开改动。本项目以链接方式原样使用，
  不触发任何 copyleft 义务。该驱动是 Go 官方 wiki 推荐的 MySQL database/sql 事实标准驱动，
  无同等维护水平的 MIT/Apache/BSD 替代（替代驱动会因 gorm.io/driver/mysql 源码直接
  import 其 ParseDSN 而仍以间接依赖形式留在 go.mod）。

| 模块 | 版本 | 许可证 | 许可证链接 |
|---|---|---|---|
| filippo.io/edwards25519 | v1.1.0 | [BSD-3-Clause](https://github.com/FiloSottile/edwards25519/blob/v1.1.0/LICENSE) |
| github.com/KyleBanks/depth | v1.2.1 | [MIT](https://github.com/KyleBanks/depth/blob/v1.2.1/LICENSE) |
| github.com/PuerkitoBio/purell | v1.1.1 | [BSD-3-Clause](https://github.com/PuerkitoBio/purell/blob/v1.1.1/LICENSE) |
| github.com/PuerkitoBio/urlesc | v0.0.0-20170810143723-de5bf2ad4578 | [BSD-3-Clause](https://github.com/PuerkitoBio/urlesc/blob/de5bf2ad4578/LICENSE) |
| github.com/bytedance/gopkg | v0.1.3 | [Apache-2.0](https://github.com/bytedance/gopkg/blob/v0.1.3/LICENSE) |
| github.com/bytedance/sonic | v1.15.0 | [Apache-2.0](https://github.com/bytedance/sonic/blob/v1.15.0/LICENSE) |
| github.com/bytedance/sonic/loader | v0.5.0 | [Apache-2.0](https://github.com/bytedance/sonic/blob/v1.15.0/LICENSE) |
| github.com/cespare/xxhash/v2 | v2.3.0 | [MIT](https://github.com/cespare/xxhash/blob/v2.3.0/LICENSE.txt) |
| github.com/cloudwego/base64x | v0.1.6 | [Apache-2.0](https://github.com/cloudwego/base64x/blob/v0.1.6/LICENSE) |
| github.com/gabriel-vasile/mimetype | v1.4.12 | [MIT](https://github.com/gabriel-vasile/mimetype/blob/v1.4.12/LICENSE) |
| github.com/gin-contrib/gzip | v1.2.6 | [MIT](https://github.com/gin-contrib/gzip/blob/v1.2.6/LICENSE) |
| github.com/gin-contrib/sse | v1.1.0 | [MIT](https://github.com/gin-contrib/sse/blob/v1.1.0/LICENSE) |
| github.com/gin-gonic/gin | v1.12.0 | [MIT](https://github.com/gin-gonic/gin/blob/v1.12.0/LICENSE) |
| github.com/go-openapi/jsonpointer | v0.19.5 | [Apache-2.0](https://github.com/go-openapi/jsonpointer/blob/v0.19.5/LICENSE) |
| github.com/go-openapi/jsonreference | v0.19.6 | [Apache-2.0](https://github.com/go-openapi/jsonreference/blob/v0.19.6/LICENSE) |
| github.com/go-openapi/spec | v0.20.4 | [Apache-2.0](https://github.com/go-openapi/spec/blob/v0.20.4/LICENSE) |
| github.com/go-openapi/swag | v0.19.15 | [Apache-2.0](https://github.com/go-openapi/swag/blob/v0.19.15/LICENSE) |
| github.com/go-playground/locales | v0.14.1 | [MIT](https://github.com/go-playground/locales/blob/v0.14.1/LICENSE) |
| github.com/go-playground/universal-translator | v0.18.1 | [MIT](https://github.com/go-playground/universal-translator/blob/v0.18.1/LICENSE) |
| github.com/go-playground/validator/v10 | v10.30.1 | [MIT](https://github.com/go-playground/validator/blob/v10.30.1/LICENSE) |
| github.com/go-sql-driver/mysql | v1.8.1 | [MPL-2.0](https://github.com/go-sql-driver/mysql/blob/v1.8.1/LICENSE) |
| github.com/goccy/go-json | v0.10.5 | [MIT](https://github.com/goccy/go-json/blob/v0.10.5/LICENSE) |
| github.com/goccy/go-yaml | v1.19.2 | [MIT](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE) |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | [MIT](https://github.com/golang-jwt/jwt/blob/v5.3.1/LICENSE) |
| github.com/gorilla/websocket | v1.5.3 | [BSD-2-Clause](https://github.com/gorilla/websocket/blob/v1.5.3/LICENSE) |
| github.com/jinzhu/inflection | v1.0.0 | [MIT](https://github.com/jinzhu/inflection/blob/v1.0.0/LICENSE) |
| github.com/jinzhu/now | v1.1.5 | [MIT](https://github.com/jinzhu/now/blob/v1.1.5/License) |
| github.com/joho/godotenv | v1.5.1 | [MIT](https://github.com/joho/godotenv/blob/v1.5.1/LICENCE) |
| github.com/josharian/intern | v1.0.0 | [MIT](https://github.com/josharian/intern/blob/v1.0.0/license.md) |
| github.com/json-iterator/go | v1.1.12 | [MIT](https://github.com/json-iterator/go/blob/v1.1.12/LICENSE) |
| github.com/klauspost/cpuid/v2 | v2.3.0 | [MIT](https://github.com/klauspost/cpuid/blob/v2.3.0/LICENSE) |
| github.com/leodido/go-urn | v1.4.0 | [MIT](https://github.com/leodido/go-urn/blob/v1.4.0/LICENSE) |
| github.com/mailru/easyjson | v0.7.6 | [MIT](https://github.com/mailru/easyjson/blob/v0.7.6/LICENSE) |
| github.com/mattn/go-isatty | v0.0.20 | [MIT](https://github.com/mattn/go-isatty/blob/v0.0.20/LICENSE) |
| github.com/mattn/go-sqlite3 | v1.14.22 | [MIT](https://github.com/mattn/go-sqlite3/blob/v1.14.22/LICENSE) |
| github.com/modern-go/concurrent | v0.0.0-20180306012644-bacd9c7ef1dd | [Apache-2.0](https://github.com/modern-go/concurrent/blob/master/LICENSE) |
| github.com/modern-go/reflect2 | v1.0.2 | [Apache-2.0](https://github.com/modern-go/reflect2/blob/v1.0.2/LICENSE) |
| github.com/pelletier/go-toml/v2 | v2.2.4 | [MIT](https://github.com/pelletier/go-toml/blob/v2.2.4/LICENSE) |
| github.com/quic-go/qpack | v0.6.0 | [MIT](https://github.com/quic-go/qpack/blob/v0.6.0/LICENSE.md) |
| github.com/quic-go/quic-go | v0.59.0 | [MIT](https://github.com/quic-go/quic-go/blob/v0.59.0/LICENSE) |
| github.com/redis/go-redis/v9 | v9.22.0 | [BSD-2-Clause](https://github.com/redis/go-redis/blob/v9.22.0/LICENSE) |
| github.com/swaggo/files | v1.0.1 | [MIT](https://github.com/swaggo/files/blob/v1.0.1/LICENSE) |
| github.com/swaggo/gin-swagger | v1.6.1 | [MIT](https://github.com/swaggo/gin-swagger/blob/v1.6.1/LICENSE) |
| github.com/swaggo/swag | v1.16.6 | [MIT](https://github.com/swaggo/swag/blob/v1.16.6/license) |
| github.com/twitchyliquid64/golang-asm | v0.15.1 | [BSD-3-Clause](https://github.com/twitchyliquid64/golang-asm/blob/v0.15.1/LICENSE) |
| github.com/ugorji/go/codec | v1.3.1 | [MIT](https://github.com/ugorji/go/blob/codec/v1.3.1/LICENSE) |
| go.mongodb.org/mongo-driver/v2 | v2.5.0 | [Apache-2.0](https://github.com/mongodb/mongo-go-driver/blob/v2.5.0/LICENSE) |
| go.uber.org/atomic | v1.11.0 | [MIT](https://github.com/uber-go/atomic/blob/v1.11.0/LICENSE.txt) |
| golang.org/x/arch | v0.22.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/arch/+/v0.22.0:LICENSE) |
| golang.org/x/crypto | v0.54.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/crypto/+/v0.54.0:LICENSE) |
| golang.org/x/mod | v0.37.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/mod/+/v0.37.0:LICENSE) |
| golang.org/x/net | v0.57.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/net/+/v0.57.0:LICENSE) |
| golang.org/x/sync | v0.22.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/sync/+/v0.22.0:LICENSE) |
| golang.org/x/sys | v0.47.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/sys/+/v0.47.0:LICENSE) |
| golang.org/x/text | v0.40.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/text/+/v0.40.0:LICENSE) |
| golang.org/x/tools | v0.47.0 | [BSD-3-Clause](https://cs.opensource.google/go/x/tools/+/v0.47.0:LICENSE) |
| google.golang.org/protobuf | v1.36.10 | [BSD-3-Clause](https://github.com/protocolbuffers/protobuf-go/blob/v1.36.10/LICENSE) |
| gopkg.in/natefinch/lumberjack.v2 | v2.2.1 | [MIT](https://github.com/natefinch/lumberjack/blob/v2.2.1/LICENSE) |
| gopkg.in/yaml.v2 | v2.4.0 | [Apache-2.0](https://github.com/go-yaml/yaml/blob/v2.4.0/LICENSE) |
| gorm.io/driver/mysql | v1.6.0 | [MIT](https://github.com/go-gorm/mysql/blob/v1.6.0/License) |
| gorm.io/driver/sqlite | v1.6.0 | [MIT](https://github.com/go-gorm/sqlite/blob/v1.6.0/License) |
| gorm.io/gorm | v1.31.2 | [MIT](https://github.com/go-gorm/gorm/blob/v1.31.2/LICENSE) |

## 重新生成

go.mod 变更后重新生成此清单：

```bash
go install github.com/google/go-licenses@latest
go-licenses csv ./...   # 输出依赖与许可证；fxcore/ 自身包与 web/node_modules 误扫项需排除
```

`go-licenses` 只扫描实际被编译引用的包：gin 默认使用 `encoding/json`，
`bytedance/sonic` 依赖链（bytedance/sonic、cloudwego/base64x、json-iterator 等）
未被实际 import，需按上表手动补充（license 类型取自模块缓存内 LICENSE 文件）。
