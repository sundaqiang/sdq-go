package common

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// RangeNum 取随机数
func RangeNum(min int, max int) int {
	if min == max {
		return min
	}
	rand.NewSource(time.Now().Unix())
	randNum := rand.Intn(max-min) + min
	return randNum
}

// TwoDecimal 保留2位小数
func TwoDecimal(num float64) float64 {
	num, _ = strconv.ParseFloat(fmt.Sprintf("%0.2f", num), 64)
	return num
}
