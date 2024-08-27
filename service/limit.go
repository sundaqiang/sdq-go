package service

import (
	"context"
	"errors"
	"github.com/go-redis/redis_rate/v10"
	"go.uber.org/zap"
)

func InitLimit(index int) {
	if index < 0 {
		return
	}
	if index < 0 || index >= len(Rdb) {
		ZapLog.Fatal("limit初始化失败", zap.Error(errors.New("索引超出Rdb")))
		return
	}

	ctx := context.Background()
	_ = Rdb[index].FlushDB(ctx).Err()

	Limiter = redis_rate.NewLimiter(Rdb[index])
	err := Limiter.Reset(ctx, "")
	if err != nil {
		ZapLog.Fatal("limit初始化错误", zap.Error(errors.New("ping失败")))
		return
	}
	ZapLog.Info("limit初始化成功")
}
