// Package logger 应用日志 + API 访问日志双写（文件轮转 + 终端）。
//
// 设计：
//   - 零侵入接入：Init 重定向标准库 log 输出（io.MultiWriter 文件+终端），
//     仓库现有 log.Printf 全部自动双写，无需逐个替换
//   - 轮转：lumberjack（大小 10MiB / 保留 7 份 / 压缩旧档），防止单文件无限增长
//   - 访问日志独立文件（access.log），结构化 JSON 一行一条，便于 grep/jq
//
// 环境变量：
//
//	LOG_DIR  日志目录（默认 "logs"，相对工作目录；设空串禁用文件日志，仅终端输出）
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置。
type Config struct {
	// Dir 日志目录；空串表示仅终端输出（不写文件）。
	Dir string
	// MaxSizeMB 单个文件轮转大小（MB）。
	MaxSizeMB int
	// MaxBackups 保留的旧文件份数。
	MaxBackups int
}

// Defaults 默认配置。
var Defaults = Config{
	Dir:        "logs",
	MaxSizeMB:  10,
	MaxBackups: 7,
}

// accessLog 访问日志 logger（独立于应用日志）。
var (
	accessMu sync.RWMutex
	access   *log.Logger // nil 表示未初始化
)

// Init 初始化日志系统（幂等）：
//   - 应用日志：标准库 log 输出重定向到 文件+终端（MultiWriter）
//   - 访问日志：创建独立 *log.Logger（文件+终端），供 AccessLog 中间件使用
//   - gin 内部日志（DefaultWriter/ErrorWriter）一并指向同一输出
//
// 返回访问日志 logger（Init 后 Access() 亦可获取）。
func Init(cfg Config) (*log.Logger, error) {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = Defaults.MaxSizeMB
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = Defaults.MaxBackups
	}

	appWriter := io.Writer(os.Stdout)
	var accessWriter io.Writer = os.Stdout

	if cfg.Dir != "" {
		if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir %s: %w", cfg.Dir, err)
		}
		appFile := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Dir, "fxcore.log"),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			Compress:   true,
		}
		accessFile := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Dir, "access.log"),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			Compress:   true,
		}
		appWriter = io.MultiWriter(os.Stdout, appFile)
		accessWriter = io.MultiWriter(os.Stdout, accessFile)
	}

	// 应用日志：标准库 log 全局重定向（现有 log.Printf 全部生效）
	log.SetOutput(appWriter)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// gin 内部日志同源
	gin.DefaultWriter = appWriter
	gin.DefaultErrorWriter = appWriter

	accessMu.Lock()
	access = log.New(accessWriter, "", log.LstdFlags|log.Lmicroseconds)
	accessMu.Unlock()

	if cfg.Dir != "" {
		log.Printf("[logger] app log -> %s/fxcore.log, access log -> %s/access.log (rotate %dMiB x%d, terminal mirror on)",
			cfg.Dir, cfg.Dir, cfg.MaxSizeMB, cfg.MaxBackups)
	} else {
		log.Printf("[logger] file logging disabled (LOG_DIR empty), terminal only")
	}
	return access, nil
}

// Access 返回访问日志 logger（Init 前调用返回 nil，调用方需判空）。
func Access() *log.Logger {
	accessMu.RLock()
	defer accessMu.RUnlock()
	return access
}
