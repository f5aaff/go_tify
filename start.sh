#!/bin/bash

bash -c "go run main.go &"
sleep 5
bash -c "firefox -P gotify --headless -url http://localhost:3000/player/app/"
