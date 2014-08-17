package web_responders

import (
	"github.com/stretchr/objx"
)

// ResponseElementConverter is a type that converts itself to a
// different structure or type when it is used as a sub-element of a
// response.  It is not used when the type is the top-level response
// data.
//
// This is particularly useful for compressing a large struct to
// something much smaller in two situations:
//
// 1. You are responding with a list of *many* of the large struct
// type, often as part of a `GET /resource` response.
//
// 2. You are responding with a struct type that includes the large
// struct type as a field, often as part of a `GET /otherresource/id`
// response.
type ResponseElementConverter interface {
	// ResponseElementData should return the data that will be used
	// instead of the ResponseElementConverter *only* when it is a
	// sub-element of a response.
	ResponseElementData(options objx.Map) interface{}
}
