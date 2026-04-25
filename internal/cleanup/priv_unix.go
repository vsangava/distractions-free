//go:build !windows

package cleanup

import "os"

// IsPrivileged reports whether the process is running as root.
func IsPrivileged() bool {
	return os.Geteuid() == 0
}
