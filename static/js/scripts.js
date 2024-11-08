let username;
const ws = new WebSocket('ws://localhost:8080/ws');

document.addEventListener('DOMContentLoaded', function() {
    const chat = document.getElementById('chat');
    const messageInput = document.getElementById('messageInput');
    const usernameModal = document.getElementById('usernameModal');
    const usernameInput = document.getElementById('usernameInput');
    const joinChatButton = document.getElementById('joinChatButton');
    const fontSelect = document.getElementById('fontSelect');
    const calcInput = document.getElementById('calcInput'); // Calculator input
    let selectedFont = fontSelect.value;

    ws.onopen = function() {
        console.log("WebSocket connection established");
    };

    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);

        if (message.type === "calculation_result") {
            displayCalculationResult(message.result);
        } else if (message.type === "file_result") {
            displayModifiedFile(message);
        } else {
            displayMessage(message.username, message.message);
        }
    };

    ws.onerror = function(error) {
        console.error("WebSocket error:", error);
    };

    ws.onclose = function() {
        console.log("WebSocket connection closed");
    };

    joinChatButton.addEventListener('click', function() {
        setUsername(usernameInput.value.trim());
    });

    fontSelect.addEventListener('change', function() {
        selectedFont = fontSelect.value;
        console.log("Selected font:", selectedFont);
    });

    messageInput.addEventListener('keydown', function(event) {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            sendMessage();
        }
    });

    usernameModal.style.display = 'flex';
});

function sendMessage() {
    const messageInput = document.getElementById('messageInput');
    if (!username) {
        alert("Please enter your username");
        return;
    }
    const message = {
        username: username,
        message: messageInput.value,
    };
    ws.send(JSON.stringify(message));
    messageInput.value = '';
}

function setUsername(name) {
    if (name) {
        username = name;
        document.getElementById('usernameModal').style.display = 'none';
        ws.send(JSON.stringify({ username: username, message: "joined" }));
    } else {
        alert("Username cannot be empty");
    }
}

function uploadFile() {
    const fileInput = document.getElementById('fileInput');
    const file = fileInput.files[0];

    if (!file) {
        alert("No file selected!");
        return;
    }

    if (file.size < 10240) {
        alert("Please upload a .txt file equal to or larger than 10KB.");
        return;
    }

    const reader = new FileReader();
    reader.onload = function(event) {
        const fileData = event.target.result;

        const fileMessage = {
            message: "file_upload",
            filename: file.name,
            data: Array.from(new Uint8Array(fileData))
        };

        console.log("Sending file message:", fileMessage);
        ws.send(JSON.stringify(fileMessage));
    };

    reader.onerror = function(error) {
        console.error("Error reading file:", error);
        alert("An error occurred while reading the file.");
    };

    reader.readAsArrayBuffer(file);
}

function sendCalculation() {
    const calcInput = document.getElementById('calcInput');
    const expression = calcInput.value.trim();

    if (!expression) {
        alert("Please enter a calculation.");
        return;
    }

    const calcMessage = {
        message: "calculate", // Label for the server
        calculation: expression,
    };

    console.log("Sending calculation request:", calcMessage);
    ws.send(JSON.stringify(calcMessage));
    calcInput.value = ''; // Clear the input field
}

function displayMessage(username, message) {
    const chat = document.getElementById('chat');
    const messageElement = document.createElement('div');
    messageElement.style.fontFamily = document.getElementById('fontSelect').value;
    messageElement.innerHTML = `<strong>${username}:</strong> ${message.replace(/\n/g, '<br>')}`;
    chat.appendChild(messageElement);
    chat.scrollTop = chat.scrollHeight;
}

function displayModifiedFile(data) {
    const chat = document.getElementById('chat');

    const fileContent = document.createElement('div');
    fileContent.innerHTML = `<strong>Modified File Content:</strong><br>${data.content.replace(/\n/g, '<br>')}`;
    chat.appendChild(fileContent);

    const downloadLink = document.createElement('a');
    downloadLink.href = data.downloadUrl;
    downloadLink.textContent = "Download Modified File";
    downloadLink.download = data.filename;
    chat.appendChild(downloadLink);

    chat.scrollTop = chat.scrollHeight;
}

function displayCalculationResult(result) {
    const chat = document.getElementById('chat');
    const resultElement = document.createElement('div');
    resultElement.innerHTML = `<strong>Calculator Result:</strong> ${result}`;
    chat.appendChild(resultElement);
    chat.scrollTop = chat.scrollHeight;
}
