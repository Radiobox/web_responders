package rest_codecs

import (
	"encoding/json"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/objx"
	"io/ioutil"
	"strconv"
)

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
