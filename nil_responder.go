package web_responders

// A NilElementConverter is a type that has a special structure in a
// response when it is nil.
type NilElementConverter interface {
	// NilElementData should return the value to be used in place of
	// the NilElementConverter in a response when the
	// NilElementConverter is nil.
	NilElementData() interface{}
}
