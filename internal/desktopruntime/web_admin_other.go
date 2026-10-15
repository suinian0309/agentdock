//go:build !windows

package desktopruntime

import (
	"context"
	"errors"
)

func platformReadWebAdminRuntime(context.Context, string) (WebAdminRuntime, error) {
	return WebAdminRuntime{}, errors.New("Web 管理面板当前只支持 Windows 桌面安装")
}

func platformReadWebAdminCredentials(string) (WebAdminCredentials, error) {
	return WebAdminCredentials{}, errors.New("Web 管理面板当前只支持 Windows 桌面安装")
}
