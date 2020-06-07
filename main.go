package main

import (
	"fmt"
	"log"
	"time"
	"net/http"
	"html/template"
	"strconv"
	"math/rand"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var connections = make(map[string]map[*websocket.Conn]bool)
var score = make(map[string]map[int]int)
var move = make(map[string]map[int]string)
var query = make(map[int]http.ResponseWriter)
var queryRoom = make(map[string]int)
var matchingCap int = 1
var room string = ""

type Message struct {
	Command int
	Data string
}

type Response struct {
	Type string
	Value string
}

type HtmlData struct {
	Room string
}

func main() {
	r := mux.NewRouter();

	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/room/{name}", mainHandler).Methods("GET")
	r.HandleFunc("/game/{room}", gameConnect).Methods("GET")

	r.HandleFunc("/query", queryHandler).Methods("POST")

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	var name string = mux.Vars(r)["name"]
	data := HtmlData{Room: name}
	template, _ := template.ParseFiles("html/index.html")
	template.Execute(w, data);
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	var lineNumber int = len(query)

	if lineNumber % 2 == 0 {
		randChar := make([]byte, 3)
		for i := 0; i < 3; i++ {
			randChar[i] = byte(65 + rand.Intn(90-65));
		}
		room = fmt.Sprintf("%s%s%s-%s", string(randChar[0]), string(randChar[1]), string(randChar[2]) , strconv.Itoa((matchingCap-1)/2))
	}

	query[lineNumber] = w
	for {
		time.Sleep(50 * time.Millisecond)
		if len(query) > matchingCap{break}
	}
	query[lineNumber] = nil

	queryRoom[room]++
	if queryRoom[room] > 1 {
		matchingCap += 2
	}

	fmt.Fprintln(w, room)
}

func gameConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w	, r, nil)

	if err != nil {
		fmt.Println(err)
		return
	}

	url :=  string(r.URL.Path)
	room := url[6:]
	player := int(len(connections[room]) + 1)

	go conn.WriteJSON(Response{Type: "Player", Value: fmt.Sprint(player)})

	if connections[room] == nil {
		connections[room] = make(map[*websocket.Conn]bool)
		score[room] = make(map[int]int)
		move[room] = make(map[int]string)
	}

	if len(connections[room]) > 1 {
		conn.Close()
		return
	}

	connections[room][conn] = true
	defer closeRoom(room)
	score[room][player] = 0
	if(player == 2){
		response := Response{Type: "gameStart", Value: "1"}
		messageClients(room, response)
	}

	gameLoop(room, player, conn)
}

func gameLoop(room string, player int, conn *websocket.Conn){
	for{
		msg := Message{}

		err := conn.ReadJSON(&msg)
		if err != nil{
			fmt.Println(err)
			return
		}

		response := Response{Type: "", Value: "0"}

		if msg.Command == 1 {
			response.Type = "RoundResult"
			if move[room][player] == "" {
				move[room][player] = msg.Data
			}

			if move[room][1] != "" && move[room][2] != "" {
				p1move := move[room][1]
				p2move := move[room][2]
				if((p1move == "s" && p2move == "p") || (p1move == "p" && p2move == "r") || (p1move == "r" && p2move == "s")) {
					response.Value = "1"
					score[room][1]++
				}else if((p2move == "s" && p1move == "p") || (p2move == "p" && p1move == "r") || (p2move == "r" && p1move == "s")) {
					response.Value = "2"
					score[room][2]++
				}
				move[room][1] = ""
				move[room][2] = ""
				if score[room][1] > 2 || score[room][2] > 2 {
					response.Type = "FinalResult"
					go func(){
						time.Sleep(time.Second * 5)
						closeRoom(room)
					}()
				}
				messageClients(room, response)
			}

		} else if msg.Command == 2 {
			response.Type = fmt.Sprintf("%s%s%s", "Player", strconv.Itoa(player), "Message")
			response.Value = msg.Data
			messageClients(room, response)
		}
	}
}

func closeRoom(room string) {
	for conn := range connections[room] {
		conn.Close()
	}
	connections[room] = nil
}

func messageClients(room string, response Response) {
	for conn := range connections[room] {
		conn.WriteJSON(response)
	}
}

