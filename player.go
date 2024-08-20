package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/f5aaff/spotify-wrappinator/requests"
	"github.com/go-chi/chi/v5"
	"io"
	"log"
	"net/http"
)

var allowedPlayerFunctions = []string{"pause", "play", "next", "previous"}

func ToggleShuffle(w http.ResponseWriter, r *http.Request) {
	currentStateReq := requests.New(requests.WithBaseURL("https://api.spotify.com/v1"), requests.WithRequestURL("/me/player"))
	requests.GetRequest(a, currentStateReq)

	res := map[string]interface{}{}
	err := json.Unmarshal(currentStateReq.Response, &res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shuffleState, ok := res["shuffle_state"].(bool)
	if !ok {
		http.Error(w, "error retrieving shuffle state, check for an active device.", http.StatusInternalServerError)
		return
	}

	toggleShuffleReq := requests.New(requests.WithBaseURL("https://api.spotify.com/v1"), requests.WithRequestURL("/me/player/shuffle"))
	requests.PutRequest(a, toggleShuffleReq, requests.Fields("state", fmt.Sprint(shuffleState)))
}

// gets currently playing, responds with the json from spotify.
// gets current device, retrieves currently playing, responds with an error
// if one occurs
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

// accepts a json object as shown in the spotify api.
// https://developer.spotify.com/documentation/web-api/reference/start-a-users-playbacK
// responds with StatusInternalServerError on error.
func PlayCustom(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	pcRequest := requests.New(requests.WithBaseURL("https://api.spotify.com/v1/"), requests.WithRequestURL("me/player/play"))

	PutBodyRequest(a, pcRequest, body)
	if pcRequest.Response == nil || len(pcRequest.Response) < 1 {
		http.Error(w, "error playing item\t"+http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(pcRequest.Response)
	if err != nil {
		log.Printf("error writing response: %s", err.Error())
	}
}

// performs basic play/pause/next/previous functions. expects a value from context
func PlayerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	playerFunc, err := ctx.Value("playerFunc").(string)
	if !err {
		http.Error(w, http.StatusText(404), 404)
		return
	}
	if playerFunc == "next" || playerFunc == "previous" {
		playerRequest := requests.New(requests.WithRequestURL("me/player/"+playerFunc), requests.WithBaseURL("https://api.spotify.com/v1/"))
		requests.ParamFormRequest(a, playerRequest)
		return
	} else {
		err2 := d.PlayPause(a, playerFunc)
		if err2 != nil {
			http.Error(w, fmt.Sprintf("err:%s\tstatus:%s", err2.Error(), http.StatusText(http.StatusInternalServerError)), http.StatusInternalServerError)
			return
		}
	}
}

// used to pull playerFunc context from request.
func PlayerCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerFunc := chi.URLParam(r, "playerFunc")
		for _, x := range allowedPlayerFunctions {
			if playerFunc == x {
				var key string = "playerFunc"
				ctx := context.WithValue(r.Context(), key, playerFunc)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, http.StatusText(404), 404)
	})
}

// unused playcustom variant using context handling and url parameters.
func PlayCustomCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var contextUriKey, positionMsKey, positionKey contextKey = "ContextUri", "position_ms", "position"

		contextUri := chi.URLParam(r, "ContextUri")
		position := chi.URLParam(r, "position")
		position_ms := chi.URLParam(r, "position_ms")
		ctx := context.WithValue(r.Context(), contextUriKey, contextUri)
		ctx = context.WithValue(ctx, positionKey, position)
		ctx = context.WithValue(ctx, positionMsKey, position_ms)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type Player struct {
	Token string
}
