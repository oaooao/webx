package twitter

import (
	"errors"
	"testing"

	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

func TestBackends_ReportsChainInOrder(t *testing.T) {
	stubTransports(t)
	oldLookup := lookupEgoCLI
	t.Cleanup(func() { lookupEgoCLI = oldLookup })

	loadAuth = func() (*twitterbe.Auth, error) { return nil, errors.New("not configured") }
	lookupEgoCLI = func() (string, error) { return "/bin/ego-browser", nil }

	got := New().(types.DiagnosableAdapter).Backends(types.MatchContext{})
	if len(got) != 2 {
		t.Fatalf("got %d backends", len(got))
	}
	if got[0].Backend != directBackend || got[0].Order != 1 || got[0].Ready {
		t.Errorf("direct = %+v", got[0])
	}
	if got[1].Backend != "egolite" || got[1].Order != 2 || !got[1].Ready {
		t.Errorf("egolite = %+v", got[1])
	}
}
