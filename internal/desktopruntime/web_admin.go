package desktopruntime

import (
	"context"

	agentconfig "github.com/uvwt/agentdock/internal/config"
)

// WebAdminSettings is the safe, non-secret settings view consumed by the
// loopback-only web control panel.
type WebAdminSettings struct {
	Port                    int                      `json:"port"`
	LogLevel                string                   `json:"log_level"`
	OAuthAccessTokenTTL     string                   `json:"oauth_access_token_ttl,omitempty"`
	MCPAppsMode             agentconfig.MCPAppsMode  `json:"mcp_apps_mode"`
	BrowserEnabled          bool                     `json:"browser_enabled"`
	BrowserCDPURL           string                   `json:"browser_cdp_url,omitempty"`
	BrowserReuseExistingCDP bool                     `json:"browser_reuse_existing_cdp"`
	ACPEnabled              bool                     `json:"acp_enabled"`
	ACPProfiles             []agentconfig.ACPProfile `json:"acp_profiles,omitempty"`
	ACPDefaultProfile       string                   `json:"acp_default_profile,omitempty"`
}

type WebAdminRuntime struct {
	RuntimeRoot      string           `json:"runtime_root"`
	LocalMCPURL      string           `json:"local_mcp_url"`
	PublicMCPURL     string           `json:"public_mcp_url,omitempty"`
	NamedServerURL   string           `json:"named_server_url,omitempty"`
	TunnelTokenSaved bool             `json:"tunnel_token_saved"`
	CredentialsReady bool             `json:"credentials_ready"`
	Settings         WebAdminSettings `json:"settings"`
}

type WebAdminCredentials struct {
	BearerToken   string `json:"bearer_token"`
	OAuthPassword string `json:"oauth_password"`
}

func ReadWebAdminRuntime(ctx context.Context, runtimeRoot string) (WebAdminRuntime, error) {
	return platformReadWebAdminRuntime(ctx, runtimeRoot)
}

func ReadWebAdminCredentials(runtimeRoot string) (WebAdminCredentials, error) {
	return platformReadWebAdminCredentials(runtimeRoot)
}
