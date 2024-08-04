package service

import (
	"errors"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sony/sonyflake"
	"github.com/sundaqiang/sdq-go/common"
	"gopkg.in/natefinch/lumberjack.v2"
	"strings"
	"time"
)

type Config struct {
	Server   *Server   `toml:"server"`
	Log      *Log      `toml:"log"`
	Database *Database `toml:"database"`
	Cache    *Cache    `toml:"cache"`
	Other    *Other    `toml:"other"`
}

type Database struct {
	Gorm  Gorm    `toml:"gorm"`
	Redis []Redis `toml:"redis"`
	Mongo Mongo   `toml:"mongo"`
}

type Server struct {
	Name  string `toml:"name"`
	Host  string `toml:"host"`
	Port  int    `toml:"port"`
	Trace string `toml:"trace"`
	Trans bool   `toml:"trans"`
}

type Log struct {
	Path       string `toml:"path"`
	File       string `toml:"file"`
	MaxSize    int    `toml:"max-size"`
	MaxBackups int    `toml:"max-backups"`
	MaxAge     int    `toml:"max-age"`
	LocalTime  bool   `toml:"local-time"`
	Compress   bool   `toml:"compress"`
	CallerSkip int    `toml:"caller-skip"`
}

type Cache struct {
	BucketCnt  uint16 `toml:"bucket-cnt"`
	CapOne     uint16 `toml:"cap-one"`
	CapTwo     uint16 `toml:"cap-two"`
	Rdb        int    `toml:"rdb"`
	Size       int    `toml:"size"`
	Expiration int64  `toml:"expiration"`
}

type Other struct {
	FastHttp  bool   `toml:"fast-http"`
	ProxyAddr string `toml:"proxy-addr"`
	Cron      bool   `toml:"cron"`
	CronAsync bool   `toml:"cron-async"`
	SonyFlake int64  `toml:"sony-flake"`
	IpdbPath  string `toml:"ipdb-path"`
	IpdbCorn  int64  `toml:"ipdb-corn"`
	Limiter   int    `toml:"limiter"`
}

var k = koanf.New(".")

func InitConfig(filePath, prefix string, conf any) error {
	if filePath != "" {
		if err := k.Load(file.Provider(filePath), toml.Parser()); err != nil {
			return err
		}
	}
	if prefix != "" {
		if err := k.Load(env.ProviderWithValue(prefix, ".", func(s string, v string) (string, interface{}) {
			key := strings.Replace(strings.ToLower(strings.TrimPrefix(s, prefix)), "_", ".", -1)
			if strings.Contains(v, " ") {
				return key, strings.Split(v, " ")
			}
			return key, v
		}), nil); err != nil {
			return err
		}
	}
	err := k.UnmarshalWithConf("", conf, koanf.UnmarshalConf{Tag: "toml"})
	if err != nil {
		return err
	}
	common.StructAssign(&config, conf, "toml")
	if config.Log != nil && !InitLogger(&lumberjack.Logger{
		Filename:   config.Log.Path + "/" + config.Log.File,
		MaxSize:    config.Log.MaxSize,
		MaxBackups: config.Log.MaxBackups,
		MaxAge:     config.Log.MaxAge,
		LocalTime:  config.Log.LocalTime,
		Compress:   config.Log.Compress,
	}, config.Log.CallerSkip) {
		return errors.New("初始化日志失败")
	}
	if config.Database != nil {
		if config.Database.Gorm.Type != "" {
			InitGORM(&config.Database.Gorm)
		}
		if len(config.Database.Redis) > 0 {
			InitRdb(&config.Database.Redis)
		}
		if config.Database.Mongo.Url != "" {
			InitMongo(&config.Database.Mongo)
		}
	}
	if config.Other != nil {
		if config.Other.FastHttp {
			InitFastHttp(config.Other.ProxyAddr)
		}
		if config.Other.Cron {
			InitGoCron(config.Other.CronAsync)
		}
		if config.Other.SonyFlake > 0 {
			InitSonyFlake(sonyflake.Settings{
				StartTime: common.Timestamp2Time(config.Other.SonyFlake, true),
			})
		}
		if config.Other.IpdbPath != "" && config.Other.IpdbCorn > 0 {
			InitIpdb(config.Other.IpdbPath, config.Other.IpdbCorn)
		}
		if config.Other.Limiter > -1 && len(Rdb) > 0 && len(Rdb) > config.Other.Limiter {
			InitLimit(config.Other.Limiter)
		}
	}
	if config.Cache != nil &&
		config.Cache.BucketCnt > 0 &&
		config.Cache.CapOne > 0 &&
		config.Cache.Expiration > 0 {
		var rdb *redis.Client
		if config.Cache.Rdb > -1 && len(Rdb) > 0 && len(Rdb) > config.Cache.Rdb {
			rdb = Rdb[config.Cache.Rdb]
		}
		InitLocalCache(
			config.Cache.BucketCnt,
			config.Cache.CapOne,
			config.Cache.CapTwo,
			rdb,
			config.Cache.Size,
			time.Duration(config.Cache.Expiration)*time.Second)
	}
	return nil
}
