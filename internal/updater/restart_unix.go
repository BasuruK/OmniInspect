//go:build !windows

package updater

import "syscall"

// detachSysProcAttr returns the platform-specific SysProcAttr needed to make
// the spawned child survive its parent's exit. On Unix systems, Setsid starts
// the child in a new session so signals delivered to the parent's process group
// (e.g. SIGHUP from a closing terminal) are not forwarded to the new instance.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
