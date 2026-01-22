package middlewares

import (
	"context"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/tokens"
	"log"
	"net/http"
	"strings"
)

func AuthMiddleware(tokenService tokens.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			authHeader := request.Header.Get("Authorization")
			if authHeader == "" {
				response := jsonutil.Response{
					Status:  "error",
					Message: "authorization header required",
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
				return
			}

			tokenString := strings.Split(authHeader, " ")

			if len(tokenString) != 2 || tokenString[0] != "Bearer" {
				response := jsonutil.Response{
					Status:  "error",
					Message: "invalid authorization header format",
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
				return
			}

			data, err := tokenService.DecodeToken(tokenString[1])
			if err != nil {
				response := jsonutil.Response{
					Status:  "error",
					Message: "invalid or expired token",
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
				return
			}

			log.Println(data)
			if !data.Verified {
				response := jsonutil.Response{
					Status:  "error",
					Message: "email not verified",
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusForbidden)
				return
			}

			ctx := context.WithValue(request.Context(), "claims", data)

			newRequest := request.WithContext(ctx)
			next.ServeHTTP(responseWriter, newRequest)
		})
	}
}
