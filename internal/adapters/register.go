// Package adapters registers all built-in adapters with the core router.
// Import this package via a blank import in cmd/webx/main.go to trigger init().
//
// NOTE: RegisterAdapter is not goroutine-safe; call only during init().
package adapters

import (
	"github.com/oaooao/webx/internal/adapters/arxiv"
	"github.com/oaooao/webx/internal/adapters/generic"
	"github.com/oaooao/webx/internal/adapters/hn"
	"github.com/oaooao/webx/internal/adapters/reddit"
	"github.com/oaooao/webx/internal/adapters/twitter"
	"github.com/oaooao/webx/internal/core"
)

func init() {
	core.RegisterAdapter(twitter.New())
	core.RegisterAdapter(reddit.New())
	core.RegisterAdapter(hn.New())
	core.RegisterAdapter(arxiv.New())
	core.RegisterAdapter(generic.New())
}
