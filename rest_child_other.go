//go:build !windows

package main

import "syscall"

// childSysProcAttr is nothing to add on the platforms that give a child no
// console of its own.
func childSysProcAttr() *syscall.SysProcAttr { return nil }
