package web_responders

// A Locationer is a type that can return its location, relative to
// the server root path.
type Locationer interface {
	Location() string
}
