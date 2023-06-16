package common

import "sort"

type KeyFunc[T any] func(T) string

/*
UniqueAndMerge 合并2个切片并去重

	func(s string) string {
		return s
	}
*/
func UniqueAndMerge[T any](slice1, slice2 []T, resultPtr *[]T, keyFunc KeyFunc[T]) {
	seen := make(map[string]struct{})
	mergedSlice := append(slice1, slice2...)
	for _, item := range mergedSlice {
		key := keyFunc(item)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			*resultPtr = append(*resultPtr, item)
		}
	}
}

// IntInSlice 判断int是否在切片内
func IntInSlice(slice []int, val int) bool {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	i := sort.Search(len(slice), func(i int) bool {
		return slice[i] >= val
	})
	return i < len(slice) && slice[i] == val
}

// Int64InSlice 判断int64是否在切片内
func Int64InSlice(slice []int64, val int64) bool {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	i := sort.Search(len(slice), func(i int) bool {
		return slice[i] >= val
	})
	return i < len(slice) && slice[i] == val
}

// StringInSlice 判断string是否在切片内
func StringInSlice(slice []string, val string) bool {
	sort.Strings(slice)
	i := sort.SearchStrings(slice, val)
	return i < len(slice) && slice[i] == val
}

// DelIntInSlice 删除切片
func DelIntInSlice(slice []int, element int) []int {
	slow := 0
	for fast := 0; fast < len(slice); fast++ {
		if slice[fast] != element {
			slice[slow] = slice[fast]
			slow++
		}
	}
	slice = slice[:slow]
	return slice
}

// DelInt64InSlice 删除切片
func DelInt64InSlice(slice []int64, element int64) []int64 {
	slow := 0
	for fast := 0; fast < len(slice); fast++ {
		if slice[fast] != element {
			slice[slow] = slice[fast]
			slow++
		}
	}
	slice = slice[:slow]
	return slice
}

// DelStringInSlice 删除切片
func DelStringInSlice(slice []string, element string) []string {
	slow := 0
	for fast := 0; fast < len(slice); fast++ {
		if slice[fast] != element {
			slice[slow] = slice[fast]
			slow++
		}
	}
	slice = slice[:slow]
	return slice
}
