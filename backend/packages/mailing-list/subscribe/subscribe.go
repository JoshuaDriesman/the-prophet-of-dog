package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"regexp"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
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

	match, _ := regexp.MatchString(".+\\@.+\\..+", event.Email)

	if !match {
		return Response{
			StatusCode: 400,
			Body:       "Email is invalid",
		}
	}

	db, connErr := sql.Open("postgres", os.Getenv("DB_CONNECTION_INFO"))
	if connErr != nil {
		log.Fatalf("Can't connect to DB: %s", connErr)
		return Response{
			StatusCode: 500,
			Body:       "DB connection error",
		}
	}
	defer db.Close()

	// Check if email already exists
	rows, selectErr := db.Query("SELECT * FROM subscribers WHERE email = %s;", event.Email)
	if selectErr != nil {
		log.Fatalf("Could not check subscriber %s: %s", event.Email, selectErr)
		return Response{
			StatusCode: 500,
			Body:       "DB query error",
		}
	}
	defer rows.Close()
	if rows.Next() {
		return Response{
			StatusCode: 400,
			Body:       "Email already exists.",
		}
	}

	newEmailID := uuid.NewString()
	_, insertErr := db.Exec("INSERT INTO subscribers (id, email, name, subscribed) VALUES (%s, %s, %s, %t);", newEmailID, event.Email, event.Name, true)
	if insertErr != nil {
		log.Fatalf("Could not insert subscriber %s: %s", event.Email, connErr)
		return Response{
			StatusCode: 500,
			Body:       "DB query error",
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "Subscribing " + event.Email,
	}
}
