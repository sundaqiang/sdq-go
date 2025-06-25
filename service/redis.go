package service

import (
	"context"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net"
	"time"

	"go.uber.org/zap"
)

type Redis struct {
	Network  string `toml:"network"`
	Addr     string `toml:"addr"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`
}

type LogHook struct{}

func (LogHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}
func (LogHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		traceID := ""
		if c, ok := ctx.(*gin.Context); ok {
			traceID = requestid.Get(c)
		} else if v := ctx.Value(config.Server.Trace); v != nil {
			if s, ok := v.(string); ok {
				traceID = s
			}
		}
		err := next(ctx, cmd)
		l := ZapLog
		if traceID != "" {
			l = l.With(zap.String(config.Server.Trace, traceID))
		}
		if err != nil || cmd.Err() != nil {
			l.Error(
				"redis_trace",
				zap.Any("args", cmd.Args()),
				zap.Error(err),
			)
		} else {
			l.Debug(
				"redis_trace",
				zap.Any("args", cmd.Args()),
				zap.String("cmd", cmd.String()),
			)
		}
		return err
	}
}

func (LogHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		traceID := ""
		if c, ok := ctx.(*gin.Context); ok {
			traceID = requestid.Get(c)
		} else if v := ctx.Value(config.Server.Trace); v != nil {
			if s, ok := v.(string); ok {
				traceID = s
			}
		}
		err := next(ctx, cmds)
		l := ZapLog
		if traceID != "" {
			l = l.With(zap.String(config.Server.Trace, traceID))
		}
		for _, cmd := range cmds {
			if cmd.Err() != nil {
				l.Error(
					"redis_trace",
					zap.Any("args", cmd.Args()),
					zap.Error(cmd.Err()),
				)
			} else if err != nil { // pipeline整体如果报错，也输出一下
				l.Error(
					"redis_trace",
					zap.Any("args", cmd.Args()),
					zap.Error(err),
				)
			} else {
				l.Debug(
					"redis_trace",
					zap.Any("args", cmd.Args()),
					zap.String("cmd", cmd.String()),
				)
			}
		}
		return err
	}
}

func (r *Redis) initRedis() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		// 连接信息
		Network:  r.Network,  // 网络类型，tcp or unix，默认tcp
		Addr:     r.Addr,     // 主机名+冒号+端口，默认localhost:6379
		Username: r.Username, // 用户
		Password: r.Password, // 密码
		DB:       r.DB,       // redis数据库index
		// 连接池容量及闲置连接数量
		PoolSize:     30, // 连接池最大socket连接数，默认为4倍CPU数， 4 * runtime.NumCPU
		MinIdleConns: 10, // 连接池保持的最小空闲连接数，它受到PoolSize的限制 默认为0，不保持
		MaxIdleConns: 0,  // 连接池保持的最大空闲连接数，多余的空闲连接将被关闭 默认为0，不限制
		// 超时
		DialTimeout:  5 * time.Second, // 连接建立超时时间，默认5秒。
		ReadTimeout:  3 * time.Second, // 读超时，默认3秒， -1表示取消读超时
		WriteTimeout: 3 * time.Second, // 写超时，默认等于读超时
		PoolTimeout:  4 * time.Second, // 代表如果连接池所有连接都在使用中，等待获取连接时间，超时将返回错误 默认是 1秒+ReadTimeout
		// 命令执行失败时的重试策略
		MaxRetries:      3,                      // 命令执行失败时，最多重试多少次，默认为0即不重试
		MinRetryBackoff: 8 * time.Millisecond,   // 每次计算重试间隔时间的下限，默认8毫秒，-1表示取消间隔
		MaxRetryBackoff: 512 * time.Millisecond, // 每次计算重试间隔时间的上限，默认512毫秒，-1表示取消间隔
	})
	if err := client.Ping(ctx).Err(); err != nil {
		ZapLog.Fatal("redis连接失败", zap.Error(err))
	} else {
		client.AddHook(LogHook{})
		Rdb = append(Rdb, client)
		ZapLog.Info("redis连接成功")
	}
}
