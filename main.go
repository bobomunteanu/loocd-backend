package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

type TimerWithId struct {
	PhoneNumber string `json:"phoneNumber"`
	Timestamp   int64  `json:"timestamp"`
	Uid         string `json:"uid"`
	Time        string `json:"time"`
	Message     string `json:"message"`
}

type Timer struct {
	PhoneNumber string `json:"phoneNumber"`
	Timestamp   int64  `json:"timestamp"`
	Time        string `json:"time"`
	Message     string `json:"message"`
}

type ID struct {
	ID string `json:"ID"`
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":3000"
	} else {
		port = ":" + port
	}

	return port
}

func parseDuration(duration string) (time.Duration, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return 0, err
	}

	if dur < time.Minute || dur > 24*time.Hour {
		return 0, fmt.Errorf("invalid duration: %s", duration)
	}

	return dur, nil
}

func sendSMS(phonenumber string, message string) {
	client := twilio.NewRestClient()

	params := &api.CreateMessageParams{}
	params.SetBody(message)
	params.SetFrom("+19143862951")
	params.SetTo(phonenumber)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		if resp.Sid != nil {
			fmt.Println(*resp.Sid)
		} else {
			fmt.Println(resp.Sid)
		}
	}
}

func checkExpiredUsers() {
	// Create a new HTTP request with the GET method and URL
	req, err := http.NewRequest("GET", "https://loocd-d2ff8-default-rtdb.europe-west1.firebasedatabase.app/users.json", nil)
	if err != nil {
		fmt.Println("Error creating HTTP request:", err)
		return
	}

	// Send the HTTP request using the default HTTP client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending HTTP request:", err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading HTTP response body:", err)
		return
	}

	// Unmarshal the response body into a map of User objects
	var timerMap map[string]Timer
	err = json.Unmarshal(body, &timerMap)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
	}

	now := time.Now()

	// Print the user objects
	for id, timer := range timerMap {
		// Calculate the time difference in minutes
		diff := int(now.Sub(time.Unix(timer.Timestamp, 0)).Minutes())
		//timeDiff := int(now.Sub(time.Unix(timer.Timestamp, 0)).Seconds() / 60)

		treshold, _ := strconv.Atoi(timer.Time)

		// Check if the time difference is greater than the threshold
		if diff < treshold {
			fmt.Printf("User with ID %s has not checked in for %d minutes.\n", id, diff)
		} else {
			sendSMS(timer.PhoneNumber, timer.Message)

			url := fmt.Sprint("https://loocd-d2ff8-default-rtdb.europe-west1.firebasedatabase.app/users/" + id + ".json")

			req, err := http.NewRequest(http.MethodDelete, url, nil)
			if err != nil {
				fmt.Println(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println(err)
			}
			defer resp.Body.Close()
		}
	}

}

func main() {

	// Periodic Expired User Check
	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				checkExpiredUsers()
			}
		}
	}()

	// HTTP Server
	server := fiber.New()

	server.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, world!")
	})

	server.Post("/remove-timer", func(c *fiber.Ctx) error {
		var id ID
		if err := c.BodyParser(&id); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}

		url := fmt.Sprint("https://loocd-d2ff8-default-rtdb.europe-west1.firebasedatabase.app/users/" + id.ID + ".json")

		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			fmt.Println(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()

		return nil
	})

	server.Put("/add-timer", func(c *fiber.Ctx) error {
		// Parse request body into User struct
		var timerwithid TimerWithId
		if err := c.BodyParser(&timerwithid); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}

		url := fmt.Sprint("https://loocd-d2ff8-default-rtdb.europe-west1.firebasedatabase.app/users/" + timerwithid.Uid + ".json")
		fmt.Println(url)
		timer := Timer{timerwithid.PhoneNumber, timerwithid.Timestamp, timerwithid.Time, timerwithid.Message}

		// Encode user to JSON
		jsonData, err := json.Marshal(timer)
		if err != nil {
			panic(err)
		}

		// Create HTTP request with JSON body
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Send request and print response
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		return nil
	})

	//go func() {
	if err := server.Listen(getPort()); err != nil {
		log.Fatalf("error starting server: %v\n", err)
	}
	//}()

	//<-done
}
