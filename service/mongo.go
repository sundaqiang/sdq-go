package service

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

type Mongo struct {
	Url      string `toml:"url"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// InitMongo 初始化
func InitMongo(info *Mongo) {
	var err error
	ctx := context.Background()
	// 连接实例
	opts := options.Client().
		ApplyURI(info.Url).
		SetCompressors([]string{"zstd", "zlib", "snappy"}).
		SetAppName("platform_taobao").
		SetBSONOptions(&options.BSONOptions{
			UseJSONStructTags:       true,
			ErrorOnInlineDuplicates: true,
			IntMinSize:              true,
			NilMapAsEmpty:           true,
			NilSliceAsEmpty:         true,
			NilByteSliceAsEmpty:     true,
			OmitZeroStruct:          true,
			UseLocalTimeZone:        true,
		}).
		SetAuth(options.Credential{
			Username: info.Username,
			Password: info.Password,
		})

	Mdb, err = mongo.Connect(ctx, opts)
	if err != nil {
		ZapLog.Fatal("mongo连接失败", zap.Error(err))
		return
	}

	// 是否连接检测
	if err = Mdb.Ping(ctx, readpref.Primary()); err != nil {
		ZapLog.Fatal("mongo连接错误", zap.Error(err))
	} else {
		ZapLog.Info("mongo连接成功")
	}
}
