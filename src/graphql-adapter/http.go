package graphqladapter

import (
	"fmt"

	"github.com/gorilla/mux"
	"github.com/graphql-go/handler"
)

func RegisterGraphql(r *mux.Router) error {
	schema, err := NewSchema()

	if err != nil {
		fmt.Println("Unable to create graphql schema")
		return err
	}

	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: false,
	})

	router := r.PathPrefix("/graphql").Subrouter()
	router.Handle("/", h)

	return nil
}
