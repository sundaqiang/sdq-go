package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

type Mongo struct {
	Url            string `toml:"url"`
	Username       string `toml:"username"`
	Password       string `toml:"password"`
	AppName        string `toml:"app-name"`
	PreferenceMode string `toml:"preference-mode"`
}

// InitMongo 初始化
func InitMongo(info *Mongo) {
	var err error
	ctx := context.Background()
	// 连接实例
	opts := options.Client().
		ApplyURI(info.Url).
		SetCompressors([]string{"zstd", "zlib", "snappy"}).
		SetConnectTimeout(10 * time.Second).  //TCP + TLS 握手超时
		SetMaxConnIdleTime(20 * time.Second). //空闲连接多久被回收，比业务峰值间隔大一点
		SetSocketTimeout(60 * time.Second).   // 单次读写最长等待，防止慢查询占住连接
		SetMaxPoolSize(200).                  // 最大连接数（一个进程内）
		SetMinPoolSize(30).                   // 预热，避免突发建连
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
	if info.AppName != "" {
		opts = opts.SetAppName(info.AppName)
	}

	if info.PreferenceMode != "" {
		// 可选：指定副本节点优先读
		readMode, _ := readpref.ModeFromString(info.PreferenceMode)
		readPref, _ := readpref.New(readMode)
		opts = opts.SetReadPreference(readPref)
	}

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
