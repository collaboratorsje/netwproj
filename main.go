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

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

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

	clients[conn] = true
	defer delete(clients, conn)

	for {
		var rawMessage map[string]interface{}
		err := conn.ReadJSON(&rawMessage)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Route message types
		if rawMessage["message"] == "calculate" {
			handleCalculation(conn, rawMessage)
			continue
		}

		if rawMessage["message"] == "file_upload" {
			handleFile(conn, rawMessage)
			continue
		}

		handleChatMessage(conn, rawMessage)
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
		broadcast <- msg
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

	modifiedData := append(byteData, []byte("\nThis is an added line from the server.")...)
	modifiedFilename := "modified_" + filename

	err = os.WriteFile(fmt.Sprintf("server_files/%s", modifiedFilename), modifiedData, 0644)
	if err != nil {
		log.Printf("Error saving modified file: %v", err)
		return
	}

	downloadUrl := fmt.Sprintf("http://localhost:8080/files/%s", modifiedFilename)

	response := map[string]interface{}{
		"type":        "file_result",
		"filename":    modifiedFilename,
		"content":     string(modifiedData),
		"downloadUrl": downloadUrl,
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
