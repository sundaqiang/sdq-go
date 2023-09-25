package service

import (
	"errors"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/sundaqiang/sdq-go/common"
	"gopkg.in/natefinch/lumberjack.v2"
	"strings"
)

type Config struct {
	Server   *Server   `toml:"server"`
	Log      *Log      `toml:"log"`
	Database *Database `toml:"database"`
	Other    *Other    `toml:"other"`
}

type Database struct {
	Gorm  Gorm  `toml:"gorm"`
	Redis Redis `toml:"redis"`
}

type Server struct {
	Name string `toml:"name"`
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type Log struct {
	Path       string `toml:"path"`
	File       string `toml:"file"`
	MaxSize    int    `toml:"max-size"`
	MaxBackups int    `toml:"max-backups"`
	MaxAge     int    `toml:"max-age"`
	LocalTime  bool   `toml:"local-time"`
	Compress   bool   `toml:"compress"`
}

type Other struct {
	FastHttp  bool   `toml:"fast-http"`
	ProxyAddr string `toml:"proxy-addr"`
	Cron      bool   `toml:"cron"`
	CronAsync bool   `toml:"cron-async"`
}

var k = koanf.New(".")

func InitConfig(filePath, prefix string, conf any) error {
	if err := k.Load(file.Provider(filePath), toml.Parser()); err != nil {
		return err
	}
	if err := k.Load(env.ProviderWithValue(prefix, ".", func(s string, v string) (string, interface{}) {
		key := strings.Replace(strings.ToLower(strings.TrimPrefix(s, prefix)), "_", ".", -1)
		if strings.Contains(v, " ") {
			return key, strings.Split(v, " ")
		}
		return key, v
	}), nil); err != nil {
		return err
	}
	err := k.UnmarshalWithConf("", conf, koanf.UnmarshalConf{Tag: "toml"})
	if err != nil {
		return err
	}
	var config Config
	common.StructAssign(&config, conf, "toml")
	if config.Log != nil && !common.InitLogger(&lumberjack.Logger{
		Filename:   config.Log.Path + "/" + config.Log.File,
		MaxSize:    config.Log.MaxSize,
		MaxBackups: config.Log.MaxBackups,
		MaxAge:     config.Log.MaxAge,
		LocalTime:  config.Log.LocalTime,
		Compress:   config.Log.Compress,
	}) {
		return errors.New("初始化日志失败")
	}
	if config.Database != nil {
		if config.Database.Gorm.Type != "" {
			InitGORM(&config.Database.Gorm)
		}
		if config.Database.Redis.Addr != "" {
			InitRdb(&config.Database.Redis)
		}
	}
	if config.Other != nil {
		if config.Other.FastHttp {
			common.InitFastHttp(config.Other.ProxyAddr)
		}
		if config.Other.Cron {
			InitGoCron(config.Other.CronAsync)
		}
	}
	return nil
}
