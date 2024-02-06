package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/nbd-wtf/go-nostr"
)

var ErrNotFound = errors.New("todo list not found")

type config struct {
	Nsec   string   `json:"nsec"`
	Relays []string `json:"relays"`
}

type todo struct {
	Id        string `json:"id"`
	Content   string `json:"content"`
	Done      bool   `json:"done"`
	CreatedAt int64  `json:"created_at"`
}

type todoList []todo

type handler struct {
	config
}

func tagName(name string) string {
	if name == "" {
		return "nostr-todo"
	}
	return "nostr-todo-" + name
}

func (s *handler) home(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	// Use configuration to pull todo list data from relays
	pool := nostr.NewSimplePool(ctx)
	filter := nostr.Filter{
		Kinds: []int{nostr.KindApplicationSpecificData},
		Tags: nostr.TagMap{
			"d": []string{tagName("food")},
		},
	}

	e := pool.QuerySingle(ctx, s.Relays, filter)
	if e == nil {
		http.Error(w, ErrNotFound.Error(), http.StatusInternalServerError)
		return
	}

	var tl todoList
	err = json.Unmarshal([]byte(e.Content), &tl)
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

	var cfg config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	h := handler{
		cfg,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.home)

	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = s.ListenAndServe()
	if err != nil {
		log.Fatal()
	}
}
