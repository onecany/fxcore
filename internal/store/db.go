// GORM 数据库层：默认 SQLite（data/fxcore.db），DB_DSN 可切 MariaDB/MySQL。
// 管理实体（users/models/exchanges/strategies/traders/telegram_configs）AutoMigrate 建表，
// 流式运行时数据（orders/fills/decisions/equities/backtest*/debate*）保持进程内内存。
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"fxcore/internal/model"
)

// DefaultDBPath 默认 SQLite 文件（相对数据目录）。
const DefaultDBPath = "fxcore.db"

// OpenDB 按 DSN 打开数据库。
//   - dsn 为空 → SQLite：<dataDir>/fxcore.db（目录自动创建）
//   - dsn 以 "mysql://" 或含 "@tcp(" → MariaDB/MySQL（如 user:pass@tcp(127.0.0.1:3306)/fxcore?charset=utf8mb4&parseTime=True&loc=Local）
//
// 返回 gorm.DB（自动 Ping + 迁移）。调用方负责 Close。
func OpenDB(dsn, dataDir string) (*gorm.DB, error) {
	if dsn == "" {
		dir := dataDir
		if dir == "" {
			dir = "data"
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create data dir: %w", err)
		}
		dsn = filepath.Join(dir, DefaultDBPath)
	}

	var (
		db  *gorm.DB
		err error
	)
	if isMySQLDSN(dsn) {
		// go-sql-driver/mysql 不接受 mysql:// scheme，剥离前缀
		db, err = gorm.Open(mysql.Open(stripMySQLScheme(dsn)), gormConfig())
	} else {
		// sqlite：WAL 提升并发读写 + busy_timeout 避免写锁竞争直接报错
		// （引擎多交易员周期写 trader/positions；WAL 下读不阻塞写）
		sqliteDSN := dsn
		if !strings.Contains(sqliteDSN, "?") {
			sqliteDSN += "?_busy_timeout=5000&_journal_mode=WAL"
		}
		db, err = gorm.Open(sqlite.Open(sqliteDSN), gormConfig())
	}
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("store: sql db handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("store: ping db: %w", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.AIModel{},
		&model.Exchange{},
		&model.Strategy{},
		&model.Trader{},
		&model.TelegramConfig{},
		// ---- 流式运行时数据（订单/成交/持仓/决策/权益）----
		&model.Order{},
		&model.Fill{},
		&model.Position{},
		&model.DecisionRecord{},
		&model.EquitySnapshot{},
		// ---- 回测 ----
		&model.BacktestRun{},
		&model.BacktestEquity{},
		&model.BacktestTrade{},
		&model.BacktestDecision{},
		&model.BacktestCheckpoint{},
		// ---- 辩论 ----
		&model.DebateSession{},
		&model.DebateParticipant{},
		&model.DebateMessage{},
		&model.DebateVote{},
	); err != nil {
		return nil, fmt.Errorf("store: automigrate: %w", err)
	}
	return db, nil
}

// isMySQLDSN 判定 MariaDB/MySQL DSN：mysql:// 前缀或标准 DSN 形态（user:pass@tcp(...)）。
func isMySQLDSN(dsn string) bool {
	if strings.HasPrefix(dsn, "mysql://") {
		return true
	}
	return strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "@unix(")
}

// stripMySQLScheme 剥离 mysql:// 前缀（go-sql-driver/mysql 不接受 scheme）。
func stripMySQLScheme(dsn string) string {
	return strings.TrimPrefix(dsn, "mysql://")
}

func gormConfig() *gorm.Config {
	return &gorm.Config{
		Logger:         gormlogger.Default.LogMode(gormlogger.Warn),
		TranslateError: true, // unique 冲突 → gorm.ErrDuplicatedKey（订单/成交幂等依赖）
	}
}
