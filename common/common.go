package common

import (
	"github.com/bytedance/sonic"
	"github.com/orca-zhang/ecache"
	"github.com/orca-zhang/ecache/dist"
	"github.com/redis/go-redis/v9"
	"github.com/sony/sonyflake"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
	"time"
)

var (
	FastHttpClient *fasthttp.Client
	SonyFlake      *sonyflake.Sonyflake
	ZapLog         *zap.Logger
	LRUCache       *ecache.Cache
	fileTypeMap    sync.Map
	json           = sonic.ConfigStd
)

func init() {
	fileTypeMap.Store("ffd8ffe000104a464946", "jpg")  // JPEG (jpg)
	fileTypeMap.Store("89504e470d0a1a0a0000", "png")  // PNG (png)
	fileTypeMap.Store("47494638396126026f01", "gif")  // GIF (gif)
	fileTypeMap.Store("49492a00227105008037", "tif")  // TIFF (tif)
	fileTypeMap.Store("424d228c010000000000", "bmp")  // 16色位图(bmp)
	fileTypeMap.Store("424d8240090000000000", "bmp")  // 24位位图(bmp)
	fileTypeMap.Store("424d8e1b030000000000", "bmp")  // 256色位图(bmp)
	fileTypeMap.Store("41433130313500000000", "dwg")  // CAD (dwg)
	fileTypeMap.Store("3c21444f435459504520", "html") // HTML (html)   3c68746d6c3e0  3c68746d6c3e0
	fileTypeMap.Store("3c68746d6c3e0", "html")        // HTML (html)   3c68746d6c3e0  3c68746d6c3e0
	fileTypeMap.Store("3c21646f637479706520", "htm")  // HTM (htm)
	fileTypeMap.Store("48544d4c207b0d0a0942", "css")  // css
	fileTypeMap.Store("696b2e71623d696b2e71", "js")   // js
	fileTypeMap.Store("7b5c727466315c616e73", "rtf")  // Rich Text Format (rtf)
	fileTypeMap.Store("38425053000100000000", "psd")  // Photoshop (psd)
	fileTypeMap.Store("46726f6d3a203d3f6762", "eml")  // Email [Outlook Express 6] (eml)
	fileTypeMap.Store("d0cf11e0a1b11ae10000", "doc")  // MS Excel 注意：word、msi 和 excel的文件头一样
	fileTypeMap.Store("d0cf11e0a1b11ae10000", "vsd")  // Visio 绘图
	fileTypeMap.Store("5374616E64617264204A", "mdb")  // MS Access (mdb)
	fileTypeMap.Store("252150532D41646F6265", "ps")
	fileTypeMap.Store("255044462d312e350d0a", "pdf")  // Adobe Acrobat (pdf)
	fileTypeMap.Store("2e524d46000000120001", "rmvb") // rmvb/rm相同
	fileTypeMap.Store("464c5601050000000900", "flv")  // flv与f4v相同
	fileTypeMap.Store("00000020667479706d70", "mp4")
	fileTypeMap.Store("49443303000000002176", "mp3")
	fileTypeMap.Store("000001ba210001000180", "mpg") //
	fileTypeMap.Store("3026b2758e66cf11a6d9", "wmv") // wmv与asf相同
	fileTypeMap.Store("52494646e27807005741", "wav") // Wave (wav)
	fileTypeMap.Store("52494646d07d60074156", "avi")
	fileTypeMap.Store("4d546864000000060001", "mid") // MIDI (mid)
	fileTypeMap.Store("504b0304140000000800", "zip")
	fileTypeMap.Store("526172211a0700cf9073", "rar")
	fileTypeMap.Store("235468697320636f6e66", "ini")
	fileTypeMap.Store("504b03040a0000000000", "jar")
	fileTypeMap.Store("4d5a9000030000000400", "exe")        // 可执行文件
	fileTypeMap.Store("3c25402070616765206c", "jsp")        // jsp文件
	fileTypeMap.Store("4d616e69666573742d56", "mf")         // MF文件
	fileTypeMap.Store("3c3f786d6c2076657273", "xml")        // xml文件
	fileTypeMap.Store("494e5345525420494e54", "sql")        // xml文件
	fileTypeMap.Store("7061636b616765207765", "java")       // java文件
	fileTypeMap.Store("406563686f206f66660d", "bat")        // bat文件
	fileTypeMap.Store("1f8b0800000000000000", "gz")         // gz文件
	fileTypeMap.Store("6c6f67346a2e726f6f74", "properties") // bat文件
	fileTypeMap.Store("cafebabe0000002e0041", "class")      // bat文件
	fileTypeMap.Store("49545346030000006000", "chm")        // bat文件
	fileTypeMap.Store("04000000010000001300", "mxp")        // bat文件
	fileTypeMap.Store("504b0304140006000800", "docx")       // docx文件
	fileTypeMap.Store("d0cf11e0a1b11ae10000", "wps")        // WPS文字wps、表格et、演示dps都是一样的
	fileTypeMap.Store("6431303a637265617465", "torrent")
	fileTypeMap.Store("6D6F6F76", "mov")         // Quicktime (mov)
	fileTypeMap.Store("FF575043", "wpd")         // WordPerfect (wpd)
	fileTypeMap.Store("CFAD12FEC5FD746F", "dbx") // Outlook Express (dbx)
	fileTypeMap.Store("2142444E", "pst")         // Outlook (pst)
	fileTypeMap.Store("AC9EBD8F", "qdf")         // Quicken (qdf)
	fileTypeMap.Store("E3828596", "pwl")         // Windows Password (pwl)
	fileTypeMap.Store("2E7261FD", "ram")         // Real Audio (ram)
}

// InitLogger 必须
func InitLogger(logsPath, serverName string) bool {
	if logsPath == "" {
		logsPath = "logs/"
	}
	if serverName == "" {
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
	debugWriter := getLogWriter(logsPath+serverName+"_debug.log", true)
	infoWriter := getLogWriter(logsPath+serverName+"_info.log", true)
	warnWriter := getLogWriter(logsPath+serverName+"_warn.log", false)
	errorWriter := getLogWriter(logsPath+serverName+"_error.log", false)
	encoder := getEncoder()
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(debugWriter), debugLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), infoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), warnLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), errorLevel),
	)
	ZapLog = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
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
func InitSonyFlake() {
	SonyFlake = sonyflake.NewSonyflake(sonyflake.Settings{})
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
