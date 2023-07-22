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
	logger := log.New(os.Stdout, "pog: ", log.Ldate)
	systemErrorResp := Response{
		Body:       "",
		StatusCode: 400,
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
		logger.Printf("Could not load link %s due to error: %s", event.Link, err.Error())
		return systemErrorResp
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Print(err.Error())
		return systemErrorResp
	}

	title := doc.Find("#post__title").First().Text()
	splitPreview := strings.Split(doc.Find(".post__content").First().Text(), ". ")
	if len(splitPreview) < 1 {
		logger.Print("Could not parse content")
		return systemErrorResp
	}
	preview := splitPreview[0]

	db, connErr := sql.Open("postgres", os.Getenv("DB_CONNECTION_INFO"))
	if connErr != nil {
		logger.Printf("Can't connect to DB: %s", connErr)
		return systemErrorResp
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, name, email FROM subscribers WHERE subscribed = true;")
	if err != nil {
		logger.Printf("Could not list subscribers: %s", err.Error())
		return systemErrorResp
	}

	subscribers := []Subscriber{}
	for rows.Next() {
		subscriber := Subscriber{
			ID:    "",
			Name:  "",
			Email: "",
		}
		err := rows.Scan(&subscriber.ID, &subscriber.Name, &subscriber.Email)
		if err != nil {
			logger.Printf("Could parse row: %s", err.Error())
			return systemErrorResp
		}
		subscribers = append(subscribers, subscriber)
	}

	sendgridApiKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridApiKey == "" {
		logger.Print("Could not retrieve sendgrid api key")
		return systemErrorResp
	}

	sendgridHost := "https://api.sendgrid.com"
	sendgridBatchIdRequest := sendgrid.GetRequest(sendgridApiKey, "/v3/mail/batch", sendgridHost)
	sendgridBatchIdRequest.Method = "POST"
	sendgridBatchIdResponse, err := sendgrid.API(sendgridBatchIdRequest)
	if err != nil {
		logger.Printf("Could not retrieve sendgrid batch ID: %s", err)
		return systemErrorResp
	}
	if sendgridBatchIdResponse.StatusCode != 201 {
		logger.Printf("Could not retrieve sendgrid batch ID: %d, %s", sendgridBatchIdResponse.StatusCode, sendgridBatchIdResponse.Body)
		return systemErrorResp
	}

	var batchIDRes SendGridBatchIDResponse
	cleanedBody := strings.TrimSpace(sendgridBatchIdResponse.Body)
	sendGridUnmarshalErr := json.Unmarshal([]byte(cleanedBody), &batchIDRes)
	if sendGridUnmarshalErr != nil {
		systemErrorResp.Body = sendGridUnmarshalErr.Error() + "\n" + sendgridBatchIdResponse.Body
		logger.Printf("Could not unmarshal SendGrid batch ID response: %s", sendGridUnmarshalErr)
		return systemErrorResp
	}

	sendClient := sendgrid.NewSendClient(sendgridApiKey)
	responses := []string{}
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

		sendgridMail.AddPersonalizations(personalization)
		// sendgridMail.BatchID = batchIDRes.BatchID

		response, err := sendClient.Send(sendgridMail)
		responses = append(responses, response.Body)

		if err != nil || response.StatusCode != 202 {
			logger.Printf("Could not send message: %s", err)
			err = nil
		}
	}

	return Response{
		StatusCode: 200,
		Body:       "success",
	}
}
