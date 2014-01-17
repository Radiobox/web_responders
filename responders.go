// The rest_codecs package takes care of our custom vendor codecs for
// Radiobox, handling responses and even providing helpers for parsing
// input parameters.
package rest_codecs

import (
	"github.com/Radiobox/rest_codecs/codecs"
	"encoding/json"
	"fmt"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/objx"
	"io/ioutil"
	"reflect"
	"strings"
	"unicode"
)

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

func ignorePanic() {
	recover()
}

func createResponse(data interface{}) map[string]interface{} {
	response := make(map[string]interface{})
	value := reflect.ValueOf(data)
	structType := value.Type()
	if structType.Name() == "" {
		// data is probably a pointer, so get the indirect.
		value = reflect.Indirect(value)
		structType = value.Type()
	}

	// This next bit assumes that the data is a struct, which it may
	// not be.  If it's not, we don't really care, though - so we'll
	// just ignore it and move on.
	defer ignorePanic()

	for i := 0; i < value.NumField(); i++ {
		fieldType := structType.Field(i)
		fieldKind := fieldType.Type.Kind()
		fieldValue := value.Field(i)

		if fieldType.Anonymous {
			for key, value := range createResponse(fieldValue.Interface()) {
				if _, ok := response[key]; !ok {
					response[key] = value
				}
			}
		} else if unicode.IsUpper(rune(fieldType.Name[0])) {
			var responseValue interface{}
			switch source := fieldValue.Interface().(type) {
			case ResponseCreator:
				responseValue = source.Response()
			case fmt.Stringer:
				responseValue = source.String()
			case error:
				responseValue = source.Error()
			default:
				if fieldKind == reflect.Struct || fieldKind == reflect.Ptr {
					responseValue = createResponse(source)
				} else {
					responseValue = source
				}
			}

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

	if response := createResponse(data); response != nil {
		data = response
	}

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

func init() {
	codecs.AddCodecs()
}
