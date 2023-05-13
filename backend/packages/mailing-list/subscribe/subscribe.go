package main

import (
	"context"
	"regexp"
)

type Event struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Response struct {
	Body       string `json:"body"`
	StatusCode int    `json:"statusCode"`
}

func Main(ctx context.Context, event Event) Response {
	if event.Email == "" {
		return Response{
			StatusCode: 400,
			Body:       "Missing email",
		}
	}

	match, _ := regexp.MatchString(".+\@.+\..+", event.Email)

	if !match {
		return Response{
			StatusCode: 400,
			Body: "Email is invalid",
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "Subscribing " + event.Email,
	}
}
