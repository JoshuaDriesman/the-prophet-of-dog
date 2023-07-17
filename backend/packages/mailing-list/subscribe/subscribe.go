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
	tx, _ := db.Begin()
	rows, selectErr := tx.Query("SELECT id FROM subscribers WHERE email = $1;", event.Email)
	if selectErr != nil {
		tx.Rollback()
		log.Fatalf("Could not check subscriber %s: %s", event.Email, selectErr)
		return false
	}

	if rows.Next() {
		// If email exists, update it to be subscribed
		var id string
		rowScanErr := rows.Scan(&id)

		if rowScanErr != nil {
			tx.Rollback()
			log.Fatalf("Scan row: %s", rowScanErr)
			return false
		}

		rows.Close()

		_, updateErr := tx.Exec("UPDATE subscribers SET subscribed =  WHERE id=$2;", true, id)

		if updateErr != nil {
			tx.Rollback()
			log.Fatalf("Could not update to subscribe user: %s", updateErr)
			return false
		}

		tx.Commit()
		return true
	}

	// if email does not exist, insert it
	newEmailID := uuid.NewString()
	_, insertErr := tx.Exec("INSERT INTO subscribers (id, email, name, subscribed) VALUES ($1, $2, $3, $4);", newEmailID, event.Email, event.Name, true)
	if insertErr != nil {
		tx.Rollback()
		log.Fatalf("Could not insert subscriber %s: %s", event.Email, insertErr)
		return false
	}

	tx.Commit()
	return true
}
