package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dextryz/todo"
)

type handler struct {
	cfg *todo.Config
}

func (s *handler) done(w http.ResponseWriter, r *http.Request) {

	id := strings.TrimPrefix(r.URL.Path, "/done/")

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodPost:

		err := todo.Done(context.Background(), s.cfg, "food", id)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Load list from set of relays
		ctx := context.Background()
		var tl todo.TodoList
		err = tl.Load(ctx, s.cfg, "food")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmpl.ExecuteTemplate(w, "index.html", tl)
		if err != nil {
			fmt.Println("error executing template:", err)
		}

	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (s *handler) remove(w http.ResponseWriter, r *http.Request) {

	id := strings.TrimPrefix(r.URL.Path, "/item/")

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodDelete:

		err := todo.Delete(context.Background(), s.cfg, "food", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Load list from set of relays
		ctx := context.Background()
		var tl todo.TodoList
		err = tl.Load(ctx, s.cfg, "food")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmpl.ExecuteTemplate(w, "index.html", tl)
		if err != nil {
			fmt.Println("error executing template:", err)
		}

	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (s *handler) list(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load list from set of relays
	ctx := context.Background()
	var tl todo.TodoList
	err = tl.Load(ctx, s.cfg, "food")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "index.html", tl)
	if err != nil {
		fmt.Println("error executing template:", err)
	}
}

func main() {

	cfgEnv, ok := os.LookupEnv("NOSTR_TODO")
	if !ok {
		log.Fatalln("env var NOSTR_TODO not set")
	}

	data, err := os.ReadFile(cfgEnv)
	if err != nil {
		log.Fatal(err)
	}

	var cfg todo.Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	h := handler{
		&cfg,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.list)
	mux.HandleFunc("/done/", h.done)
	mux.HandleFunc("/item/", h.remove)

	fs := http.FileServer(http.Dir("."))
	mux.Handle("/style.css", fs)

	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = s.ListenAndServe()
	if err != nil {
		log.Fatal()
	}
}
