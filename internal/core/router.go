package core

import (
	"sort"

	"github.com/oaooao/webx/internal/types"
)

var registry []types.Adapter

func RegisterAdapter(a types.Adapter) {
	registry = append(registry, a)
	sort.Slice(registry, func(i, j int) bool {
		return registry[i].Priority() > registry[j].Priority()
	})
}

func Route(ctx types.MatchContext) types.Adapter {
	for _, adapter := range registry {
		if ctx.RequestedKind != nil {
			kindMatch := false
			for _, k := range adapter.Kinds() {
				if k == *ctx.RequestedKind {
					kindMatch = true
					break
				}
			}
			if !kindMatch {
				continue
			}
		}
		if adapter.Match(ctx) {
			return adapter
		}
	}
	return nil
}

func ListAdapters() []types.Adapter {
	result := make([]types.Adapter, len(registry))
	copy(result, registry)
	return result
}

// ResetRegistry clears all adapters (for testing).
func ResetRegistry() {
	registry = nil
}
