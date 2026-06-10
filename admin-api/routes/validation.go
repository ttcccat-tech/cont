package routes

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// registerCustomValidators registers go-playground/validator custom rules
func init() {
	registerCustomValidators()
}

func registerCustomValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("fqdn", validateFQDN)
		v.RegisterValidation("host_port", validateHostPort)
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}
}

func validateFQDN(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true
	}
	// Allow * prefix for wildcard hosts (*.example.com)
	if strings.HasPrefix(val, "*.") {
		val = val[2:]
	}
	if len(val) > 253 || val == "" {
		return false
	}
	partRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	parts := strings.Split(val, ".")
	for _, part := range parts {
		if part == "" || len(part) > 63 || !partRegex.MatchString(part) {
			return false
		}
	}
	return true
}

func validateHostPort(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true
	}
	colonCount := strings.Count(val, ":")
	if colonCount == 0 {
		return false
	}
	if strings.HasPrefix(val, "[") {
		bracketEnd := strings.Index(val, "]")
		if bracketEnd == -1 || bracketEnd+1 >= len(val) || val[bracketEnd+1] != ':' {
			return false
		}
		portStr := val[bracketEnd+2:]
		return isValidPort(portStr)
	}
	if colonCount > 1 {
		return false
	}
	lastColon := strings.LastIndex(val, ":")
	host := val[:lastColon]
	portStr := val[lastColon+1:]
	if host == "" || portStr == "" {
		return false
	}
	return isValidPort(portStr)
}

func isValidPort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	var port int
	for _, c := range s {
		port = port*10 + int(c-'0')
		if port > 65535 {
			return false
		}
	}
	return port >= 1
}