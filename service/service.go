package service

import (
	"database/sql"
	ginZap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/redis/go-redis/v9"
	"github.com/sundaqiang/sdq-go/common"
	"gorm.io/gorm"
	"time"
	_ "time/tzdata"
)

var (
	GoCron *gocron.Scheduler
	Db     *gorm.DB
	Rdb    []*redis.Client
	sqlDB  *sql.DB
)

// InitGORM 初始化GORM
func InitGORM(info *GormInfo) {
	initDB(info)
}

// InitRdb 初始化Redis
func InitRdb(info *RdbInfo) {
	info.initRedis()
}

// InitGoCron 初始化GoCron
func InitGoCron() {
	t, timeLocationErr := time.LoadLocation("Asia/Shanghai")
	if timeLocationErr != nil {
		t = time.FixedZone("CST", 8*3600)
	}
	GoCron = gocron.NewScheduler(t)
	GoCron.SingletonModeAll()
	GoCron.StartAsync()
}

/*
InitGin 初始化Gin
编译需要加tags
-tags "sonic avx linux amd64"
*/
func InitGin(serverAddr, serverPort string, router func(r *gin.Engine)) {
	r := gin.New()
	// 将gin的日志改为zap
	r.Use(ginZap.Ginzap(common.ZapLog, time.RFC3339, true))
	r.Use(ginZap.RecoveryWithZap(common.ZapLog, true))
	// 加载路由
	router(r)
	err := r.Run(serverAddr + ":" + serverPort)
	if err != nil {
		return
	}
}
