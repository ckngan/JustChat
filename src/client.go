// Author: Haniel Martino
package main

import (
	"bufio"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"github.com/arcaneiceman/GoVector/govec"
)

//Message Format from client
type ClientMessage struct {
	UserName string
	Message  string
}

//Retrun to client
type ServerReply struct {
	Message string
}

/* Global Variables */
var username string
var chatServerIPPort string
var rpcChatServerIPPort string
var client *rpc.Client
var Logger = govec.Initialize("client", "client")

//RPC Value for receiving messages
type MessageService int

// Main method to setup for client
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [chatServer TCP ip:port]\n",
			os.Args[0])
		os.Exit(1)
	}

	chatServerIPPort = os.Args[1]

	// Welcome
	fmt.Println("\n\n", editText("<----------------------- JustChat Signup ----------------------->", 33, 1), "\n\n")
	clientSetup()
}

/* Method to initiate client setup */
func clientSetup() {

	// initial chat server connection
	startupChatConnection()

	// commands to use throughout the message
	messageCommands()

	// rpc connection to receive and send messages
	startupMessagingProtocol()
}

// Method to get client's username
func getClientUsername() string {

	// Reading input from user for username
	uname := ""
	reader := bufio.NewReader(os.Stdin)
	for {

		fmt.Print(editText("Please enter your username:", 44, 1), " ")
		inputUsername, _ := reader.ReadString('\n')
		//uname = strings.TrimSpace(inputUsername)
		uname = inputUsername
		if len(uname) > 0 {
			break
		} else {
			fmt.Println(editText("Must enter 1 or more characters\n", 31, 1))
		}
	}
	return uname
}

// Method to initiate server connection
func startupChatConnection() {
	// Connect to server
	conn, err := net.Dial("tcp", chatServerIPPort)
	checkError(err)

	// Set up buffered reader
	reader := bufio.NewReader(conn)
	for {
		// read user username and send to chat server
		uname := getClientUsername()
		//outgoing := Logger.PrepareSend("send username to chat server", []byte(uname + "\n"))
		//conn.Write(outgoing)
		conn.Write([]byte(uname + "\n"))

		serverMessage, _ := reader.ReadString('\n')

		// Logger.UnpackReceive("server message", _, serverMessage)

		// Checking for welcome message
		if serverMessage == "Welcome:"+uname {
			fmt.Println("\n\n", editText("<--------------------- Welcome to"+
				" JustChat --------------------->", 35, 1))
			username = uname
			break
		} else {

			fmt.Println(editText("Username name already taken\n", 31, 1))
		}
	}

	// If this point is reached, set global rpc address
	serverMessage, _ := reader.ReadString('\n')
	//fmt.Println(serverMessage)
	msg := strings.Split(serverMessage, "-")
	if msg[0] == "rpcAddress" {
		rpcChatServerIPPort = msg[1]
	} else {
		fmt.Println(editText("Unexpected Message, Shutting down....", 41, 1))
		os.Exit(-1)
	}
	return
}

func startupMessagingProtocol() {

	// Registering RPC service for client message receiver
	clientMessageService := new(MessageService)
	rpc.Register(clientMessageService)

	//chatServer, err := rpc.Dial("tcp", rpcChatServerIPPort)
	//checkError(err)
	//_ = getClientMessage()
	/*if (strings.ContainsAny(message, "#")) {
		commands := parseMessage(message)
	}*/

}

/* Method to check messages currently in message buffer to print to console
*/
func getNewMessages() {

}
// method to receive messages from the console
func getClientMessage() string {
	// Before we receive client message, check if their are any messages
	// to push to console.
	getNewMessages()

	// Reading input from user for message
	message := ""
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(editText(username, 44, 1), ": ")
		inputMsg, _ := reader.ReadString('\n')
		//uname = strings.TrimSpace(inputUsername)
		message = inputMsg
		if len(message) > 0 {
			break
		} else {
			fmt.Println(editText("Must enter 1 or more characters", 31, 1))
		}
	}
	return message
}

/* Utility Functions */

// method to check messages for commands (this method may be best suited on server)
func parseMessage(message string) []string {
	return nil
}

// method to print the commands users can use
func messageCommands(){

	color1 := 41
	color2 := 33

	privateMessage1 := editText("Private Message", color1, 1)
	message1 := editText("#username #some_message", color2, 2)

	privateMessage2 := editText("Private File", color1, 1)
	message2 := editText("#share #username #/path/to/file/name", color2, 2)

	publicMessage1 := editText("Public File", color1, 1)
	message3  := editText("#share #/path/to/file/name", color2, 2)

	fmt.Printf(editText("<----------------------- JustChat Commands ----------------------->\n", color2, 1))
	fmt.Printf("%s ==> %2s\n", privateMessage1, message1)
	fmt.Printf("   %s ==> %2s\n", privateMessage2, message2)
	fmt.Printf("    %s ==> %2s\n", publicMessage1, message3)
	fmt.Printf(editText("<----------------------------------------------------------------->\n\n", color2, 1))
	fmt.Printf(editText("<--------------------------Start Chatting------------------------->\n", 35, 1))

}

/* Function to edit output text color
/* Foreground/Background colors
 *			3/40	Black
 *			3/41	Red
 *		  3/42	Green
 *			3/43	Yellow
 *			3/44	Blue
 *			3/45	Magenta
 *			3/46	Cyan
 *			3/47	White
 */

func editText(text string, color int, intensity int) string {
	return "\x1b[" + strconv.Itoa(color) + ";" +
		strconv.Itoa(intensity) + "m" +
		text + "\x1b[0m"
}

// Function to handle runtime errors
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
