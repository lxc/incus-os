//nolint:unused
package response

// Empty sync response
//
// swagger:response EmptySyncResponse
type swaggerEmptySyncResponse struct {
	// Empty sync response
	// in: body
	Body struct {
		// Example: sync
		Type string `json:"type"`

		// Example: Success
		Status string `json:"status"`

		// Example: 200
		StatusCode int `json:"status_code"`
	}
}

// Bad Request
//
// swagger:response BadRequest
type swaggerBadRequest struct {
	// Bad Request
	// in: body
	Body struct {
		// Example: error
		Type string `json:"type"`

		// Example: bad request
		Error string `json:"error"`

		// Example: 400
		ErrorCode int `json:"error_code"`
	}
}

// Not found
//
// swagger:response NotFound
type swaggerNotFound struct {
	// Not found
	// in: body
	Body struct {
		// Example: error
		Type string `json:"type"`

		// Example: not found
		Error string `json:"error"`

		// Example: 404
		ErrorCode int `json:"error_code"`
	}
}

// Internal Server Error
//
// swagger:response InternalServerError
type swaggerInternalServerError struct {
	// Internal server Error
	// in: body
	Body struct {
		// Example: error
		Type string `json:"type"`

		// Example: internal server error
		Error string `json:"error"`

		// Example: 500
		ErrorCode int `json:"error_code"`
	}
}
