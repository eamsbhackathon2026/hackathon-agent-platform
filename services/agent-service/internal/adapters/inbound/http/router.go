package http

import (
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// Handler composes the implemented strict API handlers.
type Handler struct {
	*SystemHandler
	*IdentityHandler
	*CatalogHandler
	*ToolingHandler
	*SkillHandler
	*RunHandler
	*SessionHandler
	*WebhookDeliveryHandler
	*ReportHandler
}

// RouterOptions supplies authentication and browser access settings.
type RouterOptions struct {
	Resolver inbound.PrincipalResolver
	Origins  []string
	Logger   *slog.Logger
}

// NewRouter mounts and validates only operations enabled in the generated contract.
func NewRouter(handler gen.StrictServerInterface, options RouterOptions) (http.Handler, error) {
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, err
	}
	// Deployment hostnames belong to configuration, not request validation.
	spec.Servers = nil
	contract, err := legacy.NewRouter(spec)
	if err != nil {
		return nil, err
	}
	router := chi.NewRouter()
	router.Use(commonMiddleware(options.Logger), corsMiddleware(options.Origins))
	router.Use(contractMiddleware(contract, options.Resolver))
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusNotFound, "Không tìm thấy địa chỉ", "not_found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusMethodNotAllowed, "Phương thức không được hỗ trợ", "validation_failed")
	})
	strict := gen.NewStrictHandlerWithOptions(handler, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeProblem(w, http.StatusBadRequest, "Yêu cầu không hợp lệ", "validation_failed")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) { writeDomainError(w, err) },
	})
	return gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeProblem(w, http.StatusBadRequest, "Yêu cầu không hợp lệ", "validation_failed")
		},
	}), nil
}
