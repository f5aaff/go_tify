package main

import (
	"encoding/json"
	"fmt"
	"github.com/f5aaff/spotify-wrappinator/requests"
	"io"
	"log"
	"net/http"
	"strconv"
	"github.com/go-chi/chi/v5"
	"context"
)

func GetDevices(w http.ResponseWriter, r *http.Request) {

	getDevicesRequest := requests.New(requests.WithRequestURL("me/player/devices"), requests.WithBaseURL("https://api.spotify.com/v1/"))
	requests.GetRequest(a, getDevicesRequest)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(getDevicesRequest.Response)
	if err != nil {
		fmt.Println(err)
		return
	}
//	res, err := json.MarshalIndent(d, "", " ")
//	if err != nil {
//		w.WriteHeader(http.StatusInternalServerError)
//		_, err := w.Write([]byte("error obtaining devices"))
//		if err != nil {
//			return
//		}
//	}
//	w.WriteHeader(http.StatusOK)
//	_, err = w.Write(res)
//	if err != nil {
//		log.Println("error writing device to response")
//	}
}

// gets current device, responds with the json from spotify.
// responds with json representation of device, responds with error
// if one occurs
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

// decreases volume, simple get request, decreases by 10.
// gets the current device, sends a request with the current device volume - 10
func incVol(w http.ResponseWriter, r *http.Request) {
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

	incVolRequest := requests.New(requests.WithRequestURL("me/player/volume"), requests.WithBaseURL("https://api.spotify.com/v1/"))
	newvol := d.VolumePercent + 10
	newvolstr := strconv.Itoa(newvol)
	requests.PutRequest(a, incVolRequest, requests.Fields("volume_percent", newvolstr))
	w.WriteHeader(http.StatusOK)
}

// decreases volume, simple get request, decreases by 10.
// gets the current device, sends a request with the current device volume - 10
func decVol(w http.ResponseWriter, r *http.Request) {
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
	decVolRequest := requests.New(requests.WithRequestURL("me/player/volume"), requests.WithBaseURL("https://api.spotify.com/v1/"))
	newvol := d.VolumePercent - 10
	newvolstr := strconv.Itoa(newvol)
	requests.PutRequest(a, decVolRequest, requests.Fields("volume_percent", newvolstr))
	w.WriteHeader(http.StatusOK)
}

func TransferPlayback(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	pcRequest := requests.New(requests.WithBaseURL("https://api.spotify.com/v1/"), requests.WithRequestURL("me/player"))

	PutBodyRequest(a, pcRequest, body)
	if pcRequest.Response != nil {
		http.Error(w, "error transferring playback\t"+http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(pcRequest.Response)
	if err != nil {
		log.Printf("error writing response: %s", err.Error())
	}
}

func TransferPlaybackCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		device := chi.URLParam(r, "device")
		for _, x := range allowedPlayerFunctions {
			if device == x {
				var key contextKey = "device"
				ctx := context.WithValue(r.Context(), key, device)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, http.StatusText(404), 404)
	})
}
