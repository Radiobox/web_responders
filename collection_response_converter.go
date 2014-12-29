package web_responders

// CollectionResponseConverter is a type that returns a modified type when
// used as an element within a collection response (i.e. the top level of
// the response is a list type).
type CollectionResponseConverter interface {
	// CollectionResponse should return the response structure to be used
	// when the CollectionResponseConverter is used as an element at the
	// top level of a collection response.
	CollectionResponse() interface{}
}
