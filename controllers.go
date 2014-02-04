package web_responders

import (
	"github.com/stretchr/goweb/context"
)

// A BaseRestController is just a controller that always sets the Vary
// header to "Accept", since most REST APIs will change their response
// based on the Accept header.  All this really does is tell clients,
// "If your Accept header changes, you shouldn't use the cached
// value."
type BaseRestController struct{}

// After makes sure that the Vary header is set to Accept, after the
// correct method has run.  This is for client caching purposes.
func (controller *BaseRestController) After(ctx context.Context) error {
	ctx.HttpResponseWriter().Header().Set("Vary", "Accept")
	return nil
}
