package response

import (
	"encoding/json"
	"net/http"
)

type SuccessResponse struct {
	Data any `json:"data"`
}

func JSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(
		SuccessResponse{
			Data: data,
		},
	)
}