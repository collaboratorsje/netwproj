package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]string)        // Map client connections to room names
var rooms = make(map[string]map[*websocket.Conn]bool) // Room name -> list of connections
var roomPasscodes = make(map[string]string)           // Store passcodes for each room
var broadcast = make(chan Message)

const DefaultRoom = "default"

type Message struct {
	Username    string `json:"username"`
	Message     string `json:"message"`
	Calculation string `json:"calculation,omitempty"`
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleConnections)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("server_files"))))

	go handleMessages()

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

	// Ensure the default room exists
	if _, exists := rooms[DefaultRoom]; !exists {
		rooms[DefaultRoom] = make(map[*websocket.Conn]bool)
	}

	// Assign the client to the default room
	rooms[DefaultRoom][conn] = true
	clients[conn] = DefaultRoom

	defer func() {
		roomName := clients[conn]
		delete(rooms[roomName], conn)
		delete(clients, conn)
	}()

	for {
		var rawMessage map[string]interface{}
		err := conn.ReadJSON(&rawMessage)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		log.Printf("Received raw message: %v", rawMessage)

		// Handle message types
		if action, ok := rawMessage["action"].(string); ok {
			switch action {
			case "chat":
				handleChatMessage(conn, rawMessage)
			case "calculate":
				handleCalculation(conn, rawMessage)
			case "file_upload":
				handleFile(conn, rawMessage)
			case "create_room":
				handleCreateRoom(conn, rawMessage)
			case "join_room":
				handleJoinRoom(conn, rawMessage)
			case "leave_room":
				handleLeaveRoom(conn) // New case for leaving a room
			default:
				log.Printf("Unknown action: %s", action)
			}
		} else {
			// Legacy messages without "action"
			handleLegacyMessages(conn, rawMessage)
		}
	}
}

func handleLegacyMessages(conn *websocket.Conn, rawMessage map[string]interface{}) {
	if rawMessage["message"] == "joined" {
		msg := Message{
			Username: rawMessage["username"].(string),
			Message:  "joined",
		}
		broadcast <- msg
	} else if rawMessage["message"] == "calculate" {
		handleCalculation(conn, rawMessage)
	} else {
		log.Printf("Invalid legacy message: %v", rawMessage)
	}
}

func handleCreateRoom(conn *websocket.Conn, rawMessage map[string]interface{}) {
	roomName := rawMessage["roomName"].(string)
	passcode := rawMessage["passcode"].(string)

	if _, exists := rooms[roomName]; exists {
		response := Message{
			Username: "Kangaroo",
			Message:  "Room already exists.",
		}
		conn.WriteJSON(response)
		return
	}

	rooms[roomName] = make(map[*websocket.Conn]bool)
	roomPasscodes[roomName] = passcode
	response := Message{
		Username: "Kangaroo",
		Message:  fmt.Sprintf(`Room "%s" created with Passcode "%s".`, roomName, passcode),
	}
	conn.WriteJSON(response)
}

func handleJoinRoom(conn *websocket.Conn, rawMessage map[string]interface{}) {
	roomName := rawMessage["roomName"].(string)
	passcode := rawMessage["passcode"].(string)

	if roomClients, exists := rooms[roomName]; exists {
		if roomPasscodes[roomName] == passcode {
			oldRoom := clients[conn]
			delete(rooms[oldRoom], conn)

			roomClients[conn] = true
			clients[conn] = roomName
			response := Message{
				Username: "Kangaroo",
				Message:  fmt.Sprintf(`Joined room: "%s".`, roomName),
			}
			conn.WriteJSON(response)
		} else {
			response := Message{
				Username: "Kangaroo",
				Message:  "Invalid passcode.",
			}
			conn.WriteJSON(response)
		}
	} else {
		response := Message{
			Username: "Kangaroo",
			Message:  "Invalid room name or passcode.",
		}
		conn.WriteJSON(response)
	}
}

func handleLeaveRoom(conn *websocket.Conn) {
	currentRoom := clients[conn]

	if _, exists := rooms[currentRoom]; exists {
		delete(rooms[currentRoom], conn)
		clients[conn] = DefaultRoom
		rooms[DefaultRoom][conn] = true

		response := Message{
			Username: "Kangaroo",
			Message:  fmt.Sprintf(`You have left room: "%s" and joined default room.`, currentRoom),
		}

		conn.WriteJSON(response)
	} else {
		response := Message{
			Username: "Kangaroo",
			Message:  "You are not currently in a valid room.",
		}
		conn.WriteJSON(response)
	}
}

func handleChatMessage(conn *websocket.Conn, rawMessage map[string]interface{}) {
	var msg Message
	if err := mapToStruct(rawMessage, &msg); err != nil {
		log.Printf("Error parsing chat message: %v", err)
		return
	}

	if msg.Message == fmt.Sprintf("Hello from Client %s", msg.Username) {
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
		// Send the message to everyone in the client's current room
		roomName := clients[conn]
		for client := range rooms[roomName] {
			if err := client.WriteJSON(msg); err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				delete(rooms[roomName], client)
			}
		}
	}
}

func handleCalculation(conn *websocket.Conn, rawMessage map[string]interface{}) {
	calculation, ok := rawMessage["calculation"].(string)
	if !ok {
		response := Message{
			Username: "Kangaroo",
			Message:  "Invalid calculation request format.",
		}
		conn.WriteJSON(response)
		return
	}

	// Evaluate the expression
	result, err := evaluateExpression(calculation)

	var response Message
	if err != nil {
		response = Message{
			Username: "Kangaroo",
			Message:  fmt.Sprintf("Invalid calculation: %s. Supported functions: sin, cos, log, etc.", err.Error()),
		}
	} else {
		response = Message{
			Username: "Kangaroo",
			Message:  fmt.Sprintf("Result: %s = %.2f", calculation, result),
		}
	}

	conn.WriteJSON(response)
}

func evaluateExpression(expr string) (float64, error) {
	normalizedExpr := normalizeExpression(expr)

	// Register custom functions
	functions := map[string]govaluate.ExpressionFunction{
		"log": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("log expects one argument")
			}
			value, ok := args[0].(float64)
			if !ok {
				return nil, fmt.Errorf("invalid argument for log")
			}
			return math.Log(value), nil
		},
		"log10": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("log10 expects one argument")
			}
			value, ok := args[0].(float64)
			if !ok {
				return nil, fmt.Errorf("invalid argument for log10")
			}
			return math.Log10(value), nil
		},
		"sin": func(args ...interface{}) (interface{}, error) {
			value, ok := args[0].(float64)
			if !ok {
				return nil, fmt.Errorf("invalid argument for sin")
			}
			return math.Sin(value), nil
		},
		"cos": func(args ...interface{}) (interface{}, error) {
			value, ok := args[0].(float64)
			if !ok {
				return nil, fmt.Errorf("invalid argument for cos")
			}
			return math.Cos(value), nil
		},
	}

	expression, err := govaluate.NewEvaluableExpressionWithFunctions(normalizedExpr, functions)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %v", err)
	}

	// Evaluate the expression
	result, err := expression.Evaluate(nil)
	if err != nil {
		return 0, fmt.Errorf("evaluation error: %v", err)
	}

	return result.(float64), nil
}

// Normalize the expression: make lowercase and handle custom operators
func normalizeExpression(expr string) string {
	return strings.ToLower(expr) // Normalize to lowercase for functions like Sin
}

func handleFile(conn *websocket.Conn, rawMessage map[string]interface{}) {
	filename := rawMessage["filename"].(string)
	fileData := rawMessage["data"].([]interface{})

	// Convert file data to []byte
	byteData := make([]byte, len(fileData))
	for i, v := range fileData {
		byteData[i] = byte(v.(float64))
	}

	log.Printf("Received file %s:\n%s", filename, string(byteData))

	// Save original file locally
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

	// Provide download link for the modified file
	downloadUrl := fmt.Sprintf("http://localhost:8080/files/%s", modifiedFilename)

	// Send back modified file and download link
	response := map[string]interface{}{
		"type":        "file_result", // Used by frontend to display file
		"filename":    modifiedFilename,
		"content":     string(modifiedData), // File content as string for display
		"downloadUrl": downloadUrl,          // URL to download file
	}

	err = conn.WriteJSON(response)
	if err != nil {
		log.Printf("Error sending modified file to client: %v", err)
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
