//go:build !darwin

package main

// 非 macOS 平台保持空实现，避免引入平台相关依赖。
func (a *App) startStatusBar() {}

func (a *App) stopStatusBar() {}
