package service

import (
	"bytes"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/sundaqiang/sdq-go/common"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"time"
)

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(logger *lumberjack.Logger, stdOut bool) zapcore.WriteSyncer {
	var ws io.Writer
	if logger != nil {
		if stdOut {
			ws = io.MultiWriter(logger, os.Stdout)
		} else {
			ws = io.MultiWriter(logger, os.Stderr)
		}
	} else {
		if stdOut {
			ws = io.MultiWriter(os.Stdout)
		} else {
			ws = io.MultiWriter(os.Stderr)
		}
	}
	return zapcore.AddSync(ws)
}

// GinZapWithConfig returns a gin.HandlerFunc using configs
func GinZapWithConfig(logger *zap.Logger, conf *ginzap.Config, trace string) gin.HandlerFunc {
	skipPaths := make(map[string]bool, len(conf.SkipPaths))
	for _, path := range conf.SkipPaths {
		skipPaths[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		c.Next()

		if _, ok := skipPaths[path]; !ok {
			end := time.Now()
			latency := end.Sub(start)
			if conf.UTC {
				end = end.UTC()
			}

			fields := []zapcore.Field{
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.Reflect("header", c.Request.Header),
				zap.Duration("latency", latency),
			}
			// body
			if body != nil {
				if json.Valid(body) {
					fields = append(fields, zap.Reflect("body", common.Json2Map(body)))
				} else {
					fields = append(fields, zap.ByteString("body", body))
				}
			}
			// log request ID
			if requestID := c.Writer.Header().Get(common.KebabString(trace)); requestID != "" {
				fields = append(fields, zap.String(trace, requestID))
			}
			/*if conf.TimeFormat != "" {
				fields = append(fields, zap.String("time", end.Format(conf.TimeFormat)))
			}*/
			if conf.Context != nil {
				fields = append(fields, conf.Context(c)...)
			}

			if len(c.Errors) > 0 {
				// Append error field if this is an erroneous request.
				for _, e := range c.Errors.Errors() {
					logger.Error(e, fields...)
				}
			} else {
				logger.Info("Handler入口打印", fields...)
			}
		}
	}
}
