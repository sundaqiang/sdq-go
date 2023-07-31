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

/*
TimeStart4Day 取某天零点

	now := time.Now()
	// 取今天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month(), now.Day()))
	// 取前一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month(), now.Day()-1))
	// 取后一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month(), now.Day()+1))
	// 取上个月第一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month()-1, 1))
	// 取上个月最后一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month()-1, 1).AddDate(0, 1, -1))
	// 取下个月第一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month()+1, 1))
	// 取下个月最后一天
	fmt.Println(common.TimeStart4Day(now.Year(), now.Month()+1, 1).AddDate(0, 1, -1))
*/
func TimeStart4Day(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}
