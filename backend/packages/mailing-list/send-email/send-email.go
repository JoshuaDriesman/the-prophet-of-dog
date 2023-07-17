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
	Link     string `json:"link"`
	Passcode string `json:"passcode"`
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

	if event.Passcode != "bichybichybichy" {
		return Response{
			Body:       "Not Authorized",
			StatusCode: 401,
		}
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
		systemErrorResp.Body = err.Error()
		return systemErrorResp
	}
	log.Println("Got article")
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
		return systemErrorResp
	}
	log.Println("Created reader")

	title := doc.Find("#post__title").First().Text()
	splitPreview := strings.Split(doc.Find(".post__content").First().Text(), ". ")
	if len(splitPreview) < 1 {
		log.Fatalf("Could not parse content")
		return systemErrorResp
	}
	preview := splitPreview[0]
	log.Println("Parsed article")

	db, connErr := sql.Open("postgres", os.Getenv("DB_CONNECTION_INFO"))
	if connErr != nil {
		log.Fatalf("Can't connect to DB: %s", connErr)
		systemErrorResp.Body = connErr.Error()
		return systemErrorResp
	}
	defer db.Close()
	log.Println("Opened DB conn")

	rows, err := db.Query("SELECT id, name, email FROM subscribers WHERE subscribed = true;")
	if err != nil {
		log.Fatalf("Could not list subscribers: %s", err.Error())
		systemErrorResp.Body = err.Error()
		return systemErrorResp
	}
	log.Println("Got subs")

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
			systemErrorResp.Body = err.Error()
			return systemErrorResp
		}
		subscribers = append(subscribers, subscriber)
	}
	log.Println("Read subs")

	sendgridApiKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridApiKey == "" {
		log.Fatal("Could not retrieve sendgrid api key")
		return systemErrorResp
	}

	sendgridHost := "https://api.sendgrid.com"
	sendgridBatchIdRequest := sendgrid.GetRequest(sendgridApiKey, "/v3/mail/batch", sendgridHost)
	sendgridBatchIdRequest.Method = "POST"
	sendgridBatchIdResponse, err := sendgrid.API(sendgridBatchIdRequest)
	log.Println("Got batch ID")
	if err != nil {
		log.Fatalf("Could not retrieve sendgrid batch ID: %s", err)
		systemErrorResp.Body = err.Error()
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
		systemErrorResp.Body = sendGridUnmarshalErr.Error()
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
		sendgridMail.BatchID = batchID.BatchID

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
