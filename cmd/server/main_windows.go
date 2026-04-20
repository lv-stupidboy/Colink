// +build windows

package main

import (
	"golang.org/x/sys/windows"
)

func init() {
	// Windows 平台设置 UTF-8 代码页，解决控制台中文乱码
	// CP_UTF8 = 65001
	windows.SetConsoleOutputCP(65001)
	windows.SetConsoleCP(65001)
}