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
	"github.com/go-chi/render"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const ()

var (
	baseURL                string = "https://api.spotify.com/v1/"
	state                  string = "abc123"
	clientId               string = os.Getenv("CLIENT_ID")
	clientSecret           string = os.Getenv("CLIENT_SECRET")
	serverAddress          string = os.Getenv("SERVER_ADDRESS")
	serverPort             string = os.Getenv("SERVER_PORT")
	redirectURL            string = "http://localhost:3000/callback"
	conf                          = auth.New(auth.WithRedirectURL(redirectURL), auth.WithClientID(clientId), auth.WithClientSecret(clientSecret), auth.WithScopes(auth.ScopeUserReadPrivate, auth.ScopeUserReadPlaybackState, auth.ScopeUserModifyPlaybackState, auth.ScopeStreaming, auth.ScopeUserLibraryRead, auth.ScopeUserLibraryModify))
	validToken             oauth2.Token
	a                      = agent.New(conf, agent.WithToken(validToken))
	d                      = device.New()
	allowedPlayerFunctions = []string{"pause", "play", "next", "previous"}
)

func main() {

	envloaderr := godotenv.Load()
	if envloaderr != nil {
		return
	}
	/*
		if a token can't be read from file, prompt the user to log in
	*/
	if serverAddress == "" {
		serverAddress = "localhost"
	}
	if serverPort == "" {
		serverPort = "3000"
	}
	redirectURL = fmt.Sprintf("http://%s:%s/callback", serverAddress, serverPort)
	fmt.Printf(redirectURL + "\n")
	conf = auth.New(auth.WithRedirectURL(redirectURL), auth.WithClientID(clientId), auth.WithClientSecret(clientSecret), auth.WithScopes(auth.ScopeUserReadPrivate, auth.ScopeUserReadPlaybackState, auth.ScopeUserModifyPlaybackState, auth.ScopeStreaming))
	a = agent.New(conf, agent.WithToken(validToken))

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
		err := http.ListenAndServe(":3000", r) //fmt.Sprintf("%s:%s,",serverAddress, serverPort), r)
		if err != nil {
			log.Fatal(err)
		}
	}
	err := errors.New("")
	a.Token, err = auth.RefreshToken(conf, context.Background(), a.Token)
	if err != nil {
		log.Fatalf("Token Refresh error: %s", err.Error())
	}
	a.Client = auth.Client(conf, context.Background(), a.Token)
	p := Player{a.Token.AccessToken}
	UpdatePage(&p, "template.html", "index.html")

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
			r.Route("/playCustom/{ContextUri}/${position}/${position_ms}", func(r chi.Router) {
				r.Use(PlayCustomCtx)
				r.Get("/", PlayCustom)
			})
		})
	})
	r.Route("/search", func(r chi.Router) {
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Post("/", GetSearch)

	})
	r.Route("/recommendations", func(r chi.Router) {
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Post("/", GetRecommendations)

	})
	r.Route("/player", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			workDir, _ := os.Getwd()
			filesDir := http.Dir(workDir)
			rctx := chi.RouteContext(r.Context())
			pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
			fs := http.StripPrefix(pathPrefix, http.FileServer(filesDir))
			fs.ServeHTTP(w, r)
		})
	})

	http.ListenAndServe(fmt.Sprintf(":%s", serverPort), r)
	fmt.Println(serverPort)
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

type RecommendationRequest struct {
	SeedValues    map[string][]string `json:"seed_values"`
	PercentValues map[string]int      `json:"percent_values"`
	IntValues     map[string]int      `json:"int_values"`
	Limit         int                 `json:"limit"`
}

func (a *RecommendationRequest) Bind(r *http.Request) error {
	if a.SeedValues == nil {
		return errors.New("missing seed values")
	}
	return nil
}
func GetRecommendations(w http.ResponseWriter, r *http.Request) {
	data := &RecommendationRequest{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	recRequest := requests.New(requests.WithBaseURL(baseURL), requests.WithRequestURL("recommendations"))
	_ = requests.ParamRequest(a, recRequest, recommendations.ListParams(data.SeedValues), requests.Limit(data.Limit), recommendations.PercentParams(data.PercentValues), recommendations.IntParams(data.IntValues))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(recRequest.Response)
	if err != nil {
		log.Println("error writing currently playing to response: " + err.Error())
		return
	}
}

type SearchRequest struct {
	Query  string            `json:"query"`
	Tags   map[string]string `json:"tags"`
	Types  []string          `json:"types"`
	Market string            `json:"market"`
	Limit  int               `json:"limit"`
}

func GetSearch(w http.ResponseWriter, r *http.Request) {
	data := &SearchRequest{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	recRequest := requests.New(requests.WithBaseURL(baseURL), requests.WithRequestURL("search"))
	_ = requests.ParamRequest(a, recRequest, search.Query(data.Query, data.Tags), requests.Limit(data.Limit), search.Types(data.Types), search.Market(data.Market))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(recRequest.Response)
	if err != nil {
		log.Println("error writing currently playing to response: " + err.Error())
		return
	}

}

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
	position, err := ctx.Value("position").(int)
	positionptr := &position
	position_ms, err := ctx.Value("position_ms").(int)
	position_msptr := &position_ms
	if position == -1 {
		positionptr = nil
	}
	if position_ms == -1 {
		position_msptr = nil
	}

	if !err {
		http.Error(w, http.StatusText(404), 404)
	}

	err2 := d.PlayCustom(a, &contextUri, positionptr, position_msptr)
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
		position := chi.URLParam(r, "position")
		position_ms := chi.URLParam(r, "position_ms")
		ctx := context.WithValue(r.Context(), "ContextUri", contextUri)
		ctx = context.WithValue(ctx, "position", position)
		ctx = context.WithValue(ctx, "position_ms", position_ms)
		next.ServeHTTP(w, r.WithContext(ctx))
		return
	})
}

type Player struct {
	Token string
}

func UpdatePage(p *Player, tmplFile string, destFile string) {

	tmpl, err := template.New(tmplFile).ParseFiles(tmplFile)
	if err != nil {
		log.Fatal(err)
	}
	var f *os.File
	f, err = os.Create(destFile)
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(f, p)

}
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {

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
