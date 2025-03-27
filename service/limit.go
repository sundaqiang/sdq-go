package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"strconv"
	"time"
)

const redisPrefix = "rate:"

// Copyright (c) 2017 Pavel Pravosud https://github.com/rwz/redis-gcra/blob/master/vendor/perform_gcra_ratelimit.lua
var allowN = redis.NewScript(`
-- this script has side-effects, so it requires replicate commands mode
redis.replicate_commands()

local rate_limit_key = KEYS[1]
local burst = ARGV[1]
local rate = ARGV[2]
local period = ARGV[3]
local cost = tonumber(ARGV[4])

local emission_interval = period / rate
local increment = emission_interval * cost
local burst_offset = emission_interval * burst

-- redis returns time as an array containing two integers: seconds of the epoch
-- time (10 digits) and microseconds (6 digits). for convenience we need to
-- convert them to a floating point number. the resulting number is 16 digits,
-- bordering on the limits of a 64-bit double-precision floating point number.
-- adjust the epoch to be relative to Jan 1, 2017 00:00:00 GMT to avoid floating
-- point problems. this approach is good until "now" is 2,483,228,799 (Wed, 09
-- Sep 2048 01:46:39 GMT), when the adjusted value is 16 digits.
local jan_1_2017 = 1483228800
local now = redis.call("TIME")
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local tat = redis.call("GET", rate_limit_key)

if not tat then
  tat = now
else
  tat = tonumber(tat)
end

tat = math.max(tat, now)

local new_tat = tat + increment
local allow_at = new_tat - burst_offset

local diff = now - allow_at
local remaining = diff / emission_interval

if remaining < 0 then
  local reset_after = tat - now
  local retry_after = diff * -1
  return {
    0, -- allowed
    0, -- remaining
    tostring(retry_after),
    tostring(reset_after),
  }
end

local reset_after = new_tat - now
if reset_after > 0 then
  redis.call("SET", rate_limit_key, new_tat, "EX", math.ceil(reset_after))
end
local retry_after = -1
return {cost, remaining, tostring(retry_after), tostring(reset_after)}
`)

var allowAtMost = redis.NewScript(`
-- this script has side-effects, so it requires replicate commands mode
redis.replicate_commands()

local rate_limit_key = KEYS[1]
local burst = ARGV[1]
local rate = ARGV[2]
local period = ARGV[3]
local cost = tonumber(ARGV[4])

local emission_interval = period / rate
local burst_offset = emission_interval * burst

-- redis returns time as an array containing two integers: seconds of the epoch
-- time (10 digits) and microseconds (6 digits). for convenience we need to
-- convert them to a floating point number. the resulting number is 16 digits,
-- bordering on the limits of a 64-bit double-precision floating point number.
-- adjust the epoch to be relative to Jan 1, 2017 00:00:00 GMT to avoid floating
-- point problems. this approach is good until "now" is 2,483,228,799 (Wed, 09
-- Sep 2048 01:46:39 GMT), when the adjusted value is 16 digits.
local jan_1_2017 = 1483228800
local now = redis.call("TIME")
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local tat = redis.call("GET", rate_limit_key)

if not tat then
  tat = now
else
  tat = tonumber(tat)
end

tat = math.max(tat, now)

local diff = now - (tat - burst_offset)
local remaining = diff / emission_interval

if remaining < 1 then
  local reset_after = tat - now
  local retry_after = emission_interval - diff
  return {
    0, -- allowed
    0, -- remaining
    tostring(retry_after),
    tostring(reset_after),
  }
end

if remaining < cost then
  cost = remaining
  remaining = 0
else
  remaining = remaining - cost
end

local increment = emission_interval * cost
local new_tat = tat + increment

local reset_after = new_tat - now
if reset_after > 0 then
  redis.call("SET", rate_limit_key, new_tat, "EX", math.ceil(reset_after))
end

return {
  cost,
  remaining,
  tostring(-1),
  tostring(reset_after),
}
`)

type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

// RedisRate controls how frequently events are allowed to happen.
type RedisRate struct {
	rdb *redis.Client
}

type Result struct {
	// Limit is the limit that was used to obtain this result.
	Limit Limit

	// Allowed is the number of events that may happen at time now.
	Allowed int

	// Remaining is the maximum number of requests that could be
	// permitted instantaneously for this key given the current
	// state. For example, if a rate limiter allows 10 requests per
	// second and has already received 6 requests for this key this
	// second, Remaining would be 4.
	Remaining int

	// RetryAfter is the time until the next request will be permitted.
	// It should be -1 unless the rate limit has been exceeded.
	RetryAfter time.Duration

	// ResetAfter is the time until the RateLimiter returns to its
	// initial state for a given key. For example, if a rate limiter
	// manages requests per second and received one request 200ms ago,
	// Reset would return 800ms. You can also think of this as the time
	// until Limit and Remaining will be equal.
	ResetAfter time.Duration
}

func InitLimit(index int) {
	if index < 0 || index >= len(Rdb) {
		ZapLog.Fatal("limit初始化失败", zap.Error(errors.New("索引超出Rdb")))
		return
	}

	ctx := context.Background()
	Limiter = &RedisRate{
		rdb: Rdb[index],
	}
	err := Limiter.Reset(ctx, "")
	if err != nil {
		ZapLog.Fatal("limit初始化错误", zap.Error(errors.New("ping失败")))
		return
	}
	allowN.Load(ctx, Limiter.rdb)
	allowAtMost.Load(ctx, Limiter.rdb)
	ZapLog.Info("limit初始化成功")
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}

func fmtDur(d time.Duration) string {
	switch d {
	case time.Second:
		return "s"
	case time.Minute:
		return "m"
	case time.Hour:
		return "h"
	}
	return d.String()
}

func PerSecond(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Second,
		Burst:  rate,
	}
}

func PerMinute(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Minute,
		Burst:  rate,
	}
}

func PerHour(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Hour,
		Burst:  rate,
	}
}

func (l Limit) String() string {
	return fmt.Sprintf("%d req/%s (burst %d)", l.Rate, fmtDur(l.Period), l.Burst)
}

func (l Limit) IsZero() bool {
	return l == Limit{}
}

// Allow is a shortcut for AllowN(ctx, key, limit, 1).
func (l *RedisRate) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return l.AllowN(ctx, key, limit, 1)
}

// AllowN reports whether n events may happen at time now.
func (l *RedisRate) AllowN(ctx context.Context, key string, limit Limit, n int) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowN.Run(ctx, l.rdb, []string{redisPrefix + key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

// AllowAtMost reports whether at most n events may happen at time now. It returns number of allowed events that is less than or equal to n.
func (l *RedisRate) AllowAtMost(ctx context.Context, key string, limit Limit, n int) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowAtMost.Run(ctx, l.rdb, []string{redisPrefix + key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

// Reset gets a key and reset all limitations and previous usages
func (l *RedisRate) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, redisPrefix+key).Err()
}
