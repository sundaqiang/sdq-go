package service

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Mongo struct {
	Url      string `toml:"url"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

var MonC *mongo.Client

// InitMongo 初始化
func InitMongo(info *Mongo) error {
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

	MonC, err = mongo.Connect(ctx, opts)
	if err != nil {
		return err
	}

	// 是否连接检测
	return MonC.Ping(ctx, readpref.Primary())
}
