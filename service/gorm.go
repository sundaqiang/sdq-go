package service

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sundaqiang/sdq-go/common"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
	"moul.io/zapgorm2"
)

type Gorm struct {
	Type     string     `toml:"type"`
	Host     string     `toml:"host"`
	User     string     `toml:"user"`
	Password string     `toml:"password"`
	Name     string     `toml:"name"`
	Resolver []Resolver `toml:"resolver"`
}

type Resolver struct {
	Type     string `toml:"type"`
	Host     string `toml:"host"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Name     string `toml:"name"`
	Driver   *gorm.Dialector
	Data     []interface{}
}

func initDB(info *Gorm) {
	// 将gorm的日志改为zap
	newLogger := zapgorm2.New(ZapLog)
	newLogger.LogLevel = logger.Info
	newLogger.SlowThreshold = time.Second
	newLogger.SkipCallerLookup = false
	newLogger.IgnoreRecordNotFoundError = true
	if config.Server.Trace != "" {
		newLogger.Context = func(ctx context.Context) []zapcore.Field {
			c, ok := ctx.(*gin.Context)
			if ok {
				return []zapcore.Field{
					zap.String(
						config.Server.Trace,
						c.Writer.Header().Get(
							common.KebabString(config.Server.Trace),
						),
					),
				}
			}
			// 获取键值
			value := ctx.Value(config.Server.Trace)
			// 检查值是否存在
			if value != nil {
				return []zapcore.Field{
					zap.String(
						config.Server.Trace,
						c.Writer.Header().Get(
							common.KebabString(config.Server.Trace),
						),
					),
				}
			}
			return []zapcore.Field{}
		}
	}
	var err error
	var driver gorm.Dialector
	switch info.Type {
	case "mysql":
		driver = mysql.Open(info.User + ":" + info.Password + "@tcp(" + info.Host + ")/" + info.Name + "?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai")
	case "sqlite":
		driver = sqlite.Open(info.Name)
	}
	Db, err = gorm.Open(driver, &gorm.Config{
		Logger:                 newLogger,
		QueryFields:            true,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	if err != nil {
		ZapLog.Fatal("数据库连接失败", zap.Error(err))
	}

	if len(info.Resolver) > 0 {
		for _, resolver := range info.Resolver {
			switch info.Type {
			case "mysql":
				driver = mysql.Open(info.User + ":" + info.Password + "@tcp(" + info.Host + ")/" + info.Name + "?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai")
			case "sqlite":
				driver = sqlite.Open(info.Name)
			}
			err = Db.Use(dbresolver.Register(dbresolver.Config{
				Sources: []gorm.Dialector{driver},
			}, resolver.Data...))
			if err != nil {
				ZapLog.Fatal("数据库连接失败", zap.Error(err))
			}
		}
	}

	sqlDB, err := Db.DB()
	sqlDB.SetMaxOpenConns(500)
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	if err != nil {
		ZapLog.Fatal("数据库连接状态", zap.Error(err))
	} else {
		ZapLog.Info("数据库连接成功")
	}
}
