//go:build windows

package desktopruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func platformReadWebAdminRuntime(ctx context.Context, runtimeRoot string) (WebAdminRuntime, error) {
	runtime, err := loadTunnelRuntime(ctx, runtimeRoot)
	if err != nil {
		return WebAdminRuntime{}, err
	}
	namedURL, err := readTrimmedText(runtime.files.namedServerURL)
	if err != nil {
		return WebAdminRuntime{}, err
	}
	publicURL, err := readTrimmedText(runtime.files.serverURL)
	if err != nil {
		return WebAdminRuntime{}, err
	}
	tunnelTokenSaved := false
	if value, tokenErr := readProtectedText(runtime.files.token, tunnelTokenEntropy); tokenErr == nil {
		tunnelTokenSaved = strings.TrimSpace(value) != ""
	} else if !errors.Is(tokenErr, os.ErrNotExist) {
		return WebAdminRuntime{}, tokenErr
	}
	credentialsReady := true
	for _, credential := range []struct{ path, entropy string }{
		{filepath.Join(runtime.root, "auth-token.dpapi"), "agentdock.startup.v1"},
		{filepath.Join(runtime.root, "oauth-password.dpapi"), "agentdock.oauth.password.v1"},
	} {
		value, credentialErr := readProtectedText(credential.path, credential.entropy)
		if credentialErr != nil || strings.TrimSpace(value) == "" {
			credentialsReady = false
			break
		}
	}
	settings := runtime.settings
	return WebAdminRuntime{
		RuntimeRoot:      runtime.root,
		LocalMCPURL:      runtime.manifest.LocalMCPURL,
		PublicMCPURL:     strings.TrimRight(publicURL, "/") + optionalMCPSuffix(publicURL),
		NamedServerURL:   namedURL,
		TunnelTokenSaved: tunnelTokenSaved,
		CredentialsReady: credentialsReady,
		Settings: WebAdminSettings{
			Port: settings.Port, LogLevel: settings.LogLevel,
			OAuthAccessTokenTTL: settings.OAuthAccessTokenTTL, MCPAppsMode: settings.MCPAppsMode,
			BrowserEnabled: settings.BrowserEnabled, BrowserCDPURL: settings.BrowserCDPURL,
			BrowserReuseExistingCDP: settings.BrowserReuseExistingCDP,
			ACPEnabled:              settings.ACPEnabled, ACPProfiles: settings.ACPProfiles,
			ACPDefaultProfile: settings.ACPDefaultProfile,
		},
	}, nil
}

func optionalMCPSuffix(publicURL string) string {
	if strings.TrimSpace(publicURL) == "" {
		return ""
	}
	return "/mcp"
}

func platformReadWebAdminCredentials(runtimeRoot string) (WebAdminCredentials, error) {
	_, root, err := loadDesktopManifest(runtimeRoot)
	if err != nil {
		return WebAdminCredentials{}, err
	}
	bearer, err := readProtectedText(filepath.Join(root, "auth-token.dpapi"), "agentdock.startup.v1")
	if err != nil {
		return WebAdminCredentials{}, err
	}
	oauthPassword, err := readProtectedText(filepath.Join(root, "oauth-password.dpapi"), "agentdock.oauth.password.v1")
	if err != nil {
		return WebAdminCredentials{}, err
	}
	return WebAdminCredentials{BearerToken: bearer, OAuthPassword: oauthPassword}, nil
}
