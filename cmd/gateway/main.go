package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"wagr/internal/fantasy/sleeper"
)

type Server struct {
	sleeperClient *sleeper.Client
}

func main() {
	srv := &Server{
		sleeperClient: sleeper.NewClient(),
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Routes
	r.Route("/api/sleeper", func(r chi.Router) {
		r.Get("/user/{username}", srv.handleGetUser)
		r.Get("/user/{userId}/leagues", srv.handleGetUserLeagues)
		r.Get("/league/{leagueId}", srv.handleGetLeague)
		r.Get("/league/{leagueId}/teams", srv.handleGetLeagueTeams)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Println("Starting API Gateway on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := s.sleeperClient.GetUser(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, user)
}

func (s *Server) handleGetUserLeagues(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	// Default to NFL and current season
	sport := r.URL.Query().Get("sport")
	if sport == "" {
		sport = "nfl"
	}

	season := r.URL.Query().Get("season")
	if season == "" {
		season = strconv.Itoa(time.Now().Year())
	}

	leagues, err := s.sleeperClient.GetUserLeagues(userID, sport, season)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, leagues)
}

func (s *Server) handleGetLeague(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueId")

	league, err := s.sleeperClient.GetLeague(leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, league)
}

func (s *Server) handleGetLeagueTeams(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueId")

	teams, err := s.sleeperClient.GetLeagueTeams(leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, teams)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
