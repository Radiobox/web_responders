package web_responders

import (
	"github.com/stretchr/objx"
)

// A LazyLoader is a type that has some values that load lazily.  The
// LazyLoad method will be called before responding using a type that
// implements LazyLoader.
type LazyLoader interface {
	// LazyLoad should load values into all members that can be loaded
	// lazily.  The objx.Map argument can contain details about which
	// lazy values are needed, or how much detail should be loaded for
	// lazy values.
	LazyLoad(options objx.Map)
}
