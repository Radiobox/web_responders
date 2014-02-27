package web_responders

// ResponseObjectCreator should be used for types that don't want
// their actual value used in the response.  For example, if you have
// a collection type that you want to appear as a slice, but needs to
// be a struct type to keep track of some data, you can use this to
// return the slice value.
//
// This should not be confused with ResponseValueCreator, which is
// only used when a type is a value within a response object, not the
// response object itself.
//
// Note that if your ResponseObjectCreator also implements LazyLoader
// and/or RelatedLinker, LazyLoad() and/or RelatedLinks() will still
// be called on the original object, but the value returned by
// ResponseObject() will be used as the response body.
//
// Example:
//
//     type StringCollection struct {
//         collection []string
//         queryArgs map[string]string
//     }
//
//     func (coll *StringCollection) ResponseObject() interface{} {
//         return coll.collection
//     }
type ResponseObjectCreator interface {
	ResponseObject() interface{}
}
