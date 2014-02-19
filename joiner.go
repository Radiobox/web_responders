package web_responders

import (
	"github.com/stretchr/objx"
)

// A Joiner is a type that can take a map of parameters and use them
// to determine any extra data that should be joined in the response.
// This is to help reduce the number of HTTP requests made by a client
// when creating a single page of its UI.
//
// Usually, we like to generate the full response from the codec for
// those embedded elements.  A simple example using stretchr/goweb:
//
//     type SubElement struct {
//         Id int
//         Name string
//     }
//
//     type MainElement struct {
//         Id int
//         Name string
//         SubElement *SubElement
//     }
//
//     func (elem *MainElement) Join(params objx.Map) {
//         if params.Has("sub_element") {
//             options := params.Get("sub_element").ObjxMap()
//
//             // query populates the rest of elem.SubElement from the
//             // database.  The options variable can contain limits
//             // and such.
//             query(elem.SubElement, elem.SubElement.Id, options)
//         }
//     }
type Joiner interface {

	// Join should accept a map of options and populate any nested
	// structs with joined values from the database.
	Join(objx.Map, func(interface{}) interface{})
}
