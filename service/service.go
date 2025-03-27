package service

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/requestid"
	ginZap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron/v2"
	ut "github.com/go-playground/universal-translator"
	"github.com/google/uuid"
	"github.com/ipipdotnet/ipdb-go"
	"github.com/orca-zhang/ecache"
	"github.com/orca-zhang/ecache/dist"
	"github.com/redis/go-redis/v9"
	"github.com/sony/sonyflake"
	"github.com/sundaqiang/sdq-go/common"
	"github.com/valyala/fasthttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
	"strconv"
	"time"
	_ "time/tzdata"
)

var (
	config Config
	//json           = sonic.ConfigFastest
	json = sonic.Config{
		NoQuoteTextMarshaler:    true,
		NoValidateJSONMarshaler: true,
		NoValidateJSONSkip:      true,
		UseNumber:               true,
	}.Froze()
	trans          ut.Translator
	ValidatorRegs  *ValidatorReg
	GoCron         gocron.Scheduler
	Db             *gorm.DB
	Rdb            []*redis.Client
	FastHttpClient *fasthttp.Client
	SonyFlake      *sonyflake.Sonyflake
	ZapLog         *zap.Logger
	LRUCache       *ecache.Cache
	Ipdb           *ipdb.City
	Mdb            *mongo.Client
	Limiter        *RedisRate
)

type GeneralTracer struct {
	Cache *ecache.Cache
	Ctx   *context.Context
	Cron  gocron.Scheduler
	Db    *gorm.DB
	Http  *fasthttp.Client
	Log   *zap.Logger
	Rdb   []*redis.Client
	Sony  *sonyflake.Sonyflake
	Tid   string
}

// GetGeneralTracer 获取上下文实例
func GetGeneralTracer() *GeneralTracer {
	tid := uuid.New().String()
	c := context.WithValue(context.Background(), config.Server.Trace, tid)
	return &GeneralTracer{
		Cache: LRUCache,
		Cron:  GoCron,
		Ctx:   &c,
		Db:    Db.WithContext(c),
		Http:  FastHttpClient,
		Log:   ZapLog.With(zap.String(config.Server.Trace, tid)),
		Rdb:   Rdb,
		Sony:  SonyFlake,
		Tid:   tid,
	}
}

// InitGORM 初始化GORM
func InitGORM(info *Gorm) {
	if info != nil {
		initDB(info)
	}
}

// InitRdb 初始化Redis
func InitRdb(info *[]Redis) {
	if info != nil {
		for _, v := range *info {
			if v.Network != "" && v.Addr != "" {
				v.initRedis()
			}
		}
	}
}

// InitGoCron 初始化GoCron
func InitGoCron(cronAsync bool) {
	var err error
	t, timeLocationErr := time.LoadLocation("Asia/Shanghai")
	if timeLocationErr != nil {
		t = time.FixedZone("CST", 8*3600)
	}
	GoCron, err = gocron.NewScheduler(
		gocron.WithLocation(t),
		gocron.WithGlobalJobOptions(
			gocron.WithSingletonMode(
				gocron.LimitModeReschedule,
			),
		),
	)
	if err != nil {
		ZapLog.Error("GoCron初始化失败",
			zap.Error(err),
		)
		return
	}
	if cronAsync {
		GoCron.Start()
	}
}

/*
InitGin 初始化Gin
编译需要加tags
-tags "sonic avx linux amd64"
*/
func InitGin(router func(r *gin.Engine), skipPaths []string) {
	if config.Server.Host != "" {
		if config.Server.Host = common.MatchIp(config.Server.Host); config.Server.Host == "" {
			ZapLog.Error("Gin初始化失败",
				zap.Error(errors.New("错误的Ip地址")),
			)
			return
		}
	}
	if config.Server.Port < 1 {
		ZapLog.Error("Gin初始化失败",
			zap.Error(errors.New("错误的端口号")),
		)
		return
	}
	if config.Server.Trans {
		if err := initValidator("zh"); err != nil {
			ZapLog.Error("Gin初始化失败",
				zap.Error(err),
			)
			return
		}
	}
	r := gin.New()
	r.Use(gin.Recovery())
	// 一个唯一id的中间件
	r.Use(requestid.New(
		requestid.WithCustomHeaderStrKey(requestid.HeaderStrKey(common.KebabString(config.Server.Trace))),
	))
	// pprof
	if gin.Mode() != gin.ReleaseMode {
		pprof.Register(r, "dev/pprof")
	}

	// 加载路由
	if router == nil {
		ZapLog.Error("Gin初始化失败",
			zap.Error(errors.New("错误的路由函数")),
		)
		return
	}

	// 将gin的日志改为zap
	r.Use(GinZapWithConfig(ZapLog,
		&ginZap.Config{
			UTC:        false,
			TimeFormat: time.RFC3339,
			SkipPaths:  skipPaths,
		},
		config.Server.Trace,
	))
	r.Use(ginZap.RecoveryWithZap(ZapLog, true))

	router(r)

	err := r.Run(config.Server.Host + ":" + strconv.Itoa(config.Server.Port))
	if err != nil {
		ZapLog.Error("Gin初始化失败",
			zap.Error(err),
		)
		return
	}
}

/*
InitLogger 必须

	&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    1,
		MaxBackups: 30,
		MaxAge:     7,
		LocalTime:  true,
		Compress:   false,
	}
*/
func InitLogger(logger *lumberjack.Logger, callerSkip int) bool {
	if logger == nil {
		return false
	}
	debugLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.DebugLevel
	})
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	var debugLogger, infoLogger, warnLogger, errorLogger *lumberjack.Logger
	if logger.Filename != "" {
		debugLogger = &lumberjack.Logger{
			Filename: logger.Filename + "_debug.log", MaxSize: logger.MaxSize,
			MaxBackups: logger.MaxBackups, MaxAge: logger.MaxAge,
			LocalTime: logger.LocalTime, Compress: logger.Compress,
		}
		infoLogger = &lumberjack.Logger{
			Filename: logger.Filename + "_info.log", MaxSize: logger.MaxSize,
			MaxBackups: logger.MaxBackups, MaxAge: logger.MaxAge,
			LocalTime: logger.LocalTime, Compress: logger.Compress,
		}
		warnLogger = &lumberjack.Logger{
			Filename: logger.Filename + "_warn.log", MaxSize: logger.MaxSize,
			MaxBackups: logger.MaxBackups, MaxAge: logger.MaxAge,
			LocalTime: logger.LocalTime, Compress: logger.Compress,
		}
		errorLogger = &lumberjack.Logger{
			Filename: logger.Filename + "_error.log", MaxSize: logger.MaxSize,
			MaxBackups: logger.MaxBackups, MaxAge: logger.MaxAge,
			LocalTime: logger.LocalTime, Compress: logger.Compress,
		}
	}
	debugWriter := getLogWriter(debugLogger, true)
	infoWriter := getLogWriter(infoLogger, true)
	warnWriter := getLogWriter(warnLogger, false)
	errorWriter := getLogWriter(errorLogger, false)
	encoder := getEncoder()
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(debugWriter), debugLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), warnLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel),
	)
	ZapLog = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(callerSkip))
	defer func(zapLog *zap.Logger) {
		err := zapLog.Sync()
		if err != nil {

		}
	}(ZapLog)
	return true
}

// InitFastHttp 可选，CheckNetwork 检查是否有网络必选
func InitFastHttp(proxyAddr string) {
	if proxyAddr == "" {
		FastHttpClient = &fasthttp.Client{
			MaxConnsPerHost: 10240,
		}
		return
	}
	FastHttpClient = &fasthttp.Client{
		MaxConnsPerHost: 10240,
		Dial:            fastHTTPDialer(proxyAddr),
	}
}

// InitSonyFlake 初始化雪花Id
func InitSonyFlake(settings sonyflake.Settings) bool {
	var err error
	SonyFlake, err = sonyflake.New(settings)
	if err != nil {
		ZapLog.Error("雪花id初始化失败",
			zap.Error(err),
		)
		return false
	}
	return true
}

/*
InitLocalCache 本地缓存

	bucketCnt:分片数量 MAX=65535
	capPerBkt:分配尺寸 MAX=65535
	capPerBkt2:热队列尺寸 CLOSE=0
	rdb:是否绑定redis CLOSE=nil
	size:redis缓存区尺寸
*/
func InitLocalCache(bucketCnt, capPerBkt, capPerBkt2 uint16, rdb *redis.Client, size int, expiration time.Duration) {
	if capPerBkt2 > 0 {
		LRUCache = ecache.NewLRUCache(bucketCnt, capPerBkt, expiration).LRU2(capPerBkt2)
	} else {
		LRUCache = ecache.NewLRUCache(bucketCnt, capPerBkt, expiration)
	}
	if rdb != nil {
		dist.Init(Take(rdb, size))
	}
}
