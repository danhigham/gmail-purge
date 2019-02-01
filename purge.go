package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

func getValueFromCredhub(credhubURL string, credhubCaCert string, uaaCaCert string, credhubUser string, credhubPassword string, path string) (string, error) {

	ch, err := credhub.New(credhubURL,
		credhub.CaCerts(credhubCaCert, uaaCaCert),
		credhub.Auth(auth.UaaClientCredentials(credhubUser, credhubPassword)),
		// credhub.SkipTLSValidation(true),
	)

	token, err := ch.GetLatestValue(path)
	if err != nil {
		return "", err
	}

	return string(token.Value), nil
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Retrieve a token, saves the token, then returns the generated client.
func getClientWithTokenString(config *oauth2.Config, token string) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, _ := tokenFromString(token)
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromString(tokenString string) (*oauth2.Token, error) {
	tok := &oauth2.Token{}
	err := json.NewDecoder(strings.NewReader(tokenString)).Decode(tok)
	return tok, err
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func main() {
	showHeaders := flag.Bool("show-headers", false, "Show headers of messages")
	pageSize := flag.Int("page-size", 500, "Page size for messages")
	dryRun := flag.Bool("dry-run", false, "Do a dry run without modifying messages")
	saveToken := flag.Bool("save-token", false, "Save token")

	credhubURL := flag.String("credhub-url", "", "Credhub URL")
	credhubCaCert := os.Getenv("CREDHUB_CA_CERT")
	uaaCaCert := os.Getenv("UAA_CA_CERT")
	credhubUsername := os.Getenv("CREDHUB_CLIENT")
	credhubPassword := os.Getenv("CREDHUB_SECRET")
	tokenPath := flag.String("gmail-token-path", "", "Gmail Token Path")
	credsPath := flag.String("gmail-creds-path", "", "Gmail Creds Path")

	flag.Parse()

	creds, err := getValueFromCredhub(*credhubURL, uaaCaCert, credhubCaCert, credhubUsername, credhubPassword, *credsPath)
	if err != nil {
		log.Fatalf("Unable to read credentials: %v", err)
	}
	token, err := getValueFromCredhub(*credhubURL, uaaCaCert, credhubCaCert, credhubUsername, credhubPassword, *tokenPath)
	if err != nil {
		log.Fatalf("Unable to read token: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON([]byte(creds), fmt.Sprintf("%s", gmail.GmailModifyScope))
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	if *saveToken {
		getClient(config)
		return
	}

	client := getClientWithTokenString(config, token)

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	user := "me"

	headersToShow := []string{"Subject", "Date"}

	srv.Users.Messages.List(user).MaxResults(int64(*pageSize)).Q("is:unread older_than:1d").Pages(context.TODO(), func(m *gmail.ListMessagesResponse) error {

		fmt.Print(".")

		ids := []string{}

		for _, l := range m.Messages {
			ids = append(ids, l.Id)
		}

		if *showHeaders {

			for _, i := range ids {
				msg, _ := srv.Users.Messages.Get("me", i).Format("metadata").Do()
				for _, h := range msg.Payload.Headers {
					if contains(headersToShow, h.Name) {
						fmt.Printf("%s ", h.Value)
					}
				}
				fmt.Print("\n")
			}
		}

		if !*dryRun {
			if len(ids) > 0 {
				modification := &gmail.BatchModifyMessagesRequest{
					Ids:            ids,
					RemoveLabelIds: []string{"INBOX", "UNREAD"},
				}

				err = srv.Users.Messages.BatchModify("me", modification).Do()

				if err != nil {
					log.Fatalf("Unable to archive messages: %v", err)
				}
			}
		}
		return nil
	})

	fmt.Printf("\n")

}
