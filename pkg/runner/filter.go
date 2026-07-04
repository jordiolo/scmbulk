package runner

import (
	"scmbulk/pkg/config"
	"scmbulk/pkg/selection"
)

// matcher is the minimal filter behaviour the runner uses.
type matcher interface {
	Matches(rule map[string]interface{}) bool
}

func newFilter(sel config.Selection) (matcher, error) {
	return selection.New(sel)
}
