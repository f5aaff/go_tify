package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/f5aaff/spotify-wrappinator/recommendations"
	"github.com/f5aaff/spotify-wrappinator/requests"
	"github.com/f5aaff/spotify-wrappinator/search"
)
var hostaddr = "http://"+serverAddress+":"+serverPort
// gets user playlists, responds with the json from spotify.
// retrieves user playlists, responds with an error
// if one occurs
func GetPlaylists(w http.ResponseWriter, r *http.Request) {
	getPlaylistsRequest := requests.New(requests.WithRequestURL("me/playlists"), requests.WithBaseURL("https://api.spotify.com/v1/"))
	requests.GetRequest(a, getPlaylistsRequest)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(getPlaylistsRequest.Response)
	if err != nil {
		return
	}
}

func GetRecentlyPlayed(w http.ResponseWriter, r *http.Request) {

	recentRequest := requests.New(requests.WithBaseURL("https://api.spotify.com/v1/"),
		requests.WithRequestURL("me/player/recently-played"))
	requests.GetRequest(a, recentRequest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(recentRequest.Response)
}

// retrieves recently played songs, expects a json with 3 int fields, Limit
// before and after. all 3 can be omitted, and the default vals will be used.
func GetRecentlyPlayedFine(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := struct {
		Limit  int `json: "limit, omitempty"`
		Before int `json: "before", omitempty`
		After  int `json: "after", omitempty`
	}{50, 0, 0}

	var time string
	var timestamp int

	err = json.Unmarshal(reqBody, &params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if params.Before > params.After {
		time = "before"
		timestamp = params.Before
	} else {
		time = "after"
		timestamp = params.After
	}

	recentRequest := requests.New(requests.WithBaseURL("https://api.spotify.com/v1/"),
		requests.WithRequestURL("me/player/recently-played"))
	requests.ParamRequest(a,
		recentRequest, requests.Fields("limit",
			fmt.Sprint(params.Limit)), requests.Fields(time, fmt.Sprint(timestamp)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(recentRequest.Response)
}

// gets current queue, responds with the json from spotify.
// gets current device, retrieves current queue, responds with an error
// if one occurs
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
	w.Header().Add("Access-Control-Allow-Origin", hostaddr)
	_, err = w.Write([]byte(queue))
	if err != nil {
		log.Println("error writing queue to response")
		return
	}
}

type RecommendationRequest struct {
	SeedValues    map[string][]string `json:"seed_values"`
	PercentValues map[string]int      `json:"percent_values"`
	IntValues     map[string]int      `json:"int_values"`
	Limit         int                 `json:"limit"`
}

// performs paramterised request to spotify for requests, expects a json object
// object should be structured as RecommendationRequest struct above.
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

// performs a search request, expects JSON body matching searchRequest struct.
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
