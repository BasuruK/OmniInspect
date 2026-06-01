//go:build windows

package updater

import "syscall"

// createNewProcessGroup is the Windows CREATE_NEW_PROCESS_GROUP creation flag
// (0x00000200). It detaches the child from the parent's process group so that
// Ctrl+Break / console-close signals delivered to the parent are not forwarded
// to the new instance.
const createNewProcessGroup = 0x00000200

// detachSysProcAttr returns the platform-specific SysProcAttr needed to make
// the spawned child survive its parent's exit. On Windows this is the
// CREATE_NEW_PROCESS_GROUP creation flag.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}
