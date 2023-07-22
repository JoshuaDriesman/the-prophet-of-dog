package main

import (
	"context"
	"database/sql"
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
		systemErrorResp.Body = err.Error()
		// log.Fatalf("Could not load link %s due to error: %s", event.Link, err.Error())
		return systemErrorResp
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		systemErrorResp.Body = "could not get reader"
		// log.Fatal(err)
		return systemErrorResp
	}

	title := doc.Find("#post__title").First().Text()
	splitPreview := strings.Split(doc.Find(".post__content").First().Text(), ". ")
	if len(splitPreview) < 1 {
		systemErrorResp.Body = "could not parse content"
		// log.Fatalf("Could not parse content")
		return systemErrorResp
	}
	preview := splitPreview[0]

	db, connErr := sql.Open("postgres", os.Getenv("DB_CONNECTION_INFO"))
	if connErr != nil {
		systemErrorResp.Body = connErr.Error()
		// log.Fatalf("Can't connect to DB: %s", connErr)
		return systemErrorResp
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, name, email FROM subscribers WHERE subscribed = true;")
	if err != nil {
		systemErrorResp.Body = err.Error()
		// log.Fatalf("Could not list subscribers: %s", err.Error())
		return systemErrorResp
	}

	subscribers := []Subscriber{}
	for rows.Next() {
		subscriber := Subscriber{
			ID:    "",
			Email: "",
			Name:  "",
		}
		err := rows.Scan(&subscriber.ID, &subscriber.Email, &subscriber.Name)
		if err != nil {
			systemErrorResp.Body = err.Error()
			// log.Fatalf("Could parse row: %s", err.Error())
			return systemErrorResp
		}
		subscribers = append(subscribers, subscriber)
	}

	sendgridApiKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridApiKey == "" {
		systemErrorResp.Body = "could not get api key"
		// log.Fatal("Could not retrieve sendgrid api key")
		return systemErrorResp
	}

	sendgridHost := "https://api.sendgrid.com"
	// sendgridBatchIdRequest := sendgrid.GetRequest(sendgridApiKey, "/v3/mail/batch", sendgridHost)
	// sendgridBatchIdRequest.Method = "POST"
	// sendgridBatchIdResponse, err := sendgrid.API(sendgridBatchIdRequest)
	// if err != nil {
	// 	log.Printf("Could not retrieve sendgrid batch ID: %s", err)
	// 	return systemErrorResp
	// }
	// if sendgridBatchIdResponse.StatusCode != 201 {
	// 	log.Printf("Could not retrieve sendgrid batch ID: %d, %s", sendgridBatchIdResponse.StatusCode, sendgridBatchIdResponse.Body)
	// 	return systemErrorResp
	// }

	// var batchID SendGridBatchIDResponse
	// sendGridUnmarshalErr := json.Unmarshal([]byte(sendgridBatchIdRequest.Body), &batchID)
	// if sendGridUnmarshalErr != nil {
	// 	systemErrorResp.Body = sendGridUnmarshalErr.Error() + "\n" + sendgridBatchIdResponse.Body + "\n" + fmt.Sprintf("%v", batchID)
	// 	// log.Fatalf("Could not unmarshal SendGrid batch ID response: %s", sendGridUnmarshalErr)
	// 	return systemErrorResp
	// }

	sendClient := sendgrid.NewSendClient(sendgridApiKey)
	sentCount := 0
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
		// sendgridMail.BatchID = batchID.BatchID

		response, err := sendClient.Send(sendgridMail)
		responses = append(responses, fmt.Sprintf("%v", *sendgridMail.Personalizations[0].To[0]))

		if err != nil || response.StatusCode != 202 {
			systemErrorResp.Body = fmt.Sprintf("%v", sendgridMail)
			log.Printf("Could not send message: %s", err)
			err = nil
		}
		sentCount += 1
	}

	return Response{
		StatusCode: 200,
		Body:       fmt.Sprintf("success %d, %v, %v", sentCount, responses, subscribers),
	}
}
