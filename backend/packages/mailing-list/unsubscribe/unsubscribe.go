package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type Event struct {
	ID string `json:"id"`
}

type Response struct {
	Body       string `json:"body"`
	StatusCode int    `json:"statusCode"`
}

func Main(ctx context.Context, event Event) Response {
	if event.ID == "" {
		return Response{
			StatusCode: 400,
			Body:       "Missing ID",
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

	fmt.Printf("%s\n", event.ID)
	_, updateErr := db.Exec("UPDATE subscribers SET subscribed = $1 WHERE id=$2", false, event.ID)

	if updateErr != nil {
		log.Fatalf("Failed to unsubscribe %s: %s", event.ID, updateErr)
		return Response{
			StatusCode: 500,
			Body:       "Failed to unsubscribe",
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "",
	}
}
