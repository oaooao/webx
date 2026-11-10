package twitter

import (
	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

// lookupEgoCLI is a test seam.
var lookupEgoCLI = egolite.LookupCLI

// Backends implements types.DiagnosableAdapter. Reads try the direct GraphQL
// client first and fall back to ego lite; see fetchWithFallback for when.
func (a *twitterAdapter) Backends(types.MatchContext) []types.BackendStatus {
	direct := types.BackendStatus{Backend: directBackend, Order: 1}
	if _, err := loadAuth(); err == nil {
		direct.Ready = true
		direct.Detail = "stored credentials found (not validated; doctor makes no requests)"
	} else {
		direct.Detail = "no stored credentials, skipped: " + err.Error()
	}

	ego := types.BackendStatus{Backend: egolite.BackendID, Order: 2}
	if p, err := lookupEgoCLI(); err == nil {
		ego.Ready = true
		ego.Detail = "ego-browser CLI at " + p + "; used when direct has no credentials, is rejected (401/403), cannot reach x.com, or returns nothing. Login state is checked on use"
	} else {
		ego.Detail = err.Error()
	}
	return []types.BackendStatus{direct, ego}
}
