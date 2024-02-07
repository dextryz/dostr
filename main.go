package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dextryz/todo"
)

type handler struct {
	wg *sync.WaitGroup

	mu sync.Mutex

	// Relays config and secret key
	cfg *todo.Config

	// Use a pointer since we want to update the state (Done/Undone)
	items map[string]*todo.Todo

	errChan chan error
}

func (s *handler) checked(w http.ResponseWriter, r *http.Request, id string) {

	// Find item in cache
	item, ok := s.items[id]
	if !ok {
		err := fmt.Errorf("item %s not found", id)
		log.Println(err)
	}

	s.wg.Add(1)
	// Update the relays
	if item.Done {
		go func() {
			defer s.wg.Done()
			s.mu.Lock()
			err := todo.Undone(context.Background(), s.cfg, "food", id)
			if err != nil {
                s.errChan <- err
				return
			}
			s.mu.Unlock()
		}()
		item.Done = false
	} else {
		go func() {
			defer s.wg.Done()
			s.mu.Lock()
			err := todo.Done(context.Background(), s.cfg, "food", id)
			if err != nil {
                s.errChan <- err
				return
			}
			s.mu.Unlock()
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
}

func (s *handler) remove(w http.ResponseWriter, r *http.Request, id string) {

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Delete the item from relays
		s.mu.Lock()
		if err := todo.Delete(context.Background(), s.cfg, "food", id); err != nil {
			log.Printf("Deletion error: %v", err)
			s.errChan <- err
			return
		}
		log.Printf("deleting %s", id)
		s.mu.Unlock()
	}()

	// Safely delete the item from local cache
	delete(s.items, id)

	var tl todo.TodoList
	for _, v := range s.items {
		tl = append(tl, *v)
	}

	tmpl, err := template.ParseFiles("index.html", "item.html")
	if err != nil {
		s.errChan <- err
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

	var wg sync.WaitGroup

	h := handler{
		wg:      &wg,
		mu:      sync.Mutex{},
		cfg:     &cfg,
		items:   make(map[string]*todo.Todo),
		errChan: make(chan error),
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

	// Create a channel to listen for termination signals (e.g., Ctrl-C)
	stop := make(chan os.Signal, 1)
	// Catch termination signals
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Run the server in a goroutine so that it doesn't block
	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for the goroutines to finish
	h.wg.Wait()
	close(h.errChan) // Close channel to signal completion

	// Check for errors from goroutines
	for err := range h.errChan {
		if err != nil {
			log.Fatalln(err)
		}
	}

	// Wait for a termination signal
	<-stop

	// Attempt a graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}

	log.Println("Server shutdown")
}
