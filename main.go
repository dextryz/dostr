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
    // Relays config and secret key
	cfg *todo.Config

    // Use a pointer since we want to update the state (Done/Undone)
    items map[string]*todo.Todo
}

func (s *handler) checked(w http.ResponseWriter, r *http.Request, id string) {

    // Find item in cache
	item, ok := s.items[id]
    if !ok {
        err := fmt.Errorf("item %s not found", id)
        log.Println(err)
    }

    // Update the relays
	if item.Done {
        go func() {
            err := todo.Undone(context.Background(), s.cfg, "food", id)
            if err != nil {
                log.Println(err)
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
        }()
		item.Done = false
	} else {
        go func() {
            log.Println("DONE - Start")
            err := todo.Done(context.Background(), s.cfg, "food", id)
            if err != nil {
                log.Println(err)
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            log.Println("DONE - End")
        }()
		item.Done = true
	}

	tmpl, err := template.ParseFiles("item.html")
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, item)

    log.Println("Exit")
}

func (s *handler) remove(w http.ResponseWriter, r *http.Request, id string) {

    // Delete the item from relays
	err := todo.Delete(context.Background(), s.cfg, "food", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

    // Delete the item from local cache
    delete(s.items, id)

	tmpl, err := template.ParseFiles("index.html", "item.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

    var tl todo.TodoList
    for _, v := range s.items {
        tl = append(tl, *v)
    }

	err = tmpl.ExecuteTemplate(w, "index.html", tl)
	if err != nil {
		fmt.Println("error executing template:", err)
	}
}

func (s *handler) itemHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/item/")
	switch r.Method {
	case http.MethodPost:
		s.checked(w, r, id)
	case http.MethodDelete:
		s.remove(w, r, id)
	default:
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
	}
}

func (s *handler) list(w http.ResponseWriter, r *http.Request) {

	// Load list from set of relays
	var tl todo.TodoList
    err := tl.Load(context.Background(), s.cfg, "food")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

    // Cache loaded items
    for _, v := range tl {
        itemCopy := v
        s.items[v.Id] = &itemCopy
    }

	tmpl, err := template.ParseFiles("index.html", "item.html")
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
        cfg: &cfg,
        items: make(map[string]*todo.Todo),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.list)
	mux.HandleFunc("/item/", h.itemHandler)

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
