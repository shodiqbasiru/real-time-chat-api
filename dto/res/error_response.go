package res

type ErrorResponse struct {
	Status     string      `json:"status"`
	StatusCode int         `json:"status_code"`
	Error      interface{} `json:"error"`
}
