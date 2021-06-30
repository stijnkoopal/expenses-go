package bunqconnector

import (
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const clientID = ""
const secret = ""
const redirectURL = "https://koopal.xyz/api/bunq/authorize"

func connectHandler(res http.ResponseWriter, req *http.Request) {
	state := uuid.New().String()

	url := fmt.Sprintf("https://oauth.bunq.com/auth?response_type=code&client_id=%s&redirect_uri=$%s&state=%s", clientID, redirectURL, state)

	res.Header().Add("Access-Control-Expose-Headers", "Location")
	res.Header().Add("Location", url)
	res.WriteHeader(204)
}

func authorizeHandler(res http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	err := req.URL.Query().Get("error")
	state := req.URL.Query().Get("state")

	if state == "" {
		res.WriteHeader(400)
		return
	}

	if err == "" && code == "" {
		res.WriteHeader(400)
		return
	}

	if err != "" {
		io.WriteString(res, err)
		res.WriteHeader(400)
		return
	}

	// TODO
	// fetch incomplete auth by `state`
	// POST to uri"https://api.oauth.bunq.com/v1/token?grant_type=authorization_code&code=$code&redirect_uri=$redirectUrl&client_id=$clientId&client_secret=$clientSecret"
	// Initialize bunq api context with body.access_token
	// fetch bunq user id
	// save the context json, overwrite if bunq user id has one
	// publish connection established
}

// RegisterOAuthController will register a http controller that will allow for the bunq OAuth flow
func RegisterOAuthController(r *mux.Router) error {
	controller := r.PathPrefix("/bunq").Subrouter()
	controller.Methods("POST").Path("connect").HandlerFunc(connectHandler)
	controller.Methods("GET").Path("authorize").HandlerFunc(authorizeHandler)
	return nil
}
