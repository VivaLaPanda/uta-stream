package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Each token represents a list of endpoints which one is authorized to
// hit
type authMiddleware struct {
	data     authData
	enabled  bool
	basePath string
}

type authData struct {
	TokenRoles map[string][]string `json:"tokenRoles"`
	RoleNames  map[string]string   `json:"roleNames"`
}

// NewAuthMiddleware will prepare the struct which handles state for the
// authorization middleware
func NewAuthMiddleware(authConfigFile string, basePath string) (amw *authMiddleware, err error) {
	data := &authData{}
	authMiddleware := &authMiddleware{*data, false, basePath}
	if authConfigFile == "" {
		return authMiddleware, nil
	}
	configFile, err := os.Open(authConfigFile)
	defer configFile.Close()
	if err != nil {
		return authMiddleware, fmt.Errorf("failed to initialize auth middleware: %v", err)
	}

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(data)
	if err != nil {
		return authMiddleware, fmt.Errorf("failed to parse config file: %v", err)
	}
	authMiddleware.data = *data

	authMiddleware.enabled = true
	return authMiddleware, nil
}

// Authorizer middleware. If auth isn't enabled it'll just pass on the req
// Otherwise it checks the token's routes against the one being requested
func (amw *authMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !amw.enabled {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		route := r.URL.Path

		if amw.ValidateToken(token, route) {
			// Pass down the request to the next middleware (or final handler)
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

func (amw *authMiddleware) ValidateToken(token string, route string) (valid bool) {
	// Check if we have a valid token at all
	if len(token) < 7 || token[:7] != "Bearer " {
		// Apply the default token
		token = "Bearer *"
	}
	token = token[7:]
	if roles, found := amw.data.TokenRoles[token]; found {
		for _, role := range roles {
			// Wildcard perms (basically sudo)
			if role == "*" {
				return true
			}

			if route == amw.basePath+role {
				return true
			}
		}
		return false
	} else {
		// Wildcard role
		for _, role := range amw.data.TokenRoles["*"] {
			if role == "*" {
				return true
			}

			if route == amw.basePath+role {
				return true
			}
		}
		return false
	}
}
