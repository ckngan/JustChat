// Author: Haniel Martino
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"syscall"

	"github.com/arcaneiceman/GoVector/govec"
	"golang.org/x/crypto/ssh/terminal"
)

// Message Format from client
type ClientMessage struct {
	UserName string
	Message  string
	Password string
}

// Struct to join chat service
type NewClientSetup struct {
	UserName   string
	Password   string
	RpcAddress string
}

// address of chat server
type ChatServer struct {
	ServerName       string
	ServerRpcAddress string
}

// FileInfoData to build file structure in rpc call
type FileData struct {
	UserName string
	FileName string
	FileSize int64
	Data     []byte
}

//Retrun to client
type ServerReply struct {
	Message string
}

//Retrun to client
type ClientReply struct {
	Message string
}

/* Global Variables */

// RPC of chat server
var NewRpcChatServer string

// Address client is listening on for rpc calls
var clientRpcAddress string

// Load Balancers
var loadBalancers []string

// buffer Max
const bufferMax int = 100

// Storage for incoming messages
var incomingMessageBuffer []ClientMessage

// Client Username
var username string

// Client Password
var password string

// A 'pointer' that keeps track of new incoming messages
var bufferCount int
var readBufferCount int

// Load Balancer connection
var loadBalancer *rpc.Client

// GoVector log
var Logger *govec.GoLog

//RPC Value for receiving messages
type ClientMessageService int

// Method for a client to call another client to transfer file data
func (cms *ClientMessageService) TransferFile(args *FileData, reply *ClientReply) error {

	directory := getDownloadDirectory()
	// creating file to be written to
	newFile, err := os.Create(directory + "/" + args.FileName)
	checkError(err)

	// writing file received from rpc
	n, err := newFile.Write(args.Data)
	checkError(err)
	fmt.Printf("File Received with %d bytes from %s", n, args.UserName)
	fmt.Println()
	reply.Message = "Received"
	Logger.LogLocalEvent("received file transfer")
	return nil
}

// Method the load balancer calls on the clients to update rpc addresses
func (cms *ClientMessageService) UpdateRpcChatServer(args *ChatServer, reply *ClientReply) error {
	NewRpcChatServer = args.ServerRpcAddress
	reply.Message = ""
	Logger.LogLocalEvent("rpc chat server updated")
	return nil
}

// Method for server to call client to receive message
func (cms *ClientMessageService) ReceiveMessage(args *ClientMessage, reply *ClientReply) error {
	var clientMessage ClientMessage
	clientMessage.UserName = args.UserName
	clientMessage.Message = args.Message

	if bufferCount < bufferMax {
		incomingMessageBuffer[bufferCount] = clientMessage
		bufferCount++
	} else {
		// reset buffer Count
		bufferCount = 0
		incomingMessageBuffer[bufferCount] = clientMessage
	}
	Logger.LogLocalEvent("server received message: " + clientMessage.Message)
	return nil
}

func handleRpc(server net.Conn) {

}

// Main method to setup for client
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [ip:port1 ip:port2 ip:port3]\n",
			os.Args[0])
		os.Exit(1)
	}

	loadBalancer1 := os.Args[1]
	loadBalancer2 := os.Args[2]
	loadBalancer3 := os.Args[3]
	loadBalancers = []string{loadBalancer1, loadBalancer2, loadBalancer3}

	incomingMessageBuffer = make([]ClientMessage, bufferMax)

	// Registering RPC service
	clientService := new(ClientMessageService)
	rpc.Register(clientService)

	ip := getIP()
	// listen on first open port server finds
	clientServer, err := net.Listen("tcp", ip+":0")
	if err != nil {
		fmt.Println("Client Server Error:", err)
		return
	}
	defer clientServer.Close()
	
	// Do something to advertise global rpc address
	clientRpcAddress = clientServer.Addr().String()

	fmt.Println("Client IP:Port --> ", clientRpcAddress)

	// Create log
	Logger = govec.InitializeMutipleExecutions(clientRpcAddress, "sys")

	// go routine to start rpc connection for client
	go func() {
		for {
			// Accept connection from incoming clients/servers
			conn, err := clientServer.Accept()
			if err != nil {
				log.Fatal("Connection error:", err)
			}
			go rpc.ServeConn(conn)
			Logger.LogLocalEvent("rpc connection started")
			// Accept call from loadbalancer/server/client
		}
	}()

	// continuously check for server connection -- possibly chat serverMessage
	// if chat server disconnects, reconnect to load balancer, saving state
	clientSetup()
}

/* Method to initiate client setup */
func clientSetup() {

	// initial chat server connection
	startupChatConnection()

	// commands to use throughout the message
	messageCommands()

}

// Method to initiate server connection
func startupChatConnection() {

	// Welcome
	fmt.Println()
	fmt.Println(editText("<----------------------- JustChat Signup ----------------------->", 33, 1))
	fmt.Println()
	// eventually will be used in some loop
	n := 0
	// Connecting to a LoadBalancer
	conn, err := rpc.Dial("tcp", loadBalancers[n])
	checkError(err)
	Logger.LogLocalEvent("connected to a loadBalancer")

	// initializing rpc load balancer
	loadBalancer = conn

	var reply ServerReply
	var message NewClientSetup
	// Set up buffered reader
	for {
		// read user username and send to chat server
		uname := getClientUsername()
		pword := getClientPassword()
		message.UserName = uname
		message.Password = pword
		message.RpcAddress = clientRpcAddress

		err := loadBalancer.Call("MessageService.JoinChatService", message, &reply)
		checkError(err)
		Logger.LogLocalEvent("joined chat service")

		serverMessage := reply.Message
		Logger.LogLocalEvent("received message from server")

		// Checking for welcome message
		if serverMessage == "WELCOME" {
			fmt.Println("\n\n", editText("<--------------------- Welcome to"+
				" JustChat --------------------->", 35, 1))
			username = uname
			break
		} else {
			fmt.Println(editText("Username name already taken\n", 31, 1))
		}
	}
	return
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

// Method to get client's username
func getClientPassword() string {

	// Reading input from user for username
	pword := ""
	for {
		fmt.Print(editText("Please enter your password:", 44, 1), " ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))

		/*if err == nil {
			fmt.Println("\nPassword typed: " + string(bytePassword))
		}*/

		inputPword := string(bytePassword)
		pword = inputPword
		if len(pword) > 3 {
			break
		} else {
			fmt.Println(editText("Must enter 3 or more characters\n", 31, 1))
		}
	}
	return pword
}

// method to receive download directory from user
// method also checks if directory exists
func getDownloadDirectory() string {

	// Reading input from user for download
	filename := ""
	command := "Please enter download directory to receive file"
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(editText(command, 44, 1), ": ")
		inputMsg, _ := reader.ReadString('\n')
		filename = inputMsg
		if len(filename) > 0 {
			break
		} else {
			fmt.Println(editText("Must enter 1 or more characters", 31, 1))
		}
	}
	return filename
}

// method to print the commands users can use
func messageCommands() {

	color1 := 41
	color2 := 33

	privateMessage1 := editText("Private Message", color1, 1)
	message1 := editText("#username #some_message", color2, 2)

	privateMessage2 := editText("Private File", color1, 1)
	message2 := editText("#share #username #/path/to/file/name", color2, 2)

	publicMessage1 := editText("Public File", color1, 1)
	message3 := editText("#share #/path/to/file/name", color2, 2)

	fmt.Printf(editText("<----------------------- JustChat Commands ----------------------->\n", color2, 1))
	fmt.Printf("%s ==> %2s\n", privateMessage1, message1)
	fmt.Printf("   %s ==> %2s\n", privateMessage2, message2)
	fmt.Printf("    %s ==> %2s\n", publicMessage1, message3)
	fmt.Printf(editText("<----------------------------------------------------------------->\n\n", color2, 1))
	fmt.Printf(editText("<--------------------------Start Chatting------------------------->\n", 35, 1))

}

// get ip address of the host for client's server
func getIP() (ip string) {

	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil && !ipv4.IsLoopback() {
			//fmt.Println("IPv4: ", ipv4.String())
			ip = ipv4.String()
		}
	}
	return ip
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
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
