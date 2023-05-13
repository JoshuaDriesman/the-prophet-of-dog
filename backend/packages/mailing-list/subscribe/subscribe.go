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

	successSubscribed := upsertSubscriber(event, db)

	if !successSubscribed {
		return Response{
			StatusCode: 500,
			Body:       "Failed to subscribe user",
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "Subscribing " + event.Email,
	}
}

func upsertSubscriber(event Event, db *sql.DB) bool {
	// Check if email already exists
	rows, selectErr := db.Query("SELECT id FROM subscribers WHERE email = $1;", event.Email)
	if selectErr != nil {
		log.Fatalf("Could not check subscriber %s: %s", event.Email, selectErr)
		return false
	}
	defer rows.Close()
	if rows.Next() {
		// If email exists, update it to be subscribed
		var id string
		rowScanErr := rows.Scan(&id)

		if rowScanErr != nil {
			log.Fatalf("Scan row: %s", rowScanErr)
			return false
		}

		db.Query("UPDATE subscribers SET subscribed = $1 WHERE id=$2;", true, id)

		return true
	}

	// if email does not exist, insert it
	newEmailID := uuid.NewString()
	_, insertErr := db.Exec("INSERT INTO subscribers (id, email, name, subscribed) VALUES ($1, $2, $3, $4);", newEmailID, event.Email, event.Name, true)
	if insertErr != nil {
		log.Fatalf("Could not insert subscriber %s: %s", event.Email, insertErr)
		return false
	}

	return true
}
