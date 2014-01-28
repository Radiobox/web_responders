// The rest_codecs package takes care of our custom vendor codecs for
// Radiobox, handling responses and even providing helpers for parsing
// input parameters.
package rest_codecs

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Radiobox/rest_codecs/codecs"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/objx"
	"io/ioutil"
	"reflect"
	"strconv"
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
	switch value.Kind() {
	case reflect.Struct:
		return createStructResponse(value)
	case reflect.Slice, reflect.Array:
		return createSliceResponse(value)
	default:
		return data
	}
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
	kind := value.Kind()
	switch source := value.Interface().(type) {
	case ResponseCreator:
		responseValue = source.Response()
	case fmt.Stringer:
		responseValue = source.String()
	case error:
		responseValue = source.Error()
	default:
		if kind == reflect.Struct || kind == reflect.Ptr {
			responseValue = createStructResponse(value)
		} else {
			responseValue = source
		}
	}
	return
}

func createStructResponse(value reflect.Value) interface{} {
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
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

type BaseRestController struct{}

// After makes sure that the Vary header is set to Accept, after the
// correct method has run.  This is for client caching purposes.
func (controller *BaseRestController) After(ctx context.Context) error {
	ctx.HttpResponseWriter().Header().Set("Vary", "Accept")
	return nil
}

func ParseParams(ctx context.Context) (objx.Map, error) {
	if params, ok := ctx.Data()["params"]; ok {
		// We've already parsed this request, so return the cached
		// parameters.
		return params.(objx.Map), nil
	}
	request := ctx.HttpRequest()
	response := objx.Map(make(map[string]interface{}))
	switch request.Header.Get("Content-Type") {
	case "text/json":
		fallthrough
	case "application/json":
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(body, &response); err != nil {
			return nil, err
		}
	default:
		fallthrough
	case "application/x-www-form-urlencoded":
		// Assume form.
		request.ParseForm()
		for index, values := range request.Form {
			if len(values) == 1 {
				// Okay, so, here's how this works.  I hate just
				// assuming that there's only one value when I'm
				// reading a form, so I always end up testing the
				// length, which adds boilerplate code.  I want my
				// param parser to handle that case, so instead of
				// always adding a slice of values, I'm only adding
				// the single value if the length of the slice is 1.
				response.Set(index, values[0])
			} else {
				response.Set(index, values)
			}
		}
	}
	ctx.Data().Set("params", response)
	return response, nil
}

// ParsePage reads "page" and "page_size" from a set of parameters and
// parses them into offset and limit values.
func ParsePage(params objx.Map, defaultPageSize int) (offset, limit int, err error) {
	limit = defaultPageSize

	sizeVal, sizeOk := params["pageSize"]
	pageVal, pageOk := params["page"]
	if !pageOk || !sizeOk {
		return
	}

	var page, pageSize int
	switch sizeVal := sizeVal.(type) {
	case string:
		pageSize, err = strconv.Atoi(sizeVal)
		if err != nil {
			return
		}
	case int:
		pageSize = sizeVal
	case int32:
		pageSize = int(sizeVal)
	case int64:
		pageSize = int(sizeVal)
	}

	switch pageVal := pageVal.(type) {
	case string:
		page, err = strconv.Atoi(pageVal)
		if err != nil {
			return
		}
	case int:
		page = pageVal
	case int32:
		page = int(pageVal)
	case int64:
		page = int(pageVal)
	}

	offset = (page - 1) * pageSize
	limit = pageSize
	return
}

func init() {
	codecs.AddCodecs()
}
