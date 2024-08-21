package main

import (
    "bytes"
    "context"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/f5aaff/spotify-wrappinator/agent"
    "github.com/f5aaff/spotify-wrappinator/auth"
    "github.com/f5aaff/spotify-wrappinator/device"
    "github.com/f5aaff/spotify-wrappinator/requests"
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/go-chi/render"
    "github.com/joho/godotenv"
    "golang.org/x/oauth2"
)

var (
    baseURL        string = "https://api.spotify.com/v1/"
    state          string = "abc123"
    clientId       string = os.Getenv("CLIENT_ID")
    clientSecret   string = os.Getenv("CLIENT_SECRET")
    serverAddress  string = os.Getenv("SERVER_ADDRESS")
    serverPort     string = os.Getenv("SERVER_PORT")
    tokenStorePath string = os.Getenv("TOKEN_STORE_PATH")
    redirectURL string = "http://localhost:3000/callback"
    conf               = auth.New(auth.WithRedirectURL(redirectURL),
        auth.WithClientID(clientId),
        auth.WithClientSecret(clientSecret),
        auth.WithScopes(auth.ScopeUserReadPrivate,
            auth.ScopeUserReadPlaybackState,
            auth.ScopeUserModifyPlaybackState,
            auth.ScopeStreaming,
            auth.ScopeUserLibraryRead,
            auth.ScopeUserLibraryModify,
            auth.ScopeUserReadRecentlyPlayed,
            auth.ScopeUserTopRead,
            "user-read-private",
            "user-read-email",
            "user-follow-read",
            "playlist-read-private",
            "playlist-read-collaborative",
            "playlist-modify-private",
            "playlist-modify-public",

        ))
    validToken oauth2.Token
    a          = agent.New(conf, agent.WithToken(validToken))
    d          = device.New()
)

type contextKey string

func main() {

    envloaderr := godotenv.Load()
    if envloaderr != nil {
        fmt.Println("error loading .env, ensure .env is in the correct directory.")
        return
    }
    err := os.MkdirAll(tokenStorePath, os.ModePerm)
    if err != nil {
        fmt.Println("error generating token store directory. ensure .env is correct and using an absolute path")
        return
    }

    // if a token can't be read from file, prompt the user to log in

    // empty env fields, default values used
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
    // Basic CORS
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"http://*"},
        AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: false,
        MaxAge:           300, // Maximum value not ignored by any of major browsers
    }))

    // Set a timeout value on the request context (ctx), that will signal
    // through ctx.Done() that the request has timed out and further
    // processing should be stopped.
    r.Use(middleware.Timeout(60 * time.Second))

    // if the token cannot be read from file, prompt the user to login.
    if !agent.ReadTokenFromFile(a) {

        r.HandleFunc("/callback", AuthoriseSession)
        r.Get("/", func(w http.ResponseWriter, r *http.Request) {
            log.Println("request: ", r.URL.String())
        })
        url := auth.GetAuthURL(conf, state)
        fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)
        err := http.ListenAndServe(":3000", r)
        if err != nil {
            log.Fatal(err)
        }
    }

    a.Token, err = auth.RefreshToken(conf, context.Background(), a.Token)
    if err != nil {
        log.Fatalf("Token Refresh error: %s", err.Error())
    }

    a.Client = auth.Client(conf, context.Background(), a.Token)
    p := Player{a.Token.AccessToken}
    UpdatePage(&p, "template.html", "index.html")

    // playlist end points
    // <ADDRESS>:<PORT>/playlists/<ENDPOINT>
    r.Route("/playlists", func(r chi.Router) {
        r.Get("/", GetPlaylists)
    })

    // device end points
    // <ADDRESS>:<PORT>/devices/<ENDPOINT>
    r.Route("/devices", func(r chi.Router) {
        r.Get("/", GetDevice)
        r.Route("/all", func(r chi.Router) {
            r.Get("/", GetDevices)
        })
        r.Route("/queue", func(r chi.Router) {
            r.Get("/", GetQueue)
            r.Post("/add/{uri}", AddToQueue)
        })
        r.Route("/transfer", func(r chi.Router) {
            r.Post("/", TransferPlayback)
        })
        r.Route("/player/{playerFunc}", func(r chi.Router) {
            r.Use(PlayerCtx)
            r.Get("/", PlayerRequest)
        })

    })

    // player end points
    // <ADDRESS>:<PORT>/player/<ENDPOINT>
    r.Route("/player", func(r chi.Router) {

        r.Get("/currently_playing", getCurrentlyPlaying)

        r.Get("/volup", incVol)
        r.Get("/voldown", decVol)
        r.Get("/shuffle", ToggleShuffle)

        r.Route("/controls/{playerFunc}", func(r chi.Router) {
            r.Use(PlayerCtx)
            r.Get("/", PlayerRequest)
        })
        r.Route("/recently-played", func(r chi.Router) {
            r.Use(render.SetContentType(render.ContentTypeJSON))
            r.Post("/", GetRecentlyPlayedFine)
            r.Get("/", GetRecentlyPlayed)
        })

        r.Route("/play", func(r chi.Router) {
            r.Use(render.SetContentType(render.ContentTypeJSON))
            r.Post("/", PlayCustom)
        })
    })

    // search end points
    // <ADDRESS>:<PORT>/search/<ENDPOINT>
    r.Route("/search", func(r chi.Router) {
        r.Use(render.SetContentType(render.ContentTypeJSON))
        r.Post("/", GetSearch)

    })

    // top end points
    // <ADDRESS>:<PORT>/top/<artists/tracks>
    r.Route("/top/", func(r chi.Router){
        r.Get("/*",GetTop)
    })

    // recommendations end points
    // <ADDRESS>:<PORT>/recommendations/<ENDPOINT>
    r.Route("/recommendations", func(r chi.Router) {
        r.Get("/get/*", GetRecommendationsURL)
    })

    // page for actual spotify player, runs in headless firefox client
    r.Route("/player/app", func(r chi.Router) {
        r.Get("/", func(w http.ResponseWriter, r *http.Request) {
            workDir, _ := os.Getwd()
            filesDir := http.Dir(workDir)
            rctx := chi.RouteContext(r.Context())
            pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
            fs := http.StripPrefix(pathPrefix, http.FileServer(filesDir))
            fs.ServeHTTP(w, r)
        })
    })
    go startServer(r)

    go func() {
        for {
            if isServerUp() {
                break
            }
            time.Sleep(time.Millisecond * 100)
        }
        cmd := exec.Command("./start.sh")
        err := cmd.Run()
        if err != nil {
            fmt.Printf("error starting firefox: %v\n", err)
        }
    }()
    select {}
}

// perform a put request with body, expects a ClientRequest, Agent,
// and json body in the form of a []byte.
// adds an error to the response body on failure.
func PutBodyRequest(a *agent.Agent, c *requests.ClientRequest, body []byte) {
    fullUrl := c.BaseURL + c.RequestURL

    req, err := http.NewRequest("PUT", fullUrl, bytes.NewBuffer(body))
    if err != nil {
        errorString := fmt.Sprintf("error:%s\nbody:%s", err.Error(), string(body))
        log.Println(errorString)
        c.Response = []byte(errorString)
        return
    }
    bearerval := "Bearer " + a.Token.AccessToken
    req.Header.Set("Authorization", bearerval)
    req.Header.Set("Content-Type", "application/json")
    res, err := a.Client.Do(req)
    if err != nil {
        body, _ := io.ReadAll(req.Body)
        c.Response = []byte(err.Error() + "\n" + req.URL.String() + string(body))
        return
    }

    c.Response, err = io.ReadAll(res.Body)
    if err != nil {
        c.Response = []byte(err.Error())
    }
}

func PostParamRequest(a *agent.Agent, c *requests.ClientRequest, params map[string]string) {

    fullUrl := c.BaseURL + c.RequestURL
    urlVals := url.Values{}
    for k, v := range params {
        urlVals.Add(k, v)
    }
    fullUrl = fullUrl + urlVals.Encode()
    log.Printf("full url: %s", fullUrl)
    req, err := http.NewRequest("POST", fullUrl, nil)
    if err != nil {
        errorString := fmt.Sprintf("error:%s", err.Error())
        log.Println(errorString)
        c.Response = []byte(errorString)
        return
    }
    bearerval := "Bearer " + a.Token.AccessToken
    req.Header.Set("Authorization", bearerval)
    req.Header.Set("Content-Type", "application/json")
    res, err := a.Client.Do(req)
    if err != nil {
        body, _ := io.ReadAll(req.Body)
        c.Response = []byte(err.Error() + "\n" + req.URL.String() + string(body))
        return
    }

    c.Response, err = io.ReadAll(res.Body)
    if err != nil {
        c.Response = []byte(err.Error())
    }
}

// updates the underlying html template used to drive the go_tify audio device.
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
    if err != nil {
        fmt.Printf("error updating page: %s", err.Error())
    }
}

// starts the chi server.
func startServer(r http.Handler) {
    errChan := make(chan error)
    go func() {
        err := http.ListenAndServe(fmt.Sprintf(":%s", serverPort), r)
        errChan <- err
    }()
    select {
    case err := <-errChan:
        log.Fatal(err.Error())
    }
}

// simple bool to check if the server is running.
func isServerUp() bool {
    resp, err := http.Get(fmt.Sprintf("http://%s:%s/devices/", serverAddress, serverPort))
    fmt.Printf("http://%s:%s/devices/\n", serverAddress, serverPort)
    if err != nil {
        return false
    }
    defer func(Body io.ReadCloser) {
        err := Body.Close()
        if err != nil {
            fmt.Println(err.Error())
        }
    }(resp.Body)
    return true
}

// uses the underlying wrappinator library to authorise a session,
// stores a token to file, reads a token from file,
// generates/sets the token for the agent.
func AuthoriseSession(w http.ResponseWriter, r *http.Request) {
    tok, err := auth.GetToken(a.Conf, r.Context(), state, r)
    if err != nil {
        http.Error(w, "token could not be retrieved", http.StatusForbidden)
        log.Fatal(err)
    }
    a.Token = tok

    log.Println("AuthoriseSession: Storing Token to File...")
    if err = agent.StoreTokenToFile(a.Token); err != nil {
        log.Println("Could Not Save token:" + err.Error())
    }

    if st := r.FormValue("state"); st != state {
        http.NotFound(w, r)
        log.Fatalf("state mismatch: %s != %s\n", st, state)
    }

    _, err = fmt.Fprintf(w, "login successful\n%+v", a.Token)
    if err != nil {
        log.Printf("AuthoriseSession: " + err.Error())
        return
    }
    go func() {
        for range time.Tick(time.Minute * 30) {
            a.Token, _ = auth.RefreshToken(a.Conf, r.Context(), a.Token)
        }
    }()
}
