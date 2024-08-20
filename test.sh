#!/bin/bash

VERBOSE=$4
PostReq() {
    METHOD=$1
    PATH=$2
    DATA=$3
    /usr/bin/curl -s -X $METHOD "http://localhost:3000/$PATH" --header "Content-Type=application/json" --data "$DATA"
    if [[ "$VERBOSE" ]]; then
        echo "curl -s -X $METHOD http://localhost:3000/$PATH --header \"Content-Type=application/json\" --data \"$DATA\""
    fi
}
GetReq() {
    PATH=$1
    /usr/bin/curl -s http://localhost:3000/$PATH
    echo "curl -s http://localhost:3000/$PATH"
}

MakeActive() {
    DEVICE=$(GetReq "devices/all" | jq .devices[0].id)
    DATA="{\"device_ids\":[$DEVICE]}"
    PostReq "POST" "devices/transfer" $DATA
}
search() {
    INPUT=$1

    if [[ -z "$INPUT" ]]; then
        INPUT="thy art is murder"
    fi

    DATA="
    {
        \"query\" : \"$INPUT\",
        \"tags\": null,
        \"types\" : [\"track\"],
        \"Market\": \"GB\",
        \"Limit\" : 1
    }"

    PostReq "POST" "search" "$DATA"
}


ACTION=$1
INPUT=$2
case "$ACTION" in
"makeActive")
    MakeActive
    ;;

"getRecent")
    MakeActive
    GetReq "player/recently-played"
    ;;

"getPlaylists")
    GetReq "playlists"
    ;;

"search")
    search $INPUT
    ;;

"PlayCustom")
    URI=$(search $INPUT | jq .tracks.items[].uri)
    echo $URI
    PostReq "POST" "player/play" "$(cat playCustom.json)"
    ;;

"POST")
    PostReq "POST" $INPUT $3
    ;;

"GET")
    GetReq $INPUT
    ;;

*)
    GetReq "devices"
    ;;

esac
