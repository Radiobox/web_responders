package web_responders

// ResponseDataConverter should be used for types that want a
// different structure or value used as their data for responses,
// regardless of whether they're used as the top level response data
// or a sub-element within the response.
//
// Note that if your type implements both ResponseConverter and
// ResponseElementConverter, ResponseElementConverter will override
// ResponseConverter when the type is used as a sub-element in a
// response.
//
// Example:
//
//     type StringCollection struct {
//         collection []string
//         queryArgs map[string]string
//     }
//
//     func (coll *StringCollection) ResponseData() interface{} {
//         return coll.collection
//     }
type ResponseConverter interface {
	// ResponseData should return the data that will be used instead
	// of the ResponseConverter in a response.
	ResponseData() interface{}
}
