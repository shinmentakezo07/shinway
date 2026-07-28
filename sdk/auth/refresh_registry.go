package auth

import (
	"time"

	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
)

func init() {
	registerRefreshLead("codex", func() Authenticator { return NewCodexAuthenticator() })
	registerRefreshLead("claude", func() Authenticator { return NewClaudeAuthenticator() })
	registerRefreshLead("antigravity", func() Authenticator { return NewAntigravityAuthenticator() })
	registerRefreshLead("kimi", func() Authenticator { return NewKimiAuthenticator() })
	registerRefreshLead("xai", func() Authenticator { return NewXAIAuthenticator() })
}

func registerRefreshLead(provider string, factory func() Authenticator) {
	shinwayauth.RegisterRefreshLeadProvider(provider, func() *time.Duration {
		if factory == nil {
			return nil
		}
		auth := factory()
		if auth == nil {
			return nil
		}
		return auth.RefreshLead()
	})
}
