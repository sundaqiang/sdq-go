package service

import (
	"database/sql"
	"errors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/requestid"
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
	trans         ut.Translator
	ValidatorRegs *ValidatorReg
	GoCron        *gocron.Scheduler
	Db            *gorm.DB
	Rdb           []*redis.Client
	sqlDB         *sql.DB
	IsTrans       bool
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
func InitGin(serverAddr string, serverPort int, isTrans bool, router func(r *gin.Engine)) {
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
		if err := initValidator("zh"); err != nil {
			common.ZapLog.Error("Gin初始化失败",
				zap.Error(err),
			)
			return
		}
		IsTrans = isTrans
	}
	r := gin.New()
	r.Use(gin.Recovery())
	// 一个唯一id的中间件
	r.Use(requestid.New())
	// pprof
	if gin.Mode() != gin.ReleaseMode {
		pprof.Register(r, "dev/pprof")
	}
	// 将gin的日志改为zap
	r.Use(common.GinzapWithConfig(common.ZapLog, &ginZap.Config{
		UTC:        false,
		TimeFormat: time.RFC3339,
	}))
	r.Use(ginZap.RecoveryWithZap(common.ZapLog, true))
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
