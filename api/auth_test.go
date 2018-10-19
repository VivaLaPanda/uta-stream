package api

import (
	"testing"
)

func TestPopulate(t *testing.T) {
	amw, err := NewAuthMiddleware("", "")
	if err != nil {
		t.Errorf("Err should be nil, was given empty string\n")
	}
	if amw.enabled {
		t.Errorf("Middleware shouldn't enabled, was given empty string\n")
	}

	amw, err = NewAuthMiddleware("foo.bar", "")
	if err == nil {
		t.Errorf("Err should not be nil, was given nonexistent file\n")
	}
	if amw.enabled {
		t.Errorf("Middleware shouldn't enabled, was given nonexistent file\n")
	}

	amw, err = NewAuthMiddleware("test_auth.json", "")
	if err != nil {
		t.Errorf("Err should be nil, valid file was provided\n")
	}
	if !amw.enabled {
		t.Errorf("Middleware should be enabled, was given valid file\n")
	}
	if _, ok := amw.tokenRoles["foo"]; !ok {
		t.Errorf("Roles map should contain foo.\n Struct: %v\n", amw.tokenRoles)
	}
	if len(amw.tokenRoles["foo"]) == 0 {
		t.Errorf("Foo should have at least one associated route.\n Struct: %v\n", amw.tokenRoles)
	}
	if amw.tokenRoles["foo"][0] != "enqueue" {
		t.Errorf("The first route of foo should be 'enqueue'.\n Struct: %v\n", amw.tokenRoles)
	}
}

func TestValidateToken(t *testing.T) {
	amw, err := NewAuthMiddleware("test_auth.json", "")
	if err != nil {
		t.Errorf("Err should be nil, valid file was provided\n")
	}
	if !amw.enabled {
		t.Errorf("Middleware should be enabled, was given valid file\n")
	}

	valid := amw.validateToken("Bearer foo", "enqueue")
	if !valid {
		t.Errorf("Route should match token's \n")
	}
}
