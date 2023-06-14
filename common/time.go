package common

import (
	"strings"
	"time"
)

// String2Time 时间字符串转时间
func String2Time(times, format string, now bool) time.Time {
	if times == "" {
		if now {
			return time.Now()
		}
		return time.Unix(0, 0)
	}
	times = strings.TrimSpace(times)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	stamp, _ := time.ParseInLocation(format, times, loc)
	return stamp
}

// String2Timestamp 时间字符串转时间戳
func String2Timestamp(times, format string, now bool, milli bool) int64 {
	t := String2Time(times, format, now)
	if milli {
		return t.UnixMilli()
	}
	return t.Unix()
}

// Timestamp2Time 时间戳转时间
func Timestamp2Time(times int64, now bool) time.Time {
	if times == 0 {
		if now {
			return time.Now()
		}
		return time.Unix(0, 0)
	}
	if times > 9999999999 {
		times /= 1000
	}
	return time.Unix(times, 0)
}

// Timestamp2String 时间戳转时间字符串
func Timestamp2String(times int64, format string) string {
	if times == 0 {
		return "1970-01-01 08:00:00"
	}
	if times > 9999999999 {
		times /= 1000
	}
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return time.Unix(times, 0).Format(format)
}
