package sdqgo

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
