package service

import (
	"github.com/sundaqiang/sdq-go/common"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
	"moul.io/zapgorm2"
)

type GormInfo struct {
	Dsn      string
	Resolver []Resolver
}

type Resolver struct {
	Dsn   string
	Datas []interface{}
}

func initDB(info *GormInfo) {
	// 将gorm的日志改为zap
	newLogger := zapgorm2.New(common.ZapLog)
	newLogger.LogLevel = logger.Info
	newLogger.SlowThreshold = time.Second
	newLogger.SkipCallerLookup = false
	newLogger.IgnoreRecordNotFoundError = true
	var err error
	Db, err = gorm.Open(mysql.Open(info.Dsn), &gorm.Config{
		Logger:      newLogger,
		PrepareStmt: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	if err != nil {
		common.ZapLog.Fatal("数据库连接失败", zap.Error(err))
	}

	if len(info.Resolver) > 0 {
		for _, resolver := range info.Resolver {
			err = Db.Use(dbresolver.Register(dbresolver.Config{
				Sources: []gorm.Dialector{mysql.Open(resolver.Dsn)},
			}, resolver.Datas...))
			if err != nil {
				common.ZapLog.Fatal("数据库连接失败", zap.Error(err))
			}
		}
	}

	sqlDB, err = Db.DB()
	sqlDB.SetMaxOpenConns(500)
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	if err != nil {
		common.ZapLog.Fatal("数据库连接状态", zap.Error(err))
	} else {
		common.ZapLog.Info("数据库连接成功")
	}
}
