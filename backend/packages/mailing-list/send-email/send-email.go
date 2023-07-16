package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/lib/pq"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type Event struct {
	Link string `json:"link"`
}

type Response struct {
	Body       string `json:"body"`
	StatusCode int    `json:"statusCode"`
}

type Subscriber struct {
	ID    string
	Email string
	Name  string
}

type SendGridBatchIDResponse struct {
	BatchID string `json:"batch_id"`
}

func Main(ctx context.Context, event Event) Response {
	systemErrorResp := Response{
		Body:       "",
		StatusCode: 500,
	}

	if event.Link == "" {
		return Response{
			Body:       "Link is required",
			StatusCode: 400,
		}
	}

	resp, err := http.Get(event.Link)
	if err != nil {
		log.Fatalf("Could not load link %s due to error: %s", event.Link, err.Error())
		return systemErrorResp
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
		return systemErrorResp
	}

	title := doc.Find("#post__title").First().Text()
	splitPreview := strings.Split(doc.Find(".post__content").First().Text(), ". ")

	if len(splitPreview) < 1 {
		log.Fatalf("Could not parse content")
		return systemErrorResp
	}
	preview := splitPreview[0]

	db, connErr := sql.Open("postgres", os.Getenv("DB_CONNECTION_INFO"))
	if connErr != nil {
		log.Fatalf("Can't connect to DB: %s", connErr)
		return systemErrorResp
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, name, email FROM subscribers WHERE subscribed = true;")
	if err != nil {
		log.Fatalf("Could not list subscribers: %s", err.Error())
		return systemErrorResp
	}

	subscribers := []Subscriber{}
	if rows.Next() {
		subscriber := Subscriber{
			ID:    "",
			Email: "",
			Name:  "",
		}
		err := rows.Scan(subscriber.ID, subscriber.Email, subscriber.Name)
		if err != nil {
			log.Fatalf("Could parse row: %s", err.Error())
			return systemErrorResp
		}
		subscribers = append(subscribers, subscriber)
	}

	sendgridApiKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridApiKey == "" {
		log.Fatal("Could not retrieve sendgrid api key")
		return systemErrorResp
	}

	sendgridHost := "https://api.sendgrid.com"
	sendgridBatchIdRequest := sendgrid.GetRequest(sendgridApiKey, "/v3/mail/batch", sendgridHost)
	sendgridBatchIdRequest.Method = "POST"
	sendgridBatchIdResponse, err := sendgrid.API(sendgridBatchIdRequest)
	if sendgridApiKey == "" {
		log.Fatalf("Could not retrieve sendgrid batch ID: %s", err)
		return systemErrorResp
	}
	if sendgridBatchIdResponse.StatusCode != 200 {
		log.Fatal("Could not retrieve sendgrid batch ID")
		return systemErrorResp
	}

	var batchID SendGridBatchIDResponse
	sendGridUnmarshalErr := json.Unmarshal(sendgridBatchIdRequest.Body, &batchID)
	if sendGridUnmarshalErr != nil {
		log.Fatalf("Could not unmarshal SendGrid batch ID response: %s", sendGridUnmarshalErr)
		return systemErrorResp
	}

	sendClient := sendgrid.NewSendClient(sendgridApiKey)
	for _, subscriber := range subscribers {
		sendgridSendEmailRequest := sendgrid.GetRequest(sendgridApiKey, "/v3/mail/send", sendgridHost)
		sendgridSendEmailRequest.Method = "POST"

		sendgridMail := mail.NewV3Mail()
		email := mail.NewEmail("The Prophet of Dog", "eliyahu@theprophetofdog.com")
		sendgridMail.SetFrom(email)

		sendgridMail.TemplateID = "d-badb40dfb59b4bb5b4ffba15e25b077d"

		personalization := mail.NewPersonalization()
		personalization.AddTos(
			mail.NewEmail(subscriber.Name, subscriber.Email),
		)
		personalization.SetDynamicTemplateData("subject", title)
		personalization.SetDynamicTemplateData("title", title)
		personalization.SetDynamicTemplateData("preview", preview)
		personalization.SetDynamicTemplateData("link", event.Link)
		personalization.SetDynamicTemplateData("unsubscribe", fmt.Sprintf("https://theprophetofdog.com/api/mailing-list/unsubscribe?id=%s", subscriber.ID))

		sendgridMail.Personalizations = append(sendgridMail.Personalizations, personalization)

		response, err := sendClient.Send(sendgridMail)
		log.Println(response)

		if err != nil {
			log.Printf("Could not send message: %s", err)
			err = nil
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "success",
	}
}
