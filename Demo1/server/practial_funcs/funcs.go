package practial_funcs

//这里放一些实用性函数

type Unsigned interface {
	uint8 | uint16 | uint32 | uint64 | uint | uintptr
}

// 无符号数安全减法
func SafeSub[T Unsigned](x, n, min T) T {
	if x <= n || x-n < min {
		return min
	}
	return x - n
}

// 无符号数安全加法
func SafeAdd[T Unsigned](x, n, max T) T {
	if x+n > max {
		return max
	}
	return x + n
}
