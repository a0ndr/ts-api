package assert

import "fmt"

func formatMsg(format string, args ...interface{}) string {
	return fmt.Sprintf("assertion failed: "+format, args...)
}
