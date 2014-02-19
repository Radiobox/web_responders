package web_responders

import (
	"github.com/stretchr/objx"
)

// ResponseValueCreator is a datatype that prefers to create its own
// response value when it is used as a piece of a response (i.e. in a
// struct field, or as a value in a slice or map).
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
type ResponseValueCreator interface {

	// ResponseValue should return the value that will be used to
	// represent the underlying value in a response.
	ResponseValue(options objx.Map) interface{}
}
