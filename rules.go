//go:build ruleguard
// +build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

func noShouldBind(m dsl.Matcher) {
	m.Match(`$c.ShouldBind($x)`).
		Where(m.File().Name.Matches(`handler\.go$`)).
		Report(`use ShouldBindJSON instead of ShouldBind in handler files`)
}
