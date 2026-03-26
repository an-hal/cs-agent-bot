package http

import (
	deliveryHttpDeps "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/deps"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/example"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/health"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"

	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func SetupHandler(deps deliveryHttpDeps.Deps) http.Handler {
	// Initialize handlers
	exampleHandler := example.NewExampleHandler(deps.ExampleUC, deps.Logger, deps.Validator, deps.Tracer)
	healthHandler := health.NewHealthHandler(deps.Logger, deps.Tracer)

	r := router.NewRouter()

	// Configure exception handler for router
	r.SetErrorHandler(middleware.ErrorHandlingMiddleware(deps.ExceptionHandler))

	// Apply global middleware
	r.Use(middleware.TracingMiddleware(deps.Tracer))
	r.Use(middleware.RecoveryMiddleware(deps.Logger, deps.ExceptionHandler))
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware(deps.Logger))

	// Route without prefix
	r.Handle(http.MethodGet, "/readiness", healthHandler.Check)

	// Route group with prefix
	api := r.Group(deps.Cfg.RoutePrefix)

	api.HandlePrefix(http.MethodGet, "/swagger/", httpSwagger.WrapHandler)

	// Example routes (reference implementation)
	api.Handle(http.MethodGet, "/example/list", exampleHandler.GetAll)
	api.Handle(http.MethodGet, "/example/one", exampleHandler.GetByID)
	api.Handle(http.MethodPost, "/example/one", exampleHandler.Create)
	api.Handle(http.MethodDelete, "/example/one", exampleHandler.Delete)

	// TODO: Add agent bot routes here
	// Conversations, Messages, etc.

	return r
}
