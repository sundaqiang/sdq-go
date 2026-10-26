package service

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sundaqiang/sdq-go/common"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	var sensitiveFields []string
	if config.Log != nil {
		sensitiveFields = config.Log.SensitiveFields
	}
	return newRedactingEncoder(zapcore.NewJSONEncoder(encoderConfig), sensitiveFields)
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

// httpLogBodyLimit returns the configured body capture limit, defaulting to
// 8KB when unset or invalid.
func httpLogBodyLimit() int {
	if config.Log != nil && config.Log.HTTPLog != nil && config.Log.HTTPLog.BodyLimit > 0 {
		return config.Log.HTTPLog.BodyLimit
	}
	return 8 * 1024
}

// httpLogEnabled reports whether one of the HTTP log switches is configured on.
func httpLogEnabled(request bool) bool {
	if config.Log == nil || config.Log.HTTPLog == nil {
		return false
	}
	if request {
		return config.Log.HTTPLog.RequestEnabled
	}
	return config.Log.HTTPLog.ResponseEnabled
}

// requestTrace returns the configured request ID header maintained by the
// requestid middleware. Both the field name and value follow [server] trace,
// and handler changes to response headers cannot break log correlation.
func requestTrace(c *gin.Context, trace string) string {
	return c.GetHeader(common.KebabString(trace))
}

// logBodyBuffers recycles response capture buffers to reduce per-request
// allocations; buffers are reused only while they stay small.
var logBodyBuffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// captureRequestHead reads at most limit+1 bytes. It returns the complete
// captured prefix for replay and the bounded prefix for logging. Keeping the
// extra byte in replay prevents truncation detection from dropping body data.
func captureRequestHead(body io.Reader, limit int) (replay, logged []byte, truncated bool) {
	replay, _ = io.ReadAll(io.LimitReader(body, int64(limit)+1))
	logged = replay
	if len(replay) > limit {
		logged = replay[:limit]
		truncated = true
	}
	return replay, logged, truncated
}

// responseBodyWriter captures the response body for logging while keeping the
// original gin.ResponseWriter behaviour intact. Capture stops at limit so
// large responses do not grow the buffer unbounded.
type responseBodyWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	limit     int
	truncated bool
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	if remaining := w.limit - w.body.Len(); remaining > 0 {
		if len(b) > remaining {
			w.body.Write(b[:remaining])
			w.truncated = true
		} else {
			w.body.Write(b)
		}
	} else if len(b) > 0 {
		w.truncated = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseBodyWriter) WriteString(s string) (int, error) {
	if remaining := w.limit - w.body.Len(); remaining > 0 {
		if len(s) > remaining {
			w.body.WriteString(s[:remaining])
			w.truncated = true
		} else {
			w.body.WriteString(s)
		}
	} else if len(s) > 0 {
		w.truncated = true
	}
	return w.ResponseWriter.WriteString(s)
}

// appendJSONOrBytes logs structured JSON when the payload is a JSON object and
// falls back to raw bytes otherwise, parsing the payload only once.
func appendJSONOrBytes(fields []zapcore.Field, name string, data []byte) []zapcore.Field {
	if m := common.Json2Map(data); m != nil {
		return append(fields, zap.Reflect(name, m))
	}
	return append(fields, zap.ByteString(name, data))
}

// skipSet converts skip paths into a lookup set.
func skipSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[path] = true
	}
	return set
}

// isBrokenPipe reports whether a panic was caused by a disconnected client.
func isBrokenPipe(value any) bool {
	err, ok := value.(error)
	if !ok {
		return false
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var syscallErr *os.SyscallError
	if !errors.As(opErr.Err, &syscallErr) {
		return false
	}
	message := strings.ToLower(syscallErr.Error())
	return strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "connection reset by peer")
}

// GinRecoveryLog recovers handler panics, records them through zap and returns
// HTTP 500. Recovery logs use the same configured trace field as the HTTP
// access logs so panic investigation can follow one request across all logs.
func GinRecoveryLog(logger *zap.Logger, trace string, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			brokenPipe := isBrokenPipe(recovered)
			fields := []zapcore.Field{
				zap.Any("error", recovered),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("query", c.Request.URL.RawQuery),
				zap.String("ip", c.ClientIP()),
				zap.Reflect("header", c.Request.Header),
			}
			if tid := requestTrace(c, trace); tid != "" {
				fields = append(fields, zap.String(trace, tid))
			}
			if brokenPipe {
				fields = append(fields, zap.Bool("broken_pipe", true))
			} else if c.Writer.Written() {
				fields = append(fields,
					zap.Int("status", c.Writer.Status()),
					zap.Bool("response_started", true),
				)
			} else {
				fields = append(fields, zap.Int("status", http.StatusInternalServerError))
			}
			if stack && !brokenPipe {
				fields = append(fields, zap.ByteString("stack", debug.Stack()))
			}
			logger.Error("Handler发生panic", fields...)

			if brokenPipe {
				if err, ok := recovered.(error); ok {
					_ = c.Error(err)
				}
				c.Abort()
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
		}()
		c.Next()
	}
}

// GinRequestLog logs incoming requests when [log.http] request-enabled is on.
// When off it returns a passthrough middleware that skips body reading,
// parsing and logging entirely.
func GinRequestLog(logger *zap.Logger, trace string, skipPaths []string) gin.HandlerFunc {
	if !httpLogEnabled(true) {
		return func(c *gin.Context) { c.Next() }
	}
	limit := httpLogBodyLimit()
	skip := skipSet(skipPaths)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if skip[path] {
			c.Next()
			return
		}
		fields := []zapcore.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.Reflect("header", c.Request.Header),
		}
		if c.Request.Body != nil {
			replay, logged, truncated := captureRequestHead(c.Request.Body, limit)
			c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(replay), c.Request.Body))
			if len(logged) > 0 {
				fields = appendJSONOrBytes(fields, "body", logged)
			}
			if truncated {
				fields = append(fields, zap.Bool("body_truncated", true))
			}
		}
		if tid := requestTrace(c, trace); tid != "" {
			fields = append(fields, zap.String(trace, tid))
		}
		logger.Info("Handler请求开始", fields...)
		c.Next()
	}
}

// GinResponseLog logs completed requests when [log.http] response-enabled is
// on. When off it returns a passthrough middleware that never wraps the
// response writer or captures response bytes.
func GinResponseLog(logger *zap.Logger, trace string, skipPaths []string) gin.HandlerFunc {
	if !httpLogEnabled(false) {
		return func(c *gin.Context) { c.Next() }
	}
	limit := httpLogBodyLimit()
	skip := skipSet(skipPaths)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if skip[path] {
			c.Next()
			return
		}
		start := time.Now()
		buf := logBodyBuffers.Get().(*bytes.Buffer)
		buf.Reset()
		originalWriter := c.Writer
		resWriter := &responseBodyWriter{ResponseWriter: originalWriter, body: buf, limit: limit}
		defer func() {
			c.Writer = originalWriter
			buf.Reset()
			if buf.Cap() <= 1<<20 {
				logBodyBuffers.Put(buf)
			}
		}()
		c.Writer = resWriter
		c.Next()

		fields := []zapcore.Field{
			zap.Int("status", resWriter.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Duration("latency", time.Since(start)),
		}
		if resWriter.body.Len() > 0 {
			fields = appendJSONOrBytes(fields, "response", resWriter.body.Bytes())
		}
		if resWriter.truncated {
			fields = append(fields, zap.Bool("response_truncated", true))
		}
		if tid := requestTrace(c, trace); tid != "" {
			fields = append(fields, zap.String(trace, tid))
		}
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.Error(e, fields...)
			}
		} else {
			logger.Info("Handler请求完成", fields...)
		}
	}
}
