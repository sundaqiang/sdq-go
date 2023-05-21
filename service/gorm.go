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
	/*newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer（日志输出的目标，前缀和日志包含的内容——译者注）
		logger.Config{
			SlowThreshold:             time.Second, // 慢 SQL 阈值
			LogLevel:                  logger.Info, // 日志级别
			IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  colorful,    // 禁用彩色打印
		},
	)*/
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
				// `Dsn` 作为 sources（DB 的默认连接），对于 `TkExtShopDongming` 使用 `Dsn2` 作为 replicas
				Sources: []gorm.Dialector{mysql.Open(resolver.Dsn)},
			}, resolver.Datas...))

			// log.Printf("准备连接数据库: %s", dsn)
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
