package rest_codecs

import (
	"github.com/stretchr/goweb/context"
)

type BaseRestController struct{}

// After makes sure that the Vary header is set to Accept, after the
// correct method has run.  This is for client caching purposes.
func (controller *BaseRestController) After(ctx context.Context) error {
	ctx.HttpResponseWriter().Header().Set("Vary", "Accept")
	return nil
}
