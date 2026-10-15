package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRedisLoggingUsesOnlyConfiguredFields(t *testing.T) {
	oldConfig, oldLog := config, ZapLog
	t.Cleanup(func() { config, ZapLog = oldConfig, oldLog })
	config = Config{}
	var output bytes.Buffer
	ZapLog = newLogTestLogger(&output)
	ctx := context.Background()
	cmd := redis.NewStringCmd(ctx, "get", "private-redis-key")
	cmd.SetVal("private-redis-result")
	called := 0
	hook := LogHook{Debug: true}.ProcessHook(func(context.Context, redis.Cmder) error {
		called++
		return nil
	})
	if err := hook(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	operationErr := errors.New("redis error: private-redis-error")
	cmd.SetErr(operationErr)
	if err := hook(ctx, cmd); err != operationErr {
		t.Fatal("command error did not reach caller unchanged")
	}
	if called != 2 || cmd.Val() != "private-redis-result" {
		t.Fatal("logging changed command execution or its result")
	}
	for _, expected := range []string{"private-redis-key", "private-redis-result", "private-redis-error", `"args"`, `"cmd"`, "redis_trace"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("missing unredacted debug content %q", expected)
		}
	}
	output.Reset()
	ZapLog = newLogTestLogger(&output, "args", "cmd", "error")
	if err := hook(ctx, cmd); err != operationErr {
		t.Fatal("command error changed on the second trace")
	}
	assertLogOmits(t, output.String(), "private-redis-key", "private-redis-result", "private-redis-error")
	if !strings.Contains(output.String(), redactedLogValue) {
		t.Fatal("configured Redis debug fields were not redacted")
	}
}

func TestRedisPipelineUsesOnlyConfiguredFields(t *testing.T) {
	oldConfig, oldLog := config, ZapLog
	t.Cleanup(func() { config, ZapLog = oldConfig, oldLog })
	config = Config{}
	var output bytes.Buffer
	ZapLog = newLogTestLogger(&output)
	ctx := context.Background()
	good := redis.NewStringCmd(ctx, "get", "private-pipeline-key")
	good.SetVal("private-pipeline-result")
	bad := redis.NewCmd(ctx, "hset", "private-hash-key", "password", "private-hash-value")
	operationErr := errors.New("pipeline error: private-pipeline-error")
	bad.SetErr(operationErr)
	called := false
	hook := LogHook{Debug: true}.ProcessPipelineHook(func(context.Context, []redis.Cmder) error {
		called = true
		return operationErr
	})
	if err := hook(ctx, []redis.Cmder{good, bad}); err != operationErr || !called {
		t.Fatal("pipeline did not execute or its error changed")
	}
	for _, value := range []string{"private-pipeline-key", "private-pipeline-result", "private-hash-key", "private-hash-value", "private-pipeline-error"} {
		if !strings.Contains(output.String(), value) {
			t.Errorf("empty sensitive-fields unexpectedly hid %q", value)
		}
	}
	if count := strings.Count(output.String(), "redis_pipeline_trace"); count != 3 {
		t.Fatalf("got %d pipeline trace entries, want 3", count)
	}
	output.Reset()
	ZapLog = newLogTestLogger(&output, "args", "cmd", "error")
	if err := hook(ctx, []redis.Cmder{good, bad}); err != operationErr {
		t.Fatal("pipeline error changed when redaction was configured")
	}
	assertLogOmits(t, output.String(), "private-pipeline-key", "private-pipeline-result", "private-hash-key", "private-hash-value", "private-pipeline-error")
}

func TestRedisDebugSwitchPreservesErrors(t *testing.T) {
	oldConfig, oldLog := config, ZapLog
	t.Cleanup(func() { config, ZapLog = oldConfig, oldLog })
	config = Config{}
	for _, debug := range []bool{false, true} {
		name := "disabled"
		if debug {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			core, entries := observer.New(zap.DebugLevel)
			ZapLog = zap.New(core)
			ctx := context.Background()
			good := redis.NewStringCmd(ctx, "get", "key")
			good.SetVal("value")
			missing := redis.NewStringCmd(ctx, "get", "missing")
			missing.SetErr(redis.Nil)
			bad := redis.NewStringCmd(ctx, "get", "bad")
			operationErr := errors.New("redis failure")
			bad.SetErr(operationErr)
			called := 0
			hook := LogHook{Debug: debug}.ProcessHook(func(context.Context, redis.Cmder) error {
				called++
				return nil
			})
			if err := hook(ctx, good); err != nil {
				t.Fatal(err)
			}
			if err := hook(ctx, missing); err != redis.Nil {
				t.Fatal("redis.Nil changed")
			}
			if err := hook(ctx, bad); err != operationErr {
				t.Fatal("command error changed")
			}
			pipeline := LogHook{Debug: debug}.ProcessPipelineHook(func(context.Context, []redis.Cmder) error {
				called++
				return operationErr
			})
			if err := pipeline(ctx, []redis.Cmder{good, missing, bad}); err != operationErr || called != 4 {
				t.Fatal("logging changed Redis execution or the pipeline error")
			}
			wantDebug := 0
			if debug {
				wantDebug = 4
			}
			if count := entries.FilterLevelExact(zapcore.DebugLevel).Len(); count != wantDebug {
				t.Fatalf("got %d debug entries, want %d", count, wantDebug)
			}
			if count := entries.FilterLevelExact(zapcore.ErrorLevel).Len(); count != 3 {
				t.Fatalf("got %d error entries, want 3 regardless of debug", count)
			}
		})
	}
}
