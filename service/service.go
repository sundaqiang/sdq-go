package service

import (
	"database/sql"
	"errors"
	ginZap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	ut "github.com/go-playground/universal-translator"
	"github.com/redis/go-redis/v9"
	"github.com/sundaqiang/sdq-go/common"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"strconv"
	"time"
	_ "time/tzdata"
)

var (
	trans  ut.Translator
	GoCron *gocron.Scheduler
	Db     *gorm.DB
	Rdb    []*redis.Client
	sqlDB  *sql.DB
)

// InitGORM 初始化GORM
func InitGORM(info *GormInfo) {
	if info != nil {
		initDB(info)
	}
}

// InitRdb 初始化Redis
func InitRdb(info *RdbInfo) {
	if info != nil {
		info.initRedis()
	}
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
func InitGin(serverAddr string, serverPort int, isTrans, isCors bool, structs *[]TransStruct, fields *[]TransField, router func(r *gin.Engine)) {
	if serverAddr != "" {
		if serverAddr = common.MatchIp(serverAddr); serverAddr == "" {
			common.ZapLog.Error("Gin初始化失败",
				zap.Error(errors.New("错误的Ip地址")),
			)
			return
		}
	}
	if serverPort < 1 {
		common.ZapLog.Error("Gin初始化失败",
			zap.Error(errors.New("错误的端口号")),
		)
		return
	}
	if isTrans {
		if err := initTrans("zh", structs, fields); err != nil {
			common.ZapLog.Error("Gin初始化失败",
				zap.Error(err),
			)
			return
		}
	}
	r := gin.New()
	// 将gin的日志改为zap
	r.Use(ginZap.Ginzap(common.ZapLog, time.RFC3339, true))
	r.Use(ginZap.RecoveryWithZap(common.ZapLog, true))
	if isCors {
		r.Use(ginCors())
	}
	// 加载路由
	if router == nil {
		common.ZapLog.Error("Gin初始化失败",
			zap.Error(errors.New("错误的路由函数")),
		)
		return
	}
	router(r)
	err := r.Run(serverAddr + ":" + strconv.Itoa(serverPort))
	if err != nil {
		common.ZapLog.Error("Gin初始化失败",
			zap.Error(err),
		)
		return
	}
}
