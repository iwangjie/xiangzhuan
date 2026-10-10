//go:build windows

package main

import "syscall"

// childSysProcAttr keeps a rest child's console out of sight: it is a
// windowed process started by the tray, not something the user asked for.
func childSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
