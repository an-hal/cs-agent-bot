package router

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
)

type contextKey string

const ParamKey contextKey = "pathParams"

type Route struct {
	Method      string
	Pattern     *regexp.Regexp
	ParamNames  []string
	HandlerFunc http.HandlerFunc
}

type prefixRoute struct {
	Method  string
	Prefix  string
	Handler http.Handler
}

type Router struct {
	parent       *Router
	prefix       string
	routes       []Route
	prefixRoutes []prefixRoute
	middleware   []func(http.Handler) http.Handler
	errorHandler func(middleware.ErrorHandler) http.HandlerFunc
}

func NewRouter() *Router {
	return &Router{}
}

// root returns the root router in the chain.
func (r *Router) root() *Router {
	if r.parent != nil {
		return r.parent.root()
	}
	return r
}

// Group creates a new Router with the given prefix.
// All routes registered on the returned router will be prefixed with the given path.
// Routes are stored in the root router to ensure proper routing.
func (r *Router) Group(prefix string) *Router {
	return &Router{
		parent: r.root(),
		prefix: r.prefix + prefix,
	}
}

// Use adds global middleware that will be applied to all routes.
// Middleware is executed in the order it is registered.
func (r *Router) Use(mw func(http.Handler) http.Handler) {
	root := r.root()
	root.middleware = append(root.middleware, mw)
}

// SetErrorHandler configures the global error handler for routes with error-returning handlers.
// This must be called before registering any error-returning routes.
func (r *Router) SetErrorHandler(handler func(middleware.ErrorHandler) http.HandlerFunc) {
	root := r.root()
	root.errorHandler = handler
}

// Handle registers a new route with the given method and path.
func (r *Router) Handle(method, path string, handler interface{}) {
	fullPath := r.prefix + path

	paramNames := []string{}
	regexPattern := regexp.MustCompile(`\{(\w+)\}`)
	replaced := regexPattern.ReplaceAllStringFunc(fullPath, func(m string) string {
		name := m[1 : len(m)-1]
		paramNames = append(paramNames, name)
		return `([^/]+)`
	})

	finalRegex := regexp.MustCompile("^" + replaced + "$")

	// Type detection and conversion
	root := r.root()
	var handlerFunc http.HandlerFunc

	switch h := handler.(type) {
	case http.HandlerFunc:
		handlerFunc = h

	case func(http.ResponseWriter, *http.Request):
		handlerFunc = http.HandlerFunc(h)

	case middleware.ErrorHandler:
		if root.errorHandler == nil {
			panic("Error handler not configured. Call SetErrorHandler() before registering error-returning handlers.\n" +
				"Route: " + method + " " + fullPath + "\n" +
				"Hint: Add r.SetErrorHandler(middleware.ErrorHandlingMiddleware(exceptionHandler)) before route registration")
		}
		handlerFunc = root.errorHandler(h)

	case func(http.ResponseWriter, *http.Request) error:
		if root.errorHandler == nil {
			panic("Error handler not configured. Call SetErrorHandler() before registering error-returning handlers.\n" +
				"Route: " + method + " " + fullPath + "\n" +
				"Hint: Add r.SetErrorHandler(middleware.ErrorHandlingMiddleware(exceptionHandler)) before route registration")
		}
		handlerFunc = root.errorHandler(middleware.ErrorHandler(h))

	default:
		panic("Unsupported handler type: " + typeString(handler) + "\n" +
			"Route: " + method + " " + fullPath + "\n" +
			"Expected one of:\n" +
			"  - http.HandlerFunc\n" +
			"  - func(http.ResponseWriter, *http.Request)\n" +
			"  - func(http.ResponseWriter, *http.Request) error\n" +
			"  - middleware.ErrorHandler")
	}

	route := Route{
		Method:      method,
		Pattern:     finalRegex,
		ParamNames:  paramNames,
		HandlerFunc: handlerFunc,
	}

	// Always append to the root router's routes
	root.routes = append(root.routes, route)
}

func typeString(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

func (r *Router) HandlePrefix(method, prefix string, handler http.Handler) {
	fullPrefix := r.prefix + prefix

	pr := prefixRoute{
		Method:  method,
		Prefix:  fullPrefix,
		Handler: handler,
	}

	// Always append to the root router's prefixRoutes
	root := r.root()
	root.prefixRoutes = append(root.prefixRoutes, pr)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var handler http.Handler = http.HandlerFunc(r.serveRoute)

	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}

	handler.ServeHTTP(w, req)
}

func (r *Router) serveRoute(w http.ResponseWriter, req *http.Request) {
	for _, pr := range r.prefixRoutes {
		if pr.Method == req.Method && strings.HasPrefix(req.URL.Path, pr.Prefix) {
			pr.Handler.ServeHTTP(w, req)
			return
		}
	}

	for _, route := range r.routes {
		if route.Method != req.Method {
			continue
		}
		matches := route.Pattern.FindStringSubmatch(req.URL.Path)
		if matches != nil {
			params := make(map[string]string)
			for i, name := range route.ParamNames {
				params[name] = matches[i+1]
			}
			ctx := context.WithValue(req.Context(), ParamKey, params)
			route.HandlerFunc(w, req.WithContext(ctx))
			return
		}
	}
	http.NotFound(w, req)
}

func GetParam(r *http.Request, key string) string {
	params := r.Context().Value(ParamKey)
	if m, ok := params.(map[string]string); ok {
		return m[key]
	}
	return ""
}
