package api

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/audit"
	"github.com/Euphrase8/DevMail/backend/internal/auth"
	"github.com/Euphrase8/DevMail/backend/internal/dns"
	"github.com/Euphrase8/DevMail/backend/internal/domains"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/Euphrase8/DevMail/backend/internal/jobs"
	"github.com/Euphrase8/DevMail/backend/internal/mailboxes"
	"github.com/Euphrase8/DevMail/backend/internal/monitoring"
	"github.com/Euphrase8/DevMail/backend/internal/providers"
	"github.com/Euphrase8/DevMail/backend/internal/servers"
	"github.com/gin-gonic/gin"
)

type schema = map[string]any

func typeSchema(t reflect.Type) schema {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == reflect.TypeOf(time.Time{}) {
		return schema{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.String:
		return schema{"type": "string"}
	case reflect.Bool:
		return schema{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return schema{"type": "integer"}
	case reflect.Float64:
		return schema{"type": "number"}
	case reflect.Slice:
		return schema{"type": "array", "items": typeSchema(t.Elem())}
	case reflect.Struct:
		properties := schema{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name != "" && name != "-" {
				properties[name] = typeSchema(field.Type)
			}
		}
		return schema{"type": "object", "properties": properties}
	default:
		return schema{}
	}
}
func objectFields(fields map[string]string, required ...string) schema {
	properties := schema{}
	for name, kind := range fields {
		properties[name] = schema{"type": kind}
	}
	return schema{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func requestSchema(path, method string) schema {
	switch {
	case path == "/auth/setup":
		return objectFields(map[string]string{"email": "string", "name": "string", "password": "string"}, "email", "password")
	case path == "/auth/login":
		return objectFields(map[string]string{"email": "string", "password": "string", "code": "string"}, "email", "password")
	case path == "/auth/password":
		return objectFields(map[string]string{"currentPassword": "string", "newPassword": "string"}, "currentPassword", "newPassword")
	case path == "/auth/totp/enroll":
		return objectFields(map[string]string{"password": "string"}, "password")
	case path == "/auth/totp/verify":
		return objectFields(map[string]string{"code": "string"}, "code")
	case path == "/auth/totp/disable":
		return objectFields(map[string]string{"password": "string", "code": "string"}, "password", "code")
	case strings.HasSuffix(path, "/maintenance"):
		return objectFields(map[string]string{"enabled": "boolean"}, "enabled")
	case path == "/servers" || path == "/servers/{id}":
		return objectFields(map[string]string{"name": "string", "hostname": "string", "ipv4": "string", "ipv6": "string", "location": "string", "environment": "string", "notes": "string"}, "name", "hostname")
	case path == "/domains" || path == "/domains/{id}":
		required := []string{}
		if method == "POST" {
			required = []string{"name"}
		}
		return objectFields(map[string]string{"name": "string", "serverId": "string", "quotaBytes": "integer", "catchAll": "boolean"}, required...)
	case strings.HasSuffix(path, "/mailboxes") || path == "/mailboxes/{mailbox_id}":
		required := []string{}
		if method == "POST" {
			required = []string{"local", "password"}
		}
		return objectFields(map[string]string{"local": "string", "displayName": "string", "quotaBytes": "integer", "password": "string"}, required...)
	case strings.HasSuffix(path, "/aliases"):
		return objectFields(map[string]string{"source": "string", "destination": "string", "forwarding": "boolean"}, "source", "destination")
	case path == "/users" || path == "/users/{id}":
		return objectFields(map[string]string{"email": "string", "name": "string", "role": "string", "password": "string"}, "name", "role")
	case path == "/settings":
		return objectFields(map[string]string{"name": "string", "timezone": "string", "webmail": "string", "mailEngine": "string", "defaultQuotaBytes": "integer"})
	case path == "/providers" || path == "/providers/{id}":
		return objectFields(map[string]string{"name": "string", "kind": "string", "zone": "string", "token": "string"}, "name", "kind")
	case strings.HasSuffix(path, "/dkim"):
		return objectFields(map[string]string{"selector": "string"})
	case strings.HasSuffix(path, "/dns-records/apply"):
		return objectFields(map[string]string{"providerId": "string"}, "providerId")
	case path == "/jobs":
		return schema{"type": "object", "required": []string{"kind", "payload"}, "additionalProperties": false, "properties": schema{"kind": schema{"type": "string", "enum": []string{"domain.provision", "domain.remove", "dns.apply", "mailbox.create", "mailbox.password", "mailbox.delete", "alias.provision", "alias.delete"}}, "payload": schema{"type": "object", "additionalProperties": schema{"type": "string"}}}}
	case strings.HasPrefix(path, "/routing-rules") || strings.HasPrefix(path, "/backups") || strings.HasPrefix(path, "/notifications"):
		if path == "/routing-rules" || path == "/routing-rules/{id}" || path == "/backups" || path == "/backups/{id}" || path == "/notifications" || path == "/notifications/{id}" {
			return objectFields(map[string]string{"name": "string", "enabled": "boolean", "config": "object"}, "name", "config")
		}
	}
	return nil
}
func OpenAPI(r *gin.Engine) schema {
	paths := schema{}
	param := regexp.MustCompile(`:([a-zA-Z_]+)`)
	for _, route := range r.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		path := param.ReplaceAllString(strings.TrimPrefix(route.Path, "/api/v1"), "{$1}")
		method := strings.ToLower(route.Method)
		operation := schema{"summary": route.Method + " " + path, "operationId": strings.ToLower(route.Method) + strings.NewReplacer("/", "_", "{", "", "}", "").Replace(path)}
		parameters := []any{}
		for _, match := range param.FindAllStringSubmatch(route.Path, -1) {
			parameters = append(parameters, schema{"name": match[1], "in": "path", "required": true, "schema": schema{"type": "string", "format": "uuid"}})
		}
		if route.Method == "GET" {
			for _, name := range []string{"page", "perPage", "q", "status", "sort", "order"} {
				kind := "string"
				if name == "page" || name == "perPage" {
					kind = "integer"
				}
				parameters = append(parameters, schema{"name": name, "in": "query", "required": false, "schema": schema{"type": kind}, "description": "Collection endpoints support this query parameter."})
			}
		}
		if route.Method != "GET" {
			parameters = append(parameters, schema{"name": "X-CSRF-Token", "in": "header", "required": true, "schema": schema{"type": "string"}})
		}
		operation["parameters"] = parameters
		public := strings.HasPrefix(path, "/health") || path == "/auth/status" || path == "/auth/setup" || path == "/auth/login" || path == "/auth/csrf" || path == "/openapi.json"
		if !public {
			operation["security"] = []any{schema{"sessionCookie": []string{}}}
		}
		if body := requestSchema(path, route.Method); body != nil && (route.Method == "POST" || route.Method == "PATCH") {
			operation["requestBody"] = schema{"required": true, "content": schema{"application/json": schema{"schema": body}}}
		}
		responses := schema{}
		for _, code := range []string{"200", "201", "202"} {
			responses[code] = schema{"description": "Successful response (creation uses 201; queued domain provisioning uses 202)", "content": schema{"application/json": schema{"schema": schema{"$ref": "#/components/schemas/Envelope"}}}}
		}
		for _, code := range []string{"400", "401", "403", "404", "409", "422", "429", "500", "503"} {
			responses[code] = schema{"description": "Error response", "content": schema{"application/json": schema{"schema": schema{"$ref": "#/components/schemas/Error"}}}}
		}
		operation["responses"] = responses
		item, ok := paths[path].(schema)
		if !ok {
			item = schema{}
			paths[path] = item
		}
		item[method] = operation
	}
	models := schema{"Envelope": schema{"type": "object", "required": []string{"data"}, "properties": schema{"data": schema{}, "meta": objectFields(map[string]string{"page": "integer", "perPage": "integer", "total": "integer"}, "page", "perPage", "total")}}, "Error": schema{"type": "object", "required": []string{"error"}, "properties": schema{"error": objectFields(map[string]string{"code": "string", "message": "string", "fields": "object"}, "code", "message")}}}
	for name, model := range map[string]any{"User": auth.User{}, "Server": servers.Server{}, "Domain": domains.Domain{}, "Mailbox": mailboxes.Mailbox{}, "Alias": mailboxes.Alias{}, "Provider": providers.Provider{}, "DnsRecord": dns.Record{}, "Job": jobs.Job{}, "AuditEntry": audit.Entry{}, "Dashboard": monitoring.Summary{}, "OpsResource": httpapi.ResourceItem{}} {
		models[name] = typeSchema(reflect.TypeOf(model))
	}
	return schema{"openapi": "3.0.3", "info": schema{"title": "DevMail API", "version": "1.0.0", "description": "Versioned workspace API. Cookie authentication, signed double-submit CSRF, server-side roles. Lists use page/perPage/q/status/sort/order; audit also accepts action; telemetry uses limit/kind. External mail delivery and DNS credentials are prerequisites."}, "servers": []any{schema{"url": "/api/v1"}}, "paths": paths, "components": schema{"schemas": models, "securitySchemes": schema{"sessionCookie": schema{"type": "apiKey", "in": "cookie", "name": "dm_session"}}}}
}
