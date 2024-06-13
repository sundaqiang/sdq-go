package service

import (
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
	Limiter = redis_rate.NewLimiter(Rdb[index])
}
