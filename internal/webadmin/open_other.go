//go:build !windows

package webadmin

import "errors"

// OpenURL opens the web control panel in the user's default browser.
func OpenURL(string) error { return errors.New("当前平台不支持自动打开") }
