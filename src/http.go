package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func registerHealthchecks(r *mux.Router) error {
	healthchecks := r.PathPrefix("/health").Subrouter()
	healthchecks.HandleFunc("/ready", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, "OK")
	})
	return nil
}

func ServeHttp(ctx context.Context, muxes []func(r *mux.Router) error) (err error) {
	r := mux.NewRouter()

	for _, m := range muxes {
		if err = m(r); err != nil {
			return err
		}
	}

	srv := &http.Server{
		Addr:    ":9000",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Unable to listen to port 9000, because %s", err)
		}
	}()

	log.Printf("server started")

	<-ctx.Done()

	log.Printf("server stopped")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err = srv.Shutdown(ctxShutDown); err != nil {
		log.Fatalf("server Shutdown Failed:%+s", err)
	}

	log.Printf("server exited properly")

	if err == http.ErrServerClosed {
		err = nil
	}

	return err
}
