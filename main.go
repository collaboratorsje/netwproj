package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func main() {
	// Serve the main page
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleConnections)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("server_files"))))

	// Start a goroutine to handle chat messages
	go handleMessages()

	// Start the HTTP server
	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("static/index.html")
	if err != nil {
		http.Error(w, "Could not load template", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	defer delete(clients, conn)

	for {
		var rawMessage map[string]interface{}
		err := conn.ReadJSON(&rawMessage)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		if rawMessage["message"] == "file_upload" {
			handleFile(conn, rawMessage)
		} else {
			handleMessage(conn, rawMessage)
		}
	}
}

func handleFile(conn *websocket.Conn, rawMessage map[string]interface{}) {
	filename := rawMessage["filename"].(string)
	fileData := rawMessage["data"].([]interface{})

	// Convert file data to []byte
	byteData := make([]byte, len(fileData))
	for i, v := range fileData {
		byteData[i] = byte(v.(float64))
	}

	// Print the file content to the server console
	log.Printf("Received file %s:\n%s", filename, string(byteData))

	// Save the original file locally
	err := os.WriteFile(fmt.Sprintf("server_files/%s", filename), byteData, 0644)
	if err != nil {
		log.Printf("Error saving file: %v", err)
		return
	}

	// Append a new line to the file
	modifiedData := append(byteData, []byte("\nThis is an added line from the server.")...)
	modifiedFilename := "modified_" + filename

	// Save the modified file locally
	err = os.WriteFile(fmt.Sprintf("server_files/%s", modifiedFilename), modifiedData, 0644)
	if err != nil {
		log.Printf("Error saving modified file: %v", err)
		return
	}

	// Create a fake download link (in a real app, serve this via HTTP)
	downloadUrl := fmt.Sprintf("http://localhost:8080/files/%s", modifiedFilename)

	// Send the modified file content and download link to the client
	response := map[string]interface{}{
		"filename":    modifiedFilename,
		"content":     string(modifiedData),
		"downloadUrl": downloadUrl,
	}
	err = conn.WriteJSON(response)
	if err != nil {
		log.Printf("Error sending modified file to client: %v", err)
	}
}

func handleMessage(conn *websocket.Conn, rawMessage map[string]interface{}) {
	var msg Message
	if err := mapToStruct(rawMessage, &msg); err != nil {
		log.Printf("Error parsing message: %v", err)
		return
	}

	if msg.Message == "joined" {
		welcomeMsg := Message{
			Username: "System",
			Message:  fmt.Sprintf("Welcome %s!", msg.Username),
		}
		broadcast <- welcomeMsg
	} else if msg.Message == fmt.Sprintf("Hello from Client %s", msg.Username) {
		conn.WriteJSON(msg)
		response := Message{
			Username: "Kangaroo",
			Message:  "Hello from Server Kangaroo",
		}
		conn.WriteJSON(response)
	} else if msg.Message == fmt.Sprintf("Bye from Client %s", msg.Username) {
		conn.WriteJSON(msg)
		goodbyeMsg := Message{
			Username: "Kangaroo",
			Message:  "Goodbye! (Refresh the page to establish a new connection with the server)",
		}
		conn.WriteJSON(goodbyeMsg)
		return
	} else {
		broadcast <- msg
	}
}

func handleMessages() {
	for msg := range broadcast {
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func mapToStruct(input map[string]interface{}, output interface{}) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, output)
}
