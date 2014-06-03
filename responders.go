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
	"net/http"
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
		constructor func(interface{}, interface{}) interface{}
		domain      string
	)
	switch len(optionList) {
	case 3:
		domain = optionList[2].(string)
		fallthrough
	case 2:
		constructor = optionList[1].(func(interface{}, interface{}) interface{})
		fallthrough
	case 1:
		options = optionList[0].(objx.Map)
	}
	return createResponse(data, false, options, constructor, domain)
}

func createResponse(data interface{}, isSubResponse bool, options objx.Map, constructor func(interface{}, interface{}) interface{}, domain string) interface{} {

	// LazyLoad with options
	if lazyLoader, ok := data.(LazyLoader); ok {
		lazyLoader.LazyLoad(options)
	}

	responseData := data
	if responseCreator, ok := data.(ResponseObjectCreator); ok {
		responseData = responseCreator.ResponseObject()
	}

	value := reflect.ValueOf(responseData)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		data = createStructResponse(value, options, constructor, domain)
	case reflect.Slice, reflect.Array:
		data = createSliceResponse(value, options, constructor, domain)
		if options != nil && isSubResponse {
			data = constructor(data, value)
		}
	case reflect.Map:
		data = createMapResponse(value, options, constructor, domain)
	case reflect.String:
		if domain != "" {
			// Prepend the domain to all links
			strPtr := new(string)
			strVal := reflect.ValueOf(strPtr).Elem()
			strVal.Set(value.Convert(strVal.Type()))
			str := *strPtr
			if str != "" && str[0] == '/' {
				str = domain + str
			}
			data = str
		} else {
			data = responseData
		}
	default:
		data = responseData
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
func createMapResponse(value reflect.Value, options objx.Map, constructor func(interface{}, interface{}) interface{}, domain string) interface{} {
	response := reflect.MakeMap(value.Type())
	for _, key := range value.MapKeys() {
		var elementOptions objx.Map
		keyStr := key.Interface().(string)
		if options != nil {
			var elementOptionsValue *objx.Value
			if options.Has(keyStr) {
				elementOptionsValue = options.Get(keyStr)
			} else if options.Has("*") {
				elementOptionsValue = options.Get("*")
			}
			if elementOptionsValue.IsMSI() {
				elementOptions = objx.Map(elementOptionsValue.MSI())
			} else if elementOptionsValue.IsObjxMap() {
				elementOptions = elementOptionsValue.ObjxMap()
			} else {
				panic("Don't know what to do with option")
			}
		}
		itemResponse := createResponseValue(value.MapIndex(key), elementOptions, constructor, domain)
		response.SetMapIndex(key, reflect.ValueOf(itemResponse))
	}
	return response.Interface()
}

// createSliceResponse is a helper for generating a response value
// from a value of type slice.
func createSliceResponse(value reflect.Value, options objx.Map, constructor func(interface{}, interface{}) interface{}, domain string) interface{} {
	response := make([]interface{}, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		response = append(response, createResponseValue(element, options, constructor, domain))
	}
	return response
}

func ResponseTag(field reflect.StructField) string {
	var name string
	if name = field.Tag.Get("response"); name != "" {
		return name
	}
	if field.Name != "Id" {
		if name = field.Tag.Get("db"); name != "" && name != "-" {
			return name
		}
	}
	return strings.ToLower(field.Name)
}

// createStructResponse is a helper for generating a response value
// from a value of type struct.
func createStructResponse(value reflect.Value, options objx.Map, constructor func(interface{}, interface{}) interface{}, domain string) interface{} {
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
			embeddedResponse := CreateResponse(fieldValue.Interface(), options, constructor, domain).(objx.Map)
			for key, value := range embeddedResponse {
				// Don't overwrite values from the base struct
				if _, ok := response[key]; !ok {
					response[key] = value
				}
			}
		} else if unicode.IsUpper(rune(fieldType.Name[0])) {
			name := ResponseTag(fieldType)
			switch name {
			case "-":
				continue
			default:
				var subOptions objx.Map
				if options != nil && (options.Has(name) || options.Has("*")) {
					var subOptionsValue *objx.Value
					if options.Has(name) {
						subOptionsValue = options.Get(name)
					} else {
						subOptionsValue = options.Get("*")
					}
					if subOptionsValue.IsMSI() {
						subOptions = objx.Map(subOptionsValue.MSI())
					} else if subOptionsValue.IsObjxMap() {
						subOptions = subOptionsValue.ObjxMap()
					} else {
						panic("Don't know what to do with option")
					}
				}
				response[name] = createResponseValue(fieldValue, subOptions, constructor, domain)
			}
		}
	}
	return response
}

// createResponseValue is a helper for generating a response value for
// a single value in a response object.
func createResponseValue(value reflect.Value, options objx.Map, constructor func(interface{}, interface{}) interface{}, domain string) (responseValue interface{}) {
	if value.Kind() == reflect.Ptr && !value.Elem().IsValid() {
		responseValue = nil
		if nilResponder, ok := value.Interface().(NilResponder); ok {
			responseValue = nilResponder.NilResponseValue()
		}
	} else if options.Get("type").Str() != "full" {
		switch source := value.Interface().(type) {
		case ResponseValueCreator:
			responseValue = createResponse(source.ResponseValue(options), true, options, constructor, domain)
		case fmt.Stringer:
			responseValue = createResponse(source.String(), true, options, constructor, domain)
		case error:
			responseValue = createResponse(source.Error(), true, options, constructor, domain)
		default:
			responseValue = createResponse(value.Interface(), true, options, constructor, domain)
		}
	} else {
		responseValue = createResponse(value.Interface(), true, options, constructor, domain)
	}
	return
}

// RespondWithInputErrors attempts to figure out where the input
// values (in ctx) may have caused problems when being set to fields
// on data, and then add them to the input errors on the notifications
// map.
//
// For each field in data, if the field is an InputValidator,
// the input checking logic will just be handed off to its
// ValidateInput method; if the field is a RequestValueReceiver, the
// error value returned from Receive will be used to validate;
// otherwise, we will attempt to check that the input value is
// assignable to the field.
//
// If checkMissing is true, required fields that have no value present in
// the input parameters will be considered input errors and will be
// added to the message map.
func RespondWithInputErrors(ctx context.Context, notifications MessageMap, data interface{}, checkMissing bool) error {
	dataType := reflect.TypeOf(data)
	if dataType.Kind() == reflect.Ptr {
		dataType = dataType.Elem()
	}
	params, err := web_request_readers.ParseParams(ctx)
	if err != nil {
		return err
	}
	params = params.Copy()
	addInputErrors(dataType, params, notifications, checkMissing)

	// addInputErrors will delete all params that it has checked for
	// input errors, so anything remaining in params has no matching
	// field.
	for key := range params {
		notifications.SetInputMessage(key, "No target field found for this input")
	}
	status := http.StatusBadRequest
	if len(notifications.InputMessages()) == 0 {
		// There were no errors from the input, but something still
		// went wrong - this is probably an internal server error.
		status = http.StatusInternalServerError
	}
	return Respond(ctx, status, notifications, notifications)
}

func checkForInputError(fieldType reflect.Type, value interface{}) error {

	// We always want to check the pointer to the value (and never the
	// pointer to the pointer to the value) for interface matching.
	var emptyValue reflect.Value
	if fieldType.Kind() == reflect.Ptr {
		emptyValue = reflect.New(fieldType.Elem())
	} else {
		emptyValue = reflect.New(fieldType)
	}

	// A type switch would look cleaner here, but we want a very
	// specific order of preference for these interfaces.  A type
	// switch does not guarantee any preferred order, just that
	// one valid case will be executed.
	emptyInter := emptyValue.Interface()
	if validator, ok := emptyInter.(InputValidator); ok {
		return validator.ValidateInput(value)
	}
	if receiver, ok := emptyInter.(web_request_readers.RequestValueReceiver); ok {
		return receiver.Receive(value)
	}

	fieldTypeName := fieldType.Name()
	if fieldType.Kind() == reflect.Struct && strings.HasPrefix(fieldTypeName, SqlNullablePrefix) {
		// database/sql defines many Null* types,
		// where the fields are Valid (a bool) and the
		// name of the type (everything after Null).
		// We're trying to support them (somewhat)
		// here.
		typeName := fieldTypeName[len(SqlNullablePrefix):]
		nullField, ok := fieldType.FieldByName(typeName)
		if ok {
			// This is almost definitely an sql.Null* type.
			if value == nil {
				return nil
			}
			fieldType = nullField.Type
		}
	}
	if !reflect.TypeOf(value).ConvertibleTo(fieldType) {
		return errors.New("Input is of the wrong type and cannot be converted")
	}
	return nil
}

// addInputErrors (which, to be honest, should be in the
// web_request_parsers package) walks through
func addInputErrors(dataType reflect.Type, params objx.Map, notifications MessageMap, checkMissing bool) {
	for i := 0; i < dataType.NumField(); i++ {
		field := dataType.Field(i)
		if field.Anonymous {
			addInputErrors(field.Type, params, notifications, checkMissing)
			continue
		}

		if unicode.IsUpper(rune(field.Name[0])) {
			name, args := web_request_readers.NameAndArgs(field)
			if name == "-" {
				continue
			}

			optional := false
			for _, arg := range args {
				if arg == "optional" {
					optional = true
				}
			}

			value, ok := params[name]
			if !ok {
				if !optional && checkMissing {
					notifications.SetInputMessage(name, "No input for required field")
				}
				continue
			}

			// We're now at the point where we know this parameter has a
			// target field and will be checked, so remove it from the
			// map.
			delete(params, name)

			if err := checkForInputError(field.Type, value); err != nil {
				notifications.SetInputMessage(name, err.Error())
			}
		}
	}
}

// Respond performs an API response, adding some additional data to
// the context's CodecOptions to support our custom codecs.  This
// particular function is very specifically for use with the
// github.com/stretchr/goweb web framework.
//
// TODO: Move the with={} parameter to options in the mimetypes in the
// Accept header.
func Respond(ctx context.Context, status int, notifications MessageMap, data interface{}, useFullDomain ...bool) error {
	body, err := web_request_readers.ParseBody(ctx)
	if err != nil {
		return err
	}
	if ctx.QueryParams().Has("joins") {
		if m, ok := body.(objx.Map); ok {
			m.Set("joins", ctx.QueryValue("joins"))
		}
	}

	protocol := "http"
	if ctx.HttpRequest().TLS != nil {
		protocol += "s"
	}

	host := ctx.HttpRequest().Host

	requestDomain := fmt.Sprintf("%s://%s", protocol, host)
	if status == http.StatusOK {
		location := "Error: no location present"
		if locationer, ok := data.(Locationer); ok {
			location = fmt.Sprintf("%s%s", requestDomain, locationer.Location())
		}
		ctx.HttpResponseWriter().Header().Set("Location", location)

		if linker, ok := data.(RelatedLinker); ok {
			linkMap := linker.RelatedLinks()
			links := make([]string, 0, len(linkMap)+1)
			links = append(links, fmt.Sprintf(`<%s>; rel="location"`, location))
			for rel, link := range linkMap {
				link := fmt.Sprintf(`<%s%s>; rel="%s"`, requestDomain, link, rel)
				links = append(links, link)
			}
			ctx.HttpResponseWriter().Header().Set("Link", strings.Join(links, ", "))
		}
	}
	// Transitionary period - don't pass the domain to the codec
	// unless it's requested in the responder
	if len(useFullDomain) == 0 || useFullDomain[0] == false {
		requestDomain = ""
	}

	options := ctx.CodecOptions()
	options.MergeHere(objx.Map{
		"status":        status,
		"input_params":  body,
		"notifications": notifications,
		"domain":        requestDomain,
	})

	// Right now, this line is commented out to support our joins
	// logic.  Unfortunately, that means that codecs other than our
	// custom codecs from this package will not work.  Whoops.
	// data = CreateResponse(data)

	return goweb.API.WriteResponseObject(ctx, status, data)
}
