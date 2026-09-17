package http

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

type requestContextKey struct{}
type requestInfo struct {
	principal    domain.Principal
	refreshToken string
	ip           string
}

func requestFrom(ctx context.Context) requestInfo {
	value, _ := ctx.Value(requestContextKey{}).(requestInfo)
	return value
}

func contractMiddleware(contract routers.Router, resolver inbound.PrincipalResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, params, err := contract.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if _, err := url.ParseQuery(r.URL.RawQuery); err != nil {
				writeProblem(w, 400, "Tham số URL không hợp lệ", "validation_failed")
				return
			}
			p, err := resolvePrincipal(r, route.Operation, resolver)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			info := requestInfo{principal: p}
			info.ip, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				info.ip = r.RemoteAddr
			}
			cookies := r.CookiesNamed("refresh_token")
			if len(cookies) > 1 {
				writeProblem(w, 400, "Cookie không hợp lệ", "validation_failed")
				return
			}
			if len(cookies) == 1 {
				info.refreshToken = cookies[0].Value
			}
			if route.Operation.OperationID == "Refresh" && info.refreshToken == "" {
				writeDomainError(w, domain.ErrUnauthenticated)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), requestContextKey{}, info))
			bodyLimit := int64(1 << 20)
			switch route.Operation.OperationID {
			case "ImportSkill", "PreviewToolImport", "ImportTools":
				bodyLimit = 2<<20 + 64<<10
			}
			r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
			err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request: r, PathParams: params, Route: route, Options: &openapi3filter.Options{
					AuthenticationFunc:                openapi3filter.NoopAuthenticationFunc,
					RejectWhenRequestBodyNotSpecified: route.Operation.RequestBody == nil,
				},
			})
			if err != nil {
				writeProblem(w, 400, "Yêu cầu không đúng định dạng", "validation_failed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolvePrincipal(r *http.Request, operation *openapi3.Operation, resolver inbound.PrincipalResolver) (domain.Principal, error) {
	var empty domain.Principal
	authorization, key := r.Header.Get("Authorization"), r.Header.Get("X-API-Key")
	if len(r.Header.Values("Authorization")) > 1 || len(r.Header.Values("X-API-Key")) > 1 || (authorization != "" && key != "") {
		return empty, domain.ErrValidation
	}
	if operation.Security != nil && len(*operation.Security) == 0 {
		return empty, nil
	}
	if operation.OperationID == "Refresh" || operation.OperationID == "Logout" {
		return empty, nil
	}
	if resolver == nil {
		return empty, domain.ErrUnauthenticated
	}
	var p domain.Principal
	var err error
	if key != "" {
		p, err = resolver.FromAPIKey(r.Context(), key)
		if err == nil && !permitsScheme(operation, "apiKeyAuth") {
			err = domain.ErrForbidden
		}
	} else {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return empty, domain.ErrUnauthenticated
		}
		p, err = resolver.FromAccessToken(r.Context(), parts[1])
		if err == nil && !permitsScheme(operation, "bearerAuth") {
			err = domain.ErrForbidden
		}
	}
	if err != nil {
		return empty, err
	}
	if p.MustChangePassword && operation.OperationID != "GetMe" && operation.OperationID != "ChangePassword" {
		return empty, &domain.Error{Kind: domain.ErrForbidden, Detail: "Hãy đổi mật khẩu trước khi sử dụng chức năng này."}
	}
	return p, nil
}

func permitsScheme(operation *openapi3.Operation, scheme string) bool {
	if operation.Security == nil {
		return scheme == "bearerAuth"
	}
	for _, requirement := range *operation.Security {
		if _, ok := requirement[scheme]; ok {
			return true
		}
	}
	return false
}
