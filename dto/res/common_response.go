package res

type CommonResponse[T any] struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
	Data       T      `json:"data"`
}
