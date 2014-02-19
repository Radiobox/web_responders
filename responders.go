// The web_responders package takes care of our custom vendor codecs for
// Radiobox, handling responses, and even providing helpers for parsing
// input parameters.
package web_responders

import (
	"errors"
	"fmt"
	"github.com/Radiobox/web_request_readers"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/objx"
	"reflect"
	"strings"
	"unicode"
)

// database/sql has nullable values which all have the same prefix.
const SqlNullablePrefix = "Null"

// CreateResponse takes a value to be used as a response and attempts
// to generate a value to respond with, based on struct tag and
// interface matching.
//
// Values which implement LazyLoader will have their LazyLoad method
// run first, in order to load any values that haven't been loaded
// yet.
//
// Struct values will be converted to a map[string]interface{}.  Each
// field will be assigned a key - the "request" tag's value if it
// exists, or the "response" tag's value if it exists, or just the
// lowercase field name if neither tag exists.  A value of "-" for the
// key (i.e. the value of a request or response tag) will result in
// the field being skipped.
//
// CreateResponse will skip parsing any sub-elements of a response
// (i.e. entries in a slice or map, or fields of a struct) that
// implement the ResponseValueCreator, and instead just use the return
// value of their ResponseValue() method.
func CreateResponse(data interface{}, optionList ...interface{}) interface{} {
	if err, ok := data.(error); ok {
		return err.Error()
	}

	// Parse options
	var (
		options     objx.Map
		constructor func(interface{}) interface{}
	)
	switch len(optionList) {
	case 2:
		constructor = optionList[1].(func(interface{}) interface{})
		fallthrough
	case 1:
		options = optionList[0].(objx.Map)
	}
	return createResponse(data, false, options, constructor)
}

func createResponse(data interface{}, isSubResponse bool, options objx.Map, constructor func(interface{}) interface{}) interface{} {

	// LazyLoad with options
	if lazyLoader, ok := data.(LazyLoader); ok {
		lazyLoader.LazyLoad(options)
	}

	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		data = createStructResponse(value, options, constructor)
	case reflect.Slice, reflect.Array:
		data = createSliceResponse(value, options, constructor)
		if isSubResponse {
			data = constructor(data)
		}
	case reflect.Map:
		data = createMapResponse(value, options, constructor)
	}
	return data
}

// createNullableDbResponse checks for "database/sql".Null* types, or
// anything with a similar structure, and pulls out the underlying
// value.  For example:
//
//     type NullInt struct {
//         Int int
//         Valid bool
//     }
//
// If Valid is false, this function will return nil; otherwise, it
// will return the value of the Int field.
func createNullableDbResponse(value reflect.Value, valueType reflect.Type) (interface{}, error) {
	typeName := valueType.Name()
	if strings.HasPrefix(typeName, SqlNullablePrefix) {
		fieldName := typeName[len(SqlNullablePrefix):]
		val := value.FieldByName(fieldName)
		isNotNil := value.FieldByName("Valid")
		if val.IsValid() && isNotNil.IsValid() {
			// We've found a nullable type
			if isNotNil.Interface().(bool) {
				return val.Interface(), nil
			} else {
				return nil, nil
			}
		}
	}
	return nil, errors.New("No Nullable DB value found")
}

// createMapResponse is a helper for generating a response value from
// a value of type map.
func createMapResponse(value reflect.Value, options objx.Map, constructor func(interface{}) interface{}) interface{} {
	response := reflect.MakeMap(value.Type())
	for _, key := range value.MapKeys() {
		elementOptions := options.Get(key.Interface().(string)).ObjxMap()
		itemResponse := createResponseValue(value.MapIndex(key), elementOptions, constructor)
		response.SetMapIndex(key, reflect.ValueOf(itemResponse))
	}
	return response.Interface()
}

// createSliceResponse is a helper for generating a response value
// from a value of type slice.
func createSliceResponse(value reflect.Value, options objx.Map, constructor func(interface{}) interface{}) interface{} {
	response := make([]interface{}, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		response = append(response, createResponseValue(element, options, constructor))
	}
	return response
}

// createStructResponse is a helper for generating a response value
// from a value of type struct.
func createStructResponse(value reflect.Value, options objx.Map, constructor func(interface{}) interface{}) interface{} {
	structType := value.Type()

	// Support "database/sql".Null* types, and any other types
	// matching that structure
	if v, err := createNullableDbResponse(value, structType); err == nil {
		return v
	}

	response := make(objx.Map)

	for i := 0; i < value.NumField(); i++ {
		fieldType := structType.Field(i)
		fieldValue := value.Field(i)

		if fieldType.Anonymous {
			embeddedResponse := CreateResponse(fieldValue.Interface(), options, constructor).(objx.Map)
			for key, value := range embeddedResponse {
				// Don't overwrite values from the base struct
				if _, ok := response[key]; !ok {
					response[key] = value
				}
			}
		} else if unicode.IsUpper(rune(fieldType.Name[0])) {
			name := fieldType.Tag.Get("response")
			switch name {
			case "-":
				continue
			case "":
				name = strings.ToLower(fieldType.Name)
				fallthrough
			default:
				response[name] = createResponseValue(fieldValue, options.Get(name).ObjxMap(), constructor)
			}
		}
	}
	return response
}

// createResponseValue is a helper for generating a response value for
// a single value in a response object.
func createResponseValue(value reflect.Value, options objx.Map, constructor func(interface{}) interface{}) (responseValue interface{}) {
	if options.Get("type").Str() != "full" {
		switch source := value.Interface().(type) {
		case ResponseValueCreator:
			responseValue = source.ResponseValue(options)
		case fmt.Stringer:
			responseValue = source.String()
		case error:
			responseValue = source.Error()
		default:
			responseValue = CreateResponse(value.Interface(), options, constructor)
		}
	} else {
		responseValue = createResponse(value.Interface(), true, options, constructor)
	}
	return
}

// Respond performs an API response, adding some additional data to
// the context's CodecOptions to support our custom codecs.  This
// particular function is very specifically for use with the
// github.com/stretchr/goweb web framework.
//
// TODO: Move the with={} parameter to options in the mimetypes in the
// Accept header.
func Respond(ctx context.Context, status int, notifications MessageMap, data interface{}) error {
	params, err := web_request_readers.ParseParams(ctx)
	if err != nil {
		return err
	}
	params.Set("joins", ctx.QueryValue("joins"))
	options := ctx.CodecOptions()
	options.MergeHere(objx.Map{
		"status":        status,
		"input_params":  params,
		"notifications": notifications,
	})

	// Right now, this line is commented out to support our joins
	// logic.  Unfortunately, that means that codecs other than our
	// custom codecs from this package will not work.  Whoops.
	// data = CreateResponse(data)

	return goweb.API.WriteResponseObject(ctx, status, data)
}
