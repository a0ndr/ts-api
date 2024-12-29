package assert

func NotNil(obj any, format string, args ...interface{}) {
	if obj == nil {
		panic(formatMsg(format, args...))
	}
}

func IsNil(obj any, format string, args ...interface{}) {
	if obj != nil {
		panic(formatMsg(format, args...))
	}
}
