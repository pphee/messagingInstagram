package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Attachment struct {
	Type    string  `json:"type"`
	Payload Payload `json:"payload"`
}

type Payload struct {
	URL string `json:"url,omitempty"`
}

type Message struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	IsEcho      bool         `json:"is_echo,omitempty"`
}

type WebhookBody struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	Messaging []Messaging `json:"messaging"`
}

type Messaging struct {
	Sender    Sender    `json:"sender"`
	Recipient Recipient `json:"recipient"`
	Message   Message   `json:"message"`
}

type Sender struct {
	ID string `json:"id"`
}

type Recipient struct {
	ID string `json:"id"`
}

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
	}
}

func sendCustomerAMessage(pageID, response, pageToken, psid string) error {
	reqBody := fmt.Sprintf(`{"recipient":{"id":"%s"},"message":{"text":"%s"}}`, psid, response)
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages?access_token=%s", pageID, pageToken)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(reqBody))
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to execute request: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("HTTP Error: %s\nResponse Body: %s\n", resp.Status, string(body))
		return fmt.Errorf("HTTP Error: %s, Body: %s", resp.Status, string(body))
	}

	return nil
}

func sendMediaMessage(recipientID, mediaURL, mediaType, pageAccessToken string) error {
	var reqBody string

	reqBody = fmt.Sprintf(`{
        "recipient":{"id":"%s"},
        "message":{
            "attachment":{
                "type":"%s",
                "payload":{
                    "url":"%s",
                    "is_reusable":true
                }
            }
        }
    }`, recipientID, mediaType, mediaURL)

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/me/messages?access_token=%s", pageAccessToken)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP Error: %s, Body: %s", resp.Status, string(body))
	}

	return nil
}

func main() {
	r := gin.Default()

	r.GET("/webhook/messaging-webhook", func(c *gin.Context) {
		mode := c.Query("hub.mode")
		token := c.Query("hub.verify_token")
		challenge := c.Query("hub.challenge")

		verifyToken := os.Getenv("VERIFY_TOKEN")
		if mode == "subscribe" && token == verifyToken {
			c.String(http.StatusOK, challenge)
		} else {
			c.AbortWithStatus(http.StatusForbidden)
		}
	})

	r.POST("/webhook/messaging-webhook", func(c *gin.Context) {
		var body WebhookBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if body.Object == "instagram" {
			pageID := os.Getenv("PAGE_ID")
			pageAccessToken := os.Getenv("PAGE_ACCESS_TOKEN")
			for _, entry := range body.Entry {
				for _, messaging := range entry.Messaging {
					if messaging.Message.IsEcho {
						log.Printf("Ignoring echo message")
						continue
					}
					senderID := messaging.Sender.ID
					if messaging.Message.Text != "" {
						err := sendCustomerAMessage(pageID, messaging.Message.Text, pageAccessToken, senderID)
						if err != nil {
							c.JSON(http.StatusOK, gin.H{"status": "failed to send message"})
							return
						}
					}
					for _, attachment := range messaging.Message.Attachments {
						mediaURL := attachment.Payload.URL
						switch attachment.Type {
						case "image":
							if mediaURL == "" {
								mediaURL = "https://i.gifer.com/Ifph.gif"
							}
						case "video", "file", "audio":
							if mediaURL == "" {
								fmt.Printf("Error: No URL found for %s attachment\n", attachment.Type)
								continue
							}
						default:
							fmt.Printf("Received unsupported attachment type: %s\n", attachment.Type)
							continue
						}

						err := sendMediaMessage(senderID, mediaURL, attachment.Type, pageAccessToken)
						if err != nil {
							fmt.Printf("Error sending %s message: %v\n", attachment.Type, err)
						}
					}

				}
			}
			c.String(http.StatusOK, "EVENT_RECEIVED")
		} else {
			c.AbortWithStatus(http.StatusNotFound)
		}
	})

	err := r.Run(":8080")
	if err != nil {
		fmt.Println("Error running server:", err)
	}
}
