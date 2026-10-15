//go:build windows

package webadmin

import "os/exec"

// OpenURL opens the web control panel in the user's default browser.
func OpenURL(target string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", target).Start()
}
