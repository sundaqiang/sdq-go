package common

import (
	"bytes"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
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

func getLogWriter(filename string, stdOut bool) zapcore.WriteSyncer {
	var ws io.Writer
	if filename != "" {
		lumberJackLogger := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 30,
			MaxAge:     7,
			LocalTime:  true,
			Compress:   false,
		}
		if stdOut {
			ws = io.MultiWriter(lumberJackLogger, os.Stdout)
		} else {
			ws = io.MultiWriter(lumberJackLogger, os.Stderr)
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

// GinzapWithConfig returns a gin.HandlerFunc using configs
func GinzapWithConfig(logger *zap.Logger, conf *ginzap.Config) gin.HandlerFunc {
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
				zap.String("user-agent", c.Request.UserAgent()),
				zap.Duration("latency", latency),
			}
			// body
			if body != nil {
				if json.Valid(body) {
					fields = append(fields, zap.Reflect("body", Json2Map(body)))
				} else {
					fields = append(fields, zap.ByteString("body", body))
				}
			}
			// log request ID
			if requestID := c.Writer.Header().Get("X-Request-Id"); requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
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
