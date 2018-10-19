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
	tokenRoles map[string][]string
	enabled    bool
	basePath   string
}

// Initialize it somewhere
func NewAuthMiddleware(authConfigFile string, basePath string) (amw *authMiddleware, err error) {
	var tokenRoles map[string][]string
	authMiddleware := &authMiddleware{tokenRoles, false, basePath}
	if authConfigFile == "" {
		return authMiddleware, nil
	}
	configFile, err := os.Open(authConfigFile)
	defer configFile.Close()
	if err != nil {
		return authMiddleware, fmt.Errorf("failed to initialize auth middleware: %v", err)
	}
	// Get the json out of the file
	var rootJson map[string]*json.RawMessage
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&rootJson)
	if err != nil {
		return authMiddleware, fmt.Errorf("failed to initialize auth middleware: %v", err)
	}

	// Assign the json to the map
	err = json.Unmarshal(*rootJson["tokenRoles"], &tokenRoles)
	if err != nil {
		return authMiddleware, fmt.Errorf("failed to initialize auth middleware: %v", err)
	}

	authMiddleware.enabled = true
	authMiddleware.tokenRoles = tokenRoles
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

		if amw.validateToken(token, route) {
			// Pass down the request to the next middleware (or final handler)
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

func (amw *authMiddleware) validateToken(token string, route string) (valid bool) {
	if token[:7] != "Bearer " {
		return false
	}
	token = token[7:]
	if roles, found := amw.tokenRoles[token]; found {
		for _, role := range roles {
			fmt.Println("route:", route, "pathandrole", amw.basePath+role)
			if route == amw.basePath+role {
				return true
			}
		}
		return false
	} else {
		return false
	}
}
