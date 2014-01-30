// The rest_codecs package takes care of our custom vendor codecs for
// Radiobox, handling responses and even providing helpers for parsing
// input parameters.
package rest_codecs

import (
	"errors"
	"fmt"
	"github.com/Radiobox/rest_codecs/codecs"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/objx"
	"reflect"
	"strings"
	"unicode"
)

// database/sql has nullable values which all have the same prefix.
const SqlNullablePrefix = "Null"

// ResponseCreator is a datatype that prefers to create its own
// response value when used as a value within a response, rather than
// being automatically parsed into a sub-element.
type ResponseCreator interface {
	Response() interface{}
}

// MessageMap is a map intended to be used for carrying messages
// around, for the purpose of error handling.  Methods on MessageMap
// always expect the MessageMap to already contain the keys "err",
// "warn", and "info"; and for each of those to contain a slice of
// strings.
type MessageMap map[string][]string

// NewMessageMap returns a MessageMap that is properly initialized.
func NewMessageMap() MessageMap {
	return MessageMap{
		"err":  []string{},
		"warn": []string{},
		"info": []string{},
	}
}

func (mm MessageMap) addMessage(severity, message string) {
	mm[severity] = append(mm[severity], message)
}

func (mm MessageMap) AddErrorMessage(message string) {
	mm.addMessage("err", message)
}

func (mm MessageMap) Errors() []string {
	return mm["err"]
}

func (mm MessageMap) AddWarningMessage(message string) {
	mm.addMessage("warn", message)
}

func (mm MessageMap) Warnings() []string {
	return mm["warn"]
}

func (mm MessageMap) AddInfoMessage(message string) {
	mm.addMessage("info", message)
}

func (mm MessageMap) Infos() []string {
	return mm["info"]
}

func (mm MessageMap) NumErrors() int {
	return len(mm.Errors())
}

func (mm MessageMap) NumWarnings() int {
	return len(mm.Warnings())
}

func (mm MessageMap) NumInfos() int {
	return len(mm.Infos())
}

// createNullableDbResponse checks for "database/sql".Null* types, or
// anything with a similar structure, and pulls out the underlying
// value.  For example:
//
// type NullInt struct {
//     Int int
//     Valid bool
// }
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

// createResponse takes a value to be used as a response and attempts
// to generate a map of values, to be used in the response, from the
// value's fields.  It returns an empty map if the passed in value is
// not a struct.
//
// It uses the "response" struct tag for naming purposes when
// converting from a struct type to a map[string]interface{} type - if
// a field has a "response" tag, that will be used for the index in
// the response; otherwise, the lowercase field name will be used.
func createResponse(data interface{}) interface{} {
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		return createStructResponse(value)
	case reflect.Slice, reflect.Array:
		return createSliceResponse(value)
	case reflect.Map:
		return createMapResponse(value)
	default:
		return data
	}
}

func createMapResponse(value reflect.Value) interface{} {
	response := reflect.MakeMap(value.Type())
	for _, key := range value.MapKeys() {
		itemResponse := createResponseValue(value.MapIndex(key))
		response.SetMapIndex(key, reflect.ValueOf(itemResponse))
	}
	return response.Interface()
}

func createSliceResponse(value reflect.Value) []interface{} {
	response := make([]interface{}, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		response = append(response, createResponseValue(element))
	}
	return response
}

func createResponseValue(value reflect.Value) (responseValue interface{}) {
	switch source := value.Interface().(type) {
	case ResponseCreator:
		responseValue = source.Response()
	case fmt.Stringer:
		responseValue = source.String()
	case error:
		responseValue = source.Error()
	default:
		// Use reflect to try to handle nested elements
		switch value.Kind() {
		case reflect.Ptr:
			value = value.Elem()
			fallthrough
		case reflect.Struct:
			responseValue = createStructResponse(value)
		case reflect.Map:
			responseValue = createMapResponse(value)
		case reflect.Slice:
			responseValue = createSliceResponse(value)
		default:
			// Pretty sure we've handled all of the common scenarios
			// where the actual source element needs to be converted,
			// so at this point, the source element probably works
			// just fine as is.
			responseValue = source
		}
	}
	return
}

func createStructResponse(value reflect.Value) interface{} {
	structType := value.Type()

	// Support "database/sql".Null* types, and any other types
	// matching that structure
	if v, err := createNullableDbResponse(value, structType); err == nil {
		return v
	}

	response := make(map[string]interface{})

	for i := 0; i < value.NumField(); i++ {
		fieldType := structType.Field(i)
		fieldValue := value.Field(i)

		if fieldType.Anonymous {
			embeddedResponse := createResponse(fieldValue.Interface()).(map[string]interface{})
			for key, value := range embeddedResponse {
				// Don't overwrite values from the base struct
				if _, ok := response[key]; !ok {
					response[key] = value
				}
			}
		} else if unicode.IsUpper(rune(fieldType.Name[0])) {
			responseValue := createResponseValue(fieldValue)

			name := fieldType.Tag.Get("response")
			switch name {
			case "-":
				continue
			case "":
				name = strings.ToLower(fieldType.Name)
				fallthrough
			default:
				response[name] = responseValue
			}
		}
	}
	return response
}

func Respond(ctx context.Context, status int, notifications MessageMap, data interface{}) error {
	params, err := ParseParams(ctx)
	if err != nil {
		return err
	}
	options := ctx.CodecOptions()
	options.MergeHere(objx.Map{
		"status":        status,
		"input_params":  params,
		"notifications": notifications,
	})

	data = createResponse(data)

	return goweb.API.WriteResponseObject(ctx, status, data)
}

func init() {
	codecs.AddCodecs()
}
