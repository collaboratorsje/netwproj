let username;
const ws = new WebSocket('ws://localhost:8080/ws');

document.addEventListener('DOMContentLoaded', function() {
    const chat = document.getElementById('chat');
    const messageInput = document.getElementById('messageInput');
    const usernameModal = document.getElementById('usernameModal');
    const joinRoomModal = document.getElementById('joinRoomModal');
    const createRoomModal = document.getElementById('createRoomModal');
    const usernameInput = document.getElementById('usernameInput');
    const joinChatButton = document.getElementById('joinChatButton');
    const fontSelect = document.getElementById('fontSelect');
    const calcInput = document.getElementById('calcInput'); // Calculator input
    let selectedFont = fontSelect.value;

    // Ensure only the username modal is shown on load
    usernameModal.style.display = 'flex';
    joinRoomModal.style.display = 'none';
    createRoomModal.style.display = 'none';

    ws.onopen = function() {
        console.log("WebSocket connection established");
    };

    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);
    
        if (message.type === "calculation_result") {
            displayCalculationResult(message.result);
        } else if (message.type === "file_result") {
            displayModifiedFile(message);  // Handle file upload responses
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
});

function sendMessage() {
    const messageInput = document.getElementById('messageInput');
    if (!username) {
        alert("Please enter your username");
        return;
    }

    const message = {
        action: 'chat',  // Add this action field for chat messages
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
        ws.send(JSON.stringify({ 
            action: 'chat', 
            username: username, 
            message: "joined" 
        }));
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

    if (file.size < 10240) {  // Ensure file size is >=10KB
        alert("Please upload a .txt file equal to or larger than 10KB.");
        return;
    }

    const reader = new FileReader();
    reader.onload = function(event) {
        const fileData = event.target.result;

        const fileMessage = {
            action: "file_upload",  // Make sure this matches what the server expects
            filename: file.name,
            data: Array.from(new Uint8Array(fileData))  // Send file as byte array
        };

        console.log("Sending file message:", fileMessage);
        ws.send(JSON.stringify(fileMessage));  // Send to the server
    };

    reader.onerror = function(error) {
        console.error("Error reading file:", error);
        alert("An error occurred while reading the file.");
    };

    reader.readAsArrayBuffer(file);  // Read the file as binary
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
    downloadLink.textContent = `Download Modified File: ${data.filename}`;
    downloadLink.download = data.filename;

    // Wrap download link in a new div for spacing
    const downloadContainer = document.createElement('div');
    downloadContainer.appendChild(downloadLink);

    // Add spacing below the download link
    const spacing = document.createElement('div');
    spacing.innerHTML = '<br>'; // Add a line break after the link

    chat.appendChild(downloadContainer);
    chat.appendChild(spacing); // Append the spacing div to create additional line breaks

    chat.scrollTop = chat.scrollHeight; // Scroll to the latest message
}

function displayCalculationResult(result) {
    const chat = document.getElementById('chat');
    const resultElement = document.createElement('div');
    resultElement.innerHTML = `<strong>Calculator Result:</strong> ${result}`;
    chat.appendChild(resultElement);
    chat.scrollTop = chat.scrollHeight;
}

// Room Modal Functions
function openCreateRoomModal() {
    document.getElementById('createRoomModal').style.display = 'flex';
}

function closeCreateRoomModal() {
    document.getElementById('createRoomModal').style.display = 'none';
}

function openJoinRoomModal() {
    document.getElementById('joinRoomModal').style.display = 'flex';
}

function closeJoinRoomModal() {
    document.getElementById('joinRoomModal').style.display = 'none';
}

function createRoom() {
    const roomName = document.getElementById('roomNameInput').value.trim();
    const passcode = document.getElementById('roomPasscodeInput').value.trim();

    if (!roomName || !passcode) {
        alert('Both room name and passcode are required.');
        return;
    }

    ws.send(JSON.stringify({
        action: 'create_room',
        roomName: roomName,
        passcode: passcode
    }));

    closeCreateRoomModal();
}

function joinRoom() {
    const roomName = document.getElementById('joinRoomNameInput').value.trim();
    const passcode = document.getElementById('joinRoomPasscodeInput').value.trim();

    if (!roomName || !passcode) {
        alert('Both room name and passcode are required.');
        return;
    }

    ws.send(JSON.stringify({
        action: 'join_room',
        roomName: roomName,
        passcode: passcode
    }));

    closeJoinRoomModal();
}

function leaveRoom() {
    ws.send(JSON.stringify({ 
        action: 'leave_room', 
        username: username 
    }));
    alert("You have left the current room and rejoined the main chat.");
}
