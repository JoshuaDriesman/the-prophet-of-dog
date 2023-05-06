package main

import "context"

type Event struct {
	Email string `json:"email"`
}

type Response struct {
	Body       string `json:"body"`
	StatusCode int 	  `json:"statusCode"`
}

func Main(ctx context.Context, event Event) Response {
	msg := make(map[string]interface{})
	if event.Email == "" {
		return Response{
			StatusCode: 400,
			Body: "Missing email"
		}
	}

	return Response{
		Body: "Subscribing " + event.Email,
	}
}
