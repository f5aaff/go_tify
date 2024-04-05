package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/f5aaff/spotify-wrappinator/agent"
	"github.com/f5aaff/spotify-wrappinator/auth"
	"github.com/f5aaff/spotify-wrappinator/device"
	"github.com/f5aaff/spotify-wrappinator/recommendations"
	"github.com/f5aaff/spotify-wrappinator/requests"
	"github.com/f5aaff/spotify-wrappinator/search"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	redirectURL = "http://localhost:3000/redirect"
)

var (
	envloaderr                    = godotenv.Load()
	state                  string = "abc123"
	clientId               string = os.Getenv("CLIENT_ID")
	clientSecret           string = os.Getenv("CLIENT_SECRET")
	conf                          = auth.New(auth.WithRedirectURL(redirectURL), auth.WithClientID(clientId), auth.WithClientSecret(clientSecret), auth.WithScopes(auth.ScopeUserReadPrivate, auth.ScopeUserReadPlaybackState, auth.ScopeUserModifyPlaybackState, auth.ScopeStreaming))
	validToken             oauth2.Token
	a                      = agent.New(conf, agent.WithToken(validToken))
	d                      = device.New()
	allowedPlayerFunctions = []string{"pause", "play", "next", "previous"}
)

func main() {

	if envloaderr != nil {
		return
	}
	/*
		if a token can't be read from file, prompt the user to log in
	*/
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	if agent.ReadTokenFromFile(a) == false {

		r.HandleFunc("/callback", AuthoriseSession)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			log.Println("request: ", r.URL.String())
		})
		url := auth.GetAuthURL(conf, state)
		fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)
	}
	r.Route("/playlists", func(r chi.Router) {
		r.Get("/", GetPlaylists)
	})
	r.Route("/devices", func(r chi.Router) {
		r.Get("/", GetDevice)
		r.Route("/queue", func(r chi.Router) {
			r.Get("/", GetQueue)
		})
		r.Get("/currently_playing", getCurrentlyPlaying)
		r.Route("/player/{playerFunc}", func(r chi.Router) {
			r.Use(PlayerCtx)
			r.Get("/", PlayerRequest)
			r.Route("/playCustom/{ContextUri}", func(r chi.Router) {
				r.Use(PlayCustomCtx)
				r.Get("/", PlayCustom)
			})
		})
	})
	r.Route("/search", func(r chi.Router) {

	})
	r.Route("/recommendations", func(r chi.Router) {

	})

	http.ListenAndServe(":3000", r)

	noreq := 0
	if noreq != 0 {
		err := errors.New("")
		a.Token, err = auth.RefreshToken(conf, context.Background(), a.Token)
		if err != nil {
			log.Fatalf("Token Refresh error: %s", err.Error())
		}
		a.Client = auth.Client(conf, context.Background(), a.Token)

		err = d.GetCurrentDevice(a)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%+v\n", d)

		//	contextUri := "spotify:show:0qw2sRabL5MOuWg6pgyIiY"
		//	err = d.PlayCustom(a, &contextUri, nil, nil)
		//	if err != nil {
		//		fmt.Println(err)
		//	}
		paramRequest := requests.New(requests.WithRequestURL("browse/new-releases"), requests.WithBaseURL("https://api.spotify.com/v1/"))
		playerRequest := requests.New(requests.WithRequestURL("me/player/devices"), requests.WithBaseURL("https://api.spotify.com/v1/"))
		requests.ParamRequest(a, paramRequest)
		requests.ParamRequest(a, playerRequest)
		searchRequest := requests.New(requests.WithRequestURL("search"), requests.WithBaseURL("https://api.spotify.com/v1/"))
		requests.ParamRequest(a, searchRequest, search.Query("thy art is murder", nil), search.Types([]string{"artist"}), search.Market("ES"), requests.Limit(1))
		seedVals := map[string][]string{"seed_genres": {"deathmetal"}, "seed_artists": {"3et9upNERQI5IYt5jEDTxM"}}
		percentVals := map[string]int{"max_loudness": 100, "min_loudness": 90}
		intVals := map[string]int{"min_tempo": 80, "max_tempo": 200}
		recRequest := requests.New(requests.WithRequestURL("recommendations"), requests.WithBaseURL("https://api.spotify.com/v1/"))
		_ = requests.ParamRequest(a, recRequest, recommendations.ListParams(seedVals), requests.Limit(1), recommendations.PercentParams(percentVals), recommendations.IntParams(intVals))
		fmt.Printf("%s", string(recRequest.Response))
	}
}
func GetPlaylists(w http.ResponseWriter, r *http.Request) {
	getPlaylistsRequest := requests.New(requests.WithRequestURL("me/playlists"), requests.WithBaseURL("https://api.spotify.com/v1/"))
	requests.GetRequest(a, getPlaylistsRequest)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(getPlaylistsRequest.Response)
	if err != nil {
		return
	}
}
func GetDevice(w http.ResponseWriter, r *http.Request) {
	err := d.GetCurrentDevice(a)
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("error obtaining current device"))
		if err != nil {
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(res)
	if err != nil {
		log.Println("error writing device to response")
	}
}
func GetQueue(w http.ResponseWriter, r *http.Request) {
	if d.Name == "" {
		err := d.GetCurrentDevice(a)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte("error obtaining current device"))
			if err != nil {
				return
			}
			return
		}
	}
	queue, err := d.GetQueue(a)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting device queue"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(queue))
	if err != nil {
		log.Println("error writing queue to response")
		return
	}
}
func getCurrentlyPlaying(w http.ResponseWriter, r *http.Request) {
	if d.Name == "" {
		err := d.GetCurrentDevice(a)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte("error obtaining current device"))
			if err != nil {
				return
			}
			return
		}
	}
	current, err := d.GetCurrentlyPlaying(a)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting currently playing: " + err.Error()))
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(current))
	if err != nil {
		log.Println("error writing currently playing to response: " + err.Error())
		return
	}
}

func GetRecommendations(w http.ResponseWriter, r *http.Request) {}
func GetSearch(w http.ResponseWriter, r *http.Request)          {}

func PlayerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	playerFunc, err := ctx.Value("playerFunc").(string)
	if !err {
		http.Error(w, http.StatusText(404), 404)
	}
	err2 := d.PlayPause(a, playerFunc)
	if err2 != nil {
		http.Error(w, err2.Error()+http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
func PlayCustom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contextUri, err := ctx.Value("ContextUri").(string)
	if !err {
		http.Error(w, http.StatusText(404), 404)
	}

	err2 := d.PlayCustom(a, &contextUri, nil, nil)
	if err2 != nil {
		http.Error(w, err2.Error()+http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func PlayerCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerFunc := chi.URLParam(r, "playerFunc")
		for _, x := range allowedPlayerFunctions {
			if playerFunc == x {
				ctx := context.WithValue(r.Context(), "playerFunc", playerFunc)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, http.StatusText(404), 404)
		return
	})
}
func PlayCustomCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextUri := chi.URLParam(r, "ContextUri")
		ctx := context.WithValue(r.Context(), "ContextUri", contextUri)
		next.ServeHTTP(w, r.WithContext(ctx))
		return
	})
}
func AuthoriseSession(w http.ResponseWriter, r *http.Request) {
	err := errors.New("")
	a.Token, err = auth.GetToken(a.Conf, r.Context(), state, r)
	if err != nil {
		http.Error(w, "token could not be retrieved", http.StatusForbidden)
		log.Fatal(err)
	}
	log.Println("AuthoriseSession: Storing Token to File...")
	if err = agent.StoreTokenToFile(a.Token); err != nil {
		log.Println("Could Not Save token:" + err.Error())
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("state mismatch: %s != %s\n", st, state)
	}

	_, err = fmt.Fprintf(w, "login successful\n%s", a.Token)
	if err != nil {
		log.Printf("AuthoriseSession: " + err.Error())
		return
	}
}
