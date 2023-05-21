package service

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/sundaqiang/sdq-go/common"
	"time"

	"go.uber.org/zap"
)

type RdbInfo struct {
	Addr     string
	Password string
	DB       int
}

func (r *RdbInfo) initRedis() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		// 连接信息
		Network:  "tcp",      // 网络类型，tcp or unix，默认tcp
		Addr:     r.Addr,     // 主机名+冒号+端口，默认localhost:6379
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
		common.ZapLog.Fatal("redis连接失败", zap.Error(err))
	} else {
		Rdb = append(Rdb, client)
		common.ZapLog.Info("redis连接成功")
	}
}
