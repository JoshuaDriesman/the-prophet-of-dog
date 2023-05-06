package main

import "context"

type Event struct {
	Email string `json:"email"`
}

type Response struct {
	Body string `json:"body"`
}

func Main(ctx context.Context, event Event) Response {
	msg := make(map[string]interface{})
	if event.Email == "" {
		msg["statusCode"] = 400
		msg["body"] = "Missing email"
	}

	return Response{
		Body: "Subscribing " + event.Email,
	}
}
