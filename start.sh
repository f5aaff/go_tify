#!/bin/bash
firefox -CreateProfile gootify
firefox -P gootify --headless -url http://localhost:3000/player/app/
