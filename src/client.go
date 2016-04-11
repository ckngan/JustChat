// Author: Haniel Martino
package main

//**************************************************************************
//
//                           IMPORT STATEMENT
//
//**************************************************************************

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"

	"clientutil"

	"github.com/arcaneiceman/GoVector/govec"
)

//**************************************************************************
//
//                  DECLARED TYPES AND GLOBAL VARIABLES
//
//**************************************************************************

const (
	Intensity_1 = 1
	Intensity_2 = 2
	Black       = 30
	Red         = 31
	Green       = 32
	Yellow      = 33
	Blue        = 34
	Magenta     = 35
	Cyan        = 36
	White       = 37
	Black_B     = 40
	Red_B       = 41
	Green_B     = 42
	Yellow_B    = 43
	Blue_B      = 44
	Magenta_B   = 45
	Cyan_B      = 46
	White_B     = 47
)

// Message Format from client
type ClientMessage struct {
	Username string
	Message  string
}

// Struct to join chat service
type NewClientSetup struct {
	Username   string
	Password   string
	RpcAddress string
}

// address of server
type ChatServer struct {
	ServerName       string
	ServerRpcAddress string
}

// FileData to build file structure in rpc call
type FileData struct {
	Username string
	FileName string
	FileSize int64
	Data     []byte
}

// Struct for client information
type ClientRequest struct {
	Username string // return the rpc address of this client
}

type ClientInfo struct {
	Username   string
	RPC_IPPORT string
}

//Retrun to client
type ServerReply struct {
	Message string
}

/* Global Variables */

// RPC of server
var NewRpcChatServer string
var chatServer *rpc.Client

// Address client is listening on for rpc calls
var clientRpcAddress string

// Load Balancers
var loadBalancers []string

// buffer Max
const bufferMax int = 50

// Client Username
var username string

// Client Password
var password string

// download directory
var downloadDirectory string

// A channel to pipe incoming messages to
var messageChannel chan string

// Load Balancer connection
var loadBalancer *rpc.Client

// GoVector log
var Logger *govec.GoLog

//locks
// this is to signal to the channel that a new message is waiting for output
var msgMutex sync.Mutex
var msgConditional *sync.Cond

/* this is to allow another rpc method to be
* called while waiting on this current method
* to finish
 */
var sendMutex sync.Mutex
var sendCond *sync.Cond

/*  this locks stdin until user finishes entering input
 */
var inputMutex sync.Mutex
var inputCond *sync.Cond

var signup string
var welcome string

//RPC Value for receiving messages
type ClientMessageService int

//**************************************************************************
//
//                     RPC METHODS FOR CHAT SERVERS
//
//**************************************************************************

// Method for a server to call client to transfer file data
func (cms *ClientMessageService) TransferFile(args *FileData, reply *ServerReply) error {
	inputCond.L.Lock()
	reply.Message = handleFileTransfer(args.FileName, args.Username, args.Data)
	inputCond.L.Unlock()
	Logger.LogLocalEvent("received public file transfer")
	return nil
}

// Method for server to call client to receive message
func (cms *ClientMessageService) ReceiveMessage(args *ClientMessage, reply *ServerReply) error {

	messageOwner := clientutil.EditText(args.Username, Yellow, Intensity_1)
	messageBody := clientutil.EditText(args.Message, Green, Intensity_1)
	output := clientutil.RemoveNewLine(messageOwner + ": " + messageBody)
	messageChannel <- output
	msgConditional.Signal()
	Logger.LogLocalEvent("received public message")

	reply.Message = ""
	return nil
}

//**************************************************************************
//
//                     RPC METHODS FOR CLIENT TO CLIENT COMMUNICATION
//
//**************************************************************************

// Method to handle private rpc messages between clients
func (cms *ClientMessageService) TransferFilePrivate(args *FileData, reply *ServerReply) error {
	inputCond.L.Lock()
	privateFlag := clientutil.EditText("PRIVATE FILE "+args.FileName+
		" FROM => ", Yellow, Intensity_1)
	messageOwner := clientutil.EditText(args.Username, Green, Intensity_1)
	output := clientutil.RemoveNewLine(privateFlag + messageOwner)
	//messageChannel <- output
	//msgConditional.Signal()
	fmt.Println(output)
	reply.Message = handleFileTransfer(args.FileName, args.Username, args.Data)
	inputCond.L.Unlock()
	Logger.LogLocalEvent("received private file transfer")
	return nil
}

// Method to handle private rpc messages from clients
func (cms *ClientMessageService) ReceivePrivateMessage(args *ClientMessage, reply *ServerReply) error {
	inputCond.L.Lock()
	privateFlag := clientutil.EditText("PRIVATE MESSAGE FROM => ", Yellow, Intensity_1)
	messageOwner := clientutil.EditText(args.Username, Green, Intensity_1)
	messageBody := clientutil.EditText(args.Message, Yellow, Intensity_1)
	output := clientutil.RemoveNewLine(privateFlag + messageOwner +
		": " + messageBody)
	messageChannel <- output
	msgConditional.Signal()
	inputCond.L.Unlock()

	Logger.LogLocalEvent("received private message")

	reply.Message = "Received"
	return nil
}

//**************************************************************************
//
//                     RPC METHODS FOR LOAD BALANCERS
//
//**************************************************************************

// Method for the load balancer to call the client to update rpc addresses of server
func (cms *ClientMessageService) UpdateRpcChatServer(args *ChatServer, reply *ServerReply) error {
	NewRpcChatServer = args.ServerRpcAddress
	reply.Message = ""
	Logger.LogLocalEvent("rpc server updated")

	// make the rpc call to the server as it's updated
	attempts := 0
	for {
		if attempts > 5 {
			disconnectClient()
			break
		}

		chatConn, err := rpc.Dial("tcp", NewRpcChatServer)
		if err != nil {
			fmt.Print(NewRpcChatServer)
			fmt.Print(clientutil.EditText("Error connecting to JustChat\n", Red, Intensity_1))
			attempts++
		} else {
			chatServer = chatConn
			initChatServerConnection()
			break
		}
	}
	sendCond.Signal()
	return nil
}

//**************************************************************************
//
//                        LOCAL HELPER FUNCTIONS
//
//**************************************************************************

/* Method to initiate client setup */
func clientSetup() {

	// initial server connection
	startupChatConnection()

	// commands to use throughout the message
	clientutil.MessageCommands()

	// main chat function
	chat()
}

// Method to initiate server connection with loadbalancer
func startupChatConnection() {

	// Connecting to a LoadBalancer
	i := 0
	for ; i < len(loadBalancers); i++ {
		conn, err := rpc.Dial("tcp", loadBalancers[i])
		if err == nil {
			// Welcome
			fmt.Println()
			fmt.Println(clientutil.EditText(signup, Yellow, Intensity_1))
			fmt.Println()
			// initializing rpc load balancer
			loadBalancer = conn
			joinLoadBalancerServer()
			break
		}
	}
	if i == 3 {
		os.Exit(-1)
	}
	return
}

// method to create 'account' with the justchat service
func joinLoadBalancerServer() {

	var reply ServerReply
	var message NewClientSetup
	// Set up buffered reader
	for {
		// read user username and send to loadbalancer
		uname := clientutil.GetClientUsername()
		pword := clientutil.GetClientPassword()
		username = uname
		message.Username = uname
		message.Password = pword
		message.RpcAddress = clientRpcAddress

		Logger.LogLocalEvent("attempt to join chat service")
		err := loadBalancer.Call("MessageService.JoinChatService", message, &reply)
		checkError(err)

		serverMessage := reply.Message
		Logger.LogLocalEvent("received message from loadbalancer")

		// Checking for welcome message
		if serverMessage == "WELCOME" {
			fmt.Println("\n\n", clientutil.EditText(welcome, Magenta, Intensity_1))
			//username = uname
			initChatServerConnection()
			break
		} else {
			fmt.Println(clientutil.EditText("Username name"+
				" already taken\n", Red, Intensity_1))
		}
	}
}

// method to announce to server after approval by loadbalancer
func initChatServerConnection() {
	var reply ServerReply
	var info ClientInfo

	info.Username = username
	info.RPC_IPPORT = clientRpcAddress

	Logger.LogLocalEvent("initialize server connection")
	err := chatServer.Call("MessageService.ConnectionInit", info, &reply)
	checkError(err)
}

// Method to get message from client's console
func getMessage() string {
	inputCond.L.Lock()
	message := ""
	reader := bufio.NewReader(os.Stdin)
	for {
		inputMsg, _ := reader.ReadString('\n')
		message = inputMsg
		if len(message) > 0 {
			break
		} else {
			fmt.Println(clientutil.EditText("Must enter 1 or "+
				"more characters", Red, Intensity_1))
		}
	}
	inputCond.L.Unlock()
	return clientutil.RemoveNewLine(message)
}

// Method to handle all chat input from client console
func chat() {
	for {
		inputMessage := getMessage()
		messageArr := strings.Split(inputMessage, "#")
		filterAndSendMessage(messageArr)
	}
}

/* method to filter messages based on structure and to call corresponding
 * rpc methods or client methods
 */
func filterAndSendMessage(msg []string) {

	var reply ServerReply
	var sendMsg ClientMessage

	command := msg[0]
	if len(msg) == 1 {
		sendMsg.Message = command
		sendMsg.Username = username
		sendCond.L.Lock()

		Logger.LogLocalEvent("sending public message")

		err := chatServer.Call("MessageService.SendPublicMsg", sendMsg, &reply)
		for err != nil {
			sendCond.Wait()
			Logger.LogLocalEvent("sending public message again")

			err = chatServer.Call("MessageService.SendPublicMsg", sendMsg, &reply)
		}

		sendCond.L.Unlock()

	} else if len(msg) == 2 {
		command = strings.TrimSpace(msg[1])
		if command == "commands" {
			clientutil.MessageCommands()
			return
		} else if command == "available files" {
			getFileList()
		} else {
			clientutil.IncorrectCommand()
			return
		}
	} else if len(msg) == 3 {
		command = strings.TrimSpace(msg[1])
		file := strings.TrimSpace(msg[2])

		if command == "share" {
			sendPublicFile(file)
		} else if command == "get" {
			getFile(file)
		} else {
			clientutil.IncorrectCommand()
			return
		}
	} else if len(msg) == 4 {

		command = strings.TrimSpace(msg[1])
		user := strings.TrimSpace(msg[2])
		message := strings.TrimSpace(msg[3])

		if command == "share" {
			sendPrivateFile(user, message)
		} else if command == "message" {
			sendPrivateMessage(user, message)
		} else {
			clientutil.IncorrectCommand()
			return
		}
	}
}

// method to send public file to all clients
func sendPublicFile(filepath string) {

	var reply ServerReply
	//var fileData FileData
	var fileData clientutil.FileData
	fileData, err := clientutil.PackageFile(username, filepath)
	if err == nil {
		sendCond.L.Lock()

		Logger.LogLocalEvent("sending public file")

		err = chatServer.Call("MessageService.SendPublicFile", fileData, &reply)
		for err != nil {
			sendCond.Wait()
			Logger.LogLocalEvent("sending public file again")

			err = chatServer.Call("MessageService.SendPublicFile", fileData, &reply)
		}

		sendCond.L.Unlock()
	} else {
		messageChannel <- "Please check filepath"
	}

	return
}

// method to send private file in filepath to user
func sendPrivateFile(user string, filepath string) {
	var request ClientRequest
	var reply ClientInfo

	request.Username = user
	sendCond.L.Lock()
	Logger.LogLocalEvent("request private client addr")

	err := chatServer.Call("MessageService.SendPrivate", request, &reply)
	for err != nil {
		sendCond.Wait()
		Logger.LogLocalEvent("request private client addr again")
		err = chatServer.Call("MessageService.SendPrivate", request, &reply)
	}
	sendCond.L.Unlock()

	Logger.LogLocalEvent("private client addr rcvd")

	privateClient, err := rpc.Dial("tcp", reply.RPC_IPPORT)
	if err != nil {
		messageChannel <- "Seems like " + user +
			" is not accepting connections. Please try again!."
		msgConditional.Signal()
		return
	} else {
		var reply ServerReply
		fileData, err := clientutil.PackageFile(username, filepath)
		if err != nil {
			messageChannel <- "Error packaging file to send privately. Please try again!"
			msgConditional.Signal()
			return
		}

		Logger.LogLocalEvent("sending private file")
		err = privateClient.Call("ClientMessageService.TransferFilePrivate", fileData, &reply)
		if err != nil {
			messageChannel <- "Error transfering file to " + user +
				". Please try again!"
			msgConditional.Signal()
			return
		}
		messageChannel <- clientutil.EditText(user, Yellow, Intensity_1) + " " +
			reply.Message + " your file transfer."
		msgConditional.Signal()
	}
	return
}

// method to send private message based on user and message
func sendPrivateMessage(user string, message string) {
	var request ClientRequest
	var reply ClientInfo

	request.Username = user
	sendCond.L.Lock()

	Logger.LogLocalEvent("request private client addr")
	err := chatServer.Call("MessageService.SendPrivate", request, &reply)
	for err != nil {
		sendCond.Wait()
		Logger.LogLocalEvent("request private client addr again")
		err = chatServer.Call("MessageService.SendPrivate", request, &reply)
	}
	sendCond.L.Unlock()

	Logger.LogLocalEvent("private client addr rcvd")

	privateClient, err := rpc.Dial("tcp", reply.RPC_IPPORT)
	if err != nil {
		messageChannel <- "Seems like " + user +
			" is not accepting connections. Please try again!."
		msgConditional.Signal()
		return
	} else {
		var reply ServerReply
		var clientMessage ClientMessage
		clientMessage.Username = username
		clientMessage.Message = message

		Logger.LogLocalEvent("sending private message")
		err = privateClient.Call("ClientMessageService.ReceivePrivateMessage", clientMessage, &reply)
		if err != nil {
			messageChannel <- "Error sending private message to " + user +
				". Please try again!"
			msgConditional.Signal()
			return
		}
		messageChannel <- clientutil.EditText(user, Yellow, Intensity_1) + " " +
			reply.Message + " your private message."
		msgConditional.Signal()
	}
	return
}

// Method for user to accept or decline file transfers
func receiveFilePermission(filename string) (flag bool) {
	request := clientutil.EditText("Do you want to receive the file "+
		filename+" (Y/N)?", Blue_B, Intensity_1)
	reader := bufio.NewReader(os.Stdin)
	flag = false
	for {
		fmt.Print(request)
		permitInput, _ := reader.ReadString('\n')
		permit := strings.TrimSpace(permitInput)
		yes := strings.EqualFold(permit, "y")
		no := strings.EqualFold(permit, "n")
		if yes {
			flag = true
			break
		} else if no {
			flag = false
			break
		} else {
			fmt.Println(clientutil.EditText("Invalid command,"+
				" please use (Y/N)", Red, Intensity_1))
		}
	}
	return
}

// Method to handle the receipt of files
func handleFileTransfer(filename string, user string, filedata []byte) string {
	// option to receive file
	if receiveFilePermission(filename) {
		path := downloadDirectory
		// creating file to be written to
		newFile, err := os.Create(path + filename)
		if err == nil {
			// writing file received from rpc
			n, err := newFile.Write(filedata)
			if err != nil {
				messageChannel <- "Error: Cannot write file"
				msgConditional.Signal()
				return "Declined"
			}
			fmt.Println()
			output := "Receive file " +
				clientutil.EditText(filename, Yellow, Intensity_1) +
				" with size " + strconv.Itoa(n) +
				" bytes from " + clientutil.EditText(user, Yellow, Intensity_1) + "."
			messageChannel <- output
			msgConditional.Signal()
			return "Received"
		} else {
			messageChannel <- "Error: Check filepath"
			msgConditional.Signal()
			return "Declined"
		}
	} else {
		return "Declined"
	}
}

// method to retrieve available files on server
func getFileList() {
	var reply []string
	Logger.LogLocalEvent("request for file list")
	err := loadBalancer.Call("MessageService.GetFileList", "", &reply)
	if err != nil {
		messageChannel <- "Error retrieving file list. Please try again."
		msgConditional.Signal()
	} else {
		if len(reply) < 1 {
			Logger.LogLocalEvent("received file list")
			messageChannel <- "No new files to retrieve"
			msgConditional.Signal()
		} else {
			Logger.LogLocalEvent("received file list")
			for i, filename := range reply {
				messageChannel <- clientutil.EditText("Available File "+
					strconv.Itoa(i+1)+":"+filename, Green, Intensity_1)
				msgConditional.Signal()
			}
		}
	}
}

// method to get an available file from server
func getFile(filename string) {
	var reply FileData
	Logger.LogLocalEvent("made file request")
	err := chatServer.Call("MessageService.GetFile", filename, &reply)
	if err != nil {
		messageChannel <- "Error retrieving file. Please try again."
		msgConditional.Signal()
	} else {
		if reply.Username == "404" {
			messageChannel <- "File not found. Please try again."
			msgConditional.Signal()
		} else {
			_ = handleFileTransfer(reply.FileName, reply.Username, reply.Data)
			Logger.LogLocalEvent("file received")
		}
	}
}

// Method to print messages to console in order of receipt from channel
func flushToConsole() {
	for {
		msgConditional.L.Lock()
		select {
		case message := <-messageChannel:
			fmt.Println(message)
		default:
			msgConditional.Wait()
		}
		msgConditional.L.Unlock()
	}
}

//**************************************************************************
//
//                             MAIN METHOD
//
//**************************************************************************
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [loadbalancer ip:port1] "+
				"[loadbalancer ip:port2] [loadbalancer ip:port3]\n",
			os.Args[0])
		os.Exit(1)
	}

	loadBalancer1 := os.Args[1]
	loadBalancer2 := os.Args[2]
	loadBalancer3 := os.Args[3]
	loadBalancers = []string{loadBalancer1, loadBalancer2, loadBalancer3}

	msgMutex = sync.Mutex{}
	msgConditional = sync.NewCond(&msgMutex)

	sendMutex = sync.Mutex{}
	sendCond = sync.NewCond(&sendMutex)

	inputMutex = sync.Mutex{}
	inputCond = sync.NewCond(&sendMutex)

	// allocate messageChannel for global access
	messageChannel = make(chan string, bufferMax)
	signup = "<----------------------- JustChat Signup ----------------------->"
	welcome = "<--------------------- Welcome to JustChat --------------------->"
	// Registering RPC service for client's server
	clientService := new(ClientMessageService)
	rpc.Register(clientService)

	ip := clientutil.GetIP()
	// listen on first open port server finds
	clientServer, err := net.Listen("tcp", ip+":0")
	if err != nil {
		fmt.Println("Client Server Error:", err)
		return
	}

	// Do something to advertise global rpc address
	clientRpcAddress = clientServer.Addr().String()
	// download folder for files
	downloadDirectory = "../Downloads/"
	_ = os.MkdirAll(downloadDirectory, 0777)

	fmt.Println("Client RPC IP:Port --> ", clientRpcAddress)

	// Create log
	Logger = govec.InitializeMutipleExecutions("client "+clientRpcAddress, "sys")
	Logger.LogThis("Client was initialized", "client "+
		clientRpcAddress, "{\"client "+clientRpcAddress+"\":1}")

	// go routine to start rpc connection for client
	go func() {
		for {
			// Accept connection from incoming clients/servers
			conn, err := clientServer.Accept()
			if err != nil {
				log.Fatal("Connection error:", err)
			}
			go rpc.ServeConn(conn)
			// Accept call from loadbalancer/server/client
		}
	}()
	// method to receive messages from channel
	go flushToConsole()

	clientSetup()

	clientServer.Close()
	os.Exit(0)
}

// ========================== Shutdown ============================ //

func disconnectClient() {
	// Can be a bit more verbose
	os.Exit(-1)
}

// Function to handle runtime errors
func checkError(err error) {
	if err != nil {
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
