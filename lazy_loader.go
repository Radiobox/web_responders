package web_responders

// A LazyLoader is a type that has some values that load lazily.  The
// LazyLoad method will be called before responding using a type that
// implements LazyLoader.
type LazyLoader interface {
	// LazyLoad should load values into all members that can be loaded
	// lazily.
	LazyLoad()
}
