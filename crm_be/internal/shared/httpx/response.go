// Package httpx holds the conventions every HTTP handler in crm_be follows:
// response envelopes, error mapping, and cross-cutting middleware. See
// docs/architecture/api.md for the contract this package implements.
package httpx

import "github.com/gin-gonic/gin"

// Meta carries pagination metadata for list responses.
type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// OK writes a single-resource response: {"data": ...}.
func OK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// List writes a paginated list response: {"data": [...], "meta": {...}}.
//
// The envelope is present from the first version of the API even though
// MVP list endpoints are internal-only — adding pagination metadata later
// would change the response's root type for every existing client.
func List(c *gin.Context, data any, meta Meta) {
	c.JSON(200, gin.H{"data": data, "meta": meta})
}
