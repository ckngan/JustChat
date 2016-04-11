// Author: Haniel Martino
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/arcaneiceman/GoVector/govec"
	"golang.org/x/crypto/ssh/terminal"
)

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

// address of chat server
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

// RPC of chat server
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
var msgMutex sync.Mutex
var msgConditional *sync.Cond

var sendMutex sync.Mutex
var sendCond *sync.Cond

var signup string
var welcome string

//========================== Client RPC Service ======================== //
//RPC Value for receiving messages
type ClientMessageService int

// Method for a client to call another client to transfer file data
func (cms *ClientMessageService) TransferFile(args *FileData, reply *ServerReply) error {
	reply.Message = handleFileTransfer(args.FileName, args.Username, args.Data)
	Logger.LogLocalEvent("received public file transfer")
	return nil
}

// Method to handle private rpc messages from clients
func (cms *ClientMessageService) TransferFilePrivate(args *FileData, reply *ServerReply) error {
	privateFlag := editText("PRIVATE FILE "+args.FileName+" FROM => ", Yellow, Intensity_1)
	messageOwner := editText(args.Username, Green, Intensity_1)
	output := removeNewLine(privateFlag + messageOwner)
	fmt.Println(output)
	//messageChannel <- output
	//msgConditional.Signal()
	reply.Message = handleFileTransfer(args.FileName, args.Username, args.Data)
	Logger.LogLocalEvent("received private file transfer")
	return nil
}

// Method the load balancer calls on the clients to update rpc addresses
func (cms *ClientMessageService) UpdateRpcChatServer(args *ChatServer, reply *ServerReply) error {
	NewRpcChatServer = args.ServerRpcAddress
	reply.Message = ""
	Logger.LogLocalEvent("rpc chat server updated")

	// make the rpc call to the server as it's updated
	attempts := 0
	for {
		if attempts > 5 {
			disconnectClient()
			break
		}
		
		Logger.LogLocalEvent("dialing updated chat server")

		chatConn, err := rpc.Dial("tcp", NewRpcChatServer)
		if err != nil {
			fmt.Print(NewRpcChatServer)
			fmt.Print(editText("Error connecting to JustChat\n", Red, Intensity_1))
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

// Method for server to call client to receive message
func (cms *ClientMessageService) ReceiveMessage(args *ClientMessage, reply *ServerReply) error {
	messageOwner := editText(args.Username, Yellow, Intensity_1)
	messageBody := editText(args.Message, Green, Intensity_1)
	output := removeNewLine(messageOwner + ": " + messageBody)
	messageChannel <- output
	msgConditional.Signal()
	
	Logger.LogLocalEvent("received public message")

	reply.Message = ""
	return nil
}

// Method to handle private rpc messages from clients
func (cms *ClientMessageService) ReceivePrivateMessage(args *ClientMessage, reply *ServerReply) error {
	privateFlag := editText("PRIVATE MESSAGE FROM => ", Yellow, Intensity_1)
	messageOwner := editText(args.Username, Green, Intensity_1)
	messageBody := editText(args.Message, Yellow, Intensity_1)
	output := removeNewLine(privateFlag + messageOwner + ": " + messageBody)
	messageChannel <- output
	msgConditional.Signal()

	Logger.LogLocalEvent("received private message")

	reply.Message = "Received"
	return nil
}

// ========================== Helper Methods ============================ //

/* Method to initiate client setup */
func clientSetup() {

	// initial chat server connection
	startupChatConnection()

	// commands to use throughout the message
	messageCommands()

	// main chat function
	chat()
}

// Method to initiate server connection
func startupChatConnection() {

	// Connecting to a LoadBalancer
	i := 0
	for ; i < len(loadBalancers); i++ {
		Logger.LogLocalEvent("dialing a loadbalancer")
		conn, err := rpc.Dial("tcp", loadBalancers[i])
		if err == nil {
			// Welcome
			fmt.Println()
			fmt.Println(editText(signup, Yellow, Intensity_1))
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

func joinLoadBalancerServer() {

	var reply ServerReply
	var message NewClientSetup
	// Set up buffered reader
	for {
		// read user username and send to chat server
		uname := getClientUsername()
		pword := getClientPassword()
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
			fmt.Println("\n\n", editText(welcome, Magenta, Intensity_1))
			//username = uname
			initChatServerConnection()
			break
		} else {
			fmt.Println(editText("Username name already taken\n", Red, Intensity_1))
		}
	}
}

func initChatServerConnection() {
	var reply ServerReply
	var info ClientInfo

	info.Username = username
	info.RPC_IPPORT = clientRpcAddress

	Logger.LogLocalEvent("initialize chat server connection")
	err := chatServer.Call("MessageService.ConnectionInit", info, &reply)
	checkError(err)
}

// Method to get client's username
func getClientUsername() string {

	// Reading input from user for username
	uname := ""
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(editText("Please enter your username:", Blue_B, Intensity_1), " ")
		inputUsername, _ := reader.ReadString('\n')
		uname = inputUsername

		if len(uname) > 0 {
			break
		} else {
			fmt.Println(editText("Must enter 1 or more characters\n", Red, Intensity_1))
		}
	}
	reader.Reset(os.Stdin)
	return removeNewLine(uname)
}

// Method to get client's username
func getClientPassword() string {

	// Reading input from user for username
	pword := ""
	for {
		fmt.Print(editText("Please enter your password:", Blue_B, Intensity_1), " ")
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))

		if err != nil {
			fmt.Println("\nPassword typed: " + string(bytePassword))
		}
		fmt.Println()
		inputPword := string(bytePassword)
		pword = inputPword
		if len(pword) > 3 {
			break
		} else {
			fmt.Println(editText("Must enter 3 or more characters\n", Red, Intensity_1))
		}
	}
	return pword
}

// Method to get message from client's console
func getMessage() string {

	message := ""
	reader := bufio.NewReader(os.Stdin)
	//consoleUsername := strings.Split(username, "\n")[0]

	for {
		//fmt.Print(editText(consoleUsername, Blue_B, Intensity_1), ":")
		inputMsg, _ := reader.ReadString('\n')
		message = inputMsg
		if len(message) > 0 {
			break
		} else {
			fmt.Println(editText("Must enter 1 or more characters", Red, Intensity_1))
		}
	}
	reader.Reset(os.Stdin)
	return removeNewLine(message)
}

// Method to handle all chat input from client
func chat() {
	for {
		inputMessage := getMessage()
		messageArr := strings.Split(inputMessage, "#")
		filterAndSendMessage(messageArr)
	}
}

// method to filter messages and then send
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
			messageCommands()
			return
		} else if command == "available files" {
			getFileList()
		}
	} else if len(msg) == 3 {
		command = strings.TrimSpace(msg[1])
		file := strings.TrimSpace(msg[2])

		if command == "share" {
			sendPublicFile(file)
		} else if command == "get" {
			getFile(file)
		} else {
			fmt.Println("Incorrect command!!!!!")
			messageCommands()
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
			fmt.Println("Incorrect command!!!!!")
			messageCommands()
			return
		}
	}
}

// method to send public file
func sendPublicFile(filepath string) {

	var reply ServerReply
	var fileData FileData
	fileData, err := packageFile(filepath)
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

func packageFile(path string) (fileData FileData, err error) {
	//var fileData FileData

	_, file := filepath.Split(path)

	r, err := os.Open(path)
	if err != nil {
		var fileData FileData
		var err error
		return fileData, err
	}
	fis, _ := r.Stat()
	fileData.Username = username
	fileData.FileSize = fis.Size()
	fileData.FileName = file
	fileData.Data = make([]byte, fileData.FileSize)
	_, _ = r.Read(fileData.Data)
	r.Close()
	return fileData, nil

}

// method to send private file
// func (ms *MessageService) SendPrivate(args *ClientRequest, reply *ServerReply)
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
	Logger.LogLocalEvent("dialing private client")

	privateClient, err := rpc.Dial("tcp", reply.RPC_IPPORT)
	if err != nil {
		messageChannel <- "Seems like " + user + " is not accepting connections. Please try again!."
		msgConditional.Signal()
		return
	} else {
		var reply ServerReply
		fileData, err := packageFile(filepath)
		if err != nil {
			messageChannel <- "Error packaging file to send privately. Please try again!"
			msgConditional.Signal()
			return
		}

		Logger.LogLocalEvent("sending private file")
		err = privateClient.Call("ClientMessageService.TransferFilePrivate", fileData, &reply)
		if err != nil {
			messageChannel <- "Error transfering file to " + user + ". Please try again!"
			msgConditional.Signal()
			return
		}
		messageChannel <- editText(user, Yellow, Intensity_1) + " " + reply.Message + " your file transfer."
		msgConditional.Signal()
	}

	// reply should be IP port of the ...
	return
}

// method to send private message
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
	Logger.LogLocalEvent("dialing private client")

	privateClient, err := rpc.Dial("tcp", reply.RPC_IPPORT)
	if err != nil {
		messageChannel <- "Seems like " + user + " is not accepting connections. Please try again!."
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
			messageChannel <- "Error sending private message to " + user + ". Please try again!"
			msgConditional.Signal()
			return
		}
		messageChannel <- editText(user, Yellow, Intensity_1) + " " + reply.Message + " your private message."
		msgConditional.Signal()
	}
	// reply should be IP port of the ...
	return
}

// Method for user to accept or decline file transfers
func receiveFilePermission(filename string) (flag bool) {
	request := editText("Do you want to receive the file "+filename+" (Y/N)?", Blue_B, Intensity_1)
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
			fmt.Println(editText("Invalid command, please use (Y/N)", Red, Intensity_1))
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
			output := "Receive file " + editText(filename, Yellow, Intensity_1) + " with size " + strconv.Itoa(n) + " bytes from " + editText(user, Yellow, Intensity_1) + "."
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
				messageChannel <- editText("Available File "+strconv.Itoa(i+1)+":"+filename, Green, Intensity_1)
				msgConditional.Signal()
			}
		}
	}
}

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

// Method to print messages to console in order of receipt
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

// method to print the commands users can use
func messageCommands() {

	color1 := Red_B
	color2 := Yellow

	// 3 strings (message in first)
	privateMessage1 := editText("Private Message", color1, Intensity_1)
	message1 := editText("#message #username #some_message", color2, Intensity_2)

	// 3 strings (share in first)
	privateMessage2 := editText("Private File", color1, Intensity_1)
	message2 := editText("#share #username #/path/to/file/name", color2, Intensity_2)

	// 2 strings (share in first)
	publicMessage1 := editText("Public File", color1, Intensity_1)
	message3 := editText("#share #/path/to/file/name", color2, Intensity_2)

	// 1 string (share in first)
	commands := editText("Commands", color1, Intensity_1)
	message4 := editText("#commands", color2, Intensity_2)

	files := editText("Available Files", color1, Intensity_1)
	message5 := editText("#available files", color2, Intensity_2)

	getFile := editText("Get File", color1, Intensity_1)
	message6 := editText("#get #filename", color2, Intensity_2)

	fmt.Printf(editText("<----------------------- JustChat Commands ----------------------->\n", color2, Intensity_1))
	fmt.Printf("%s ==> %2s\n", privateMessage1, message1)
	fmt.Printf("   %s ==> %2s\n", privateMessage2, message2)
	fmt.Printf("    %s ==> %2s\n", publicMessage1, message3)
	fmt.Printf("%s ==> %2s\n", files, message5)
	fmt.Printf("       %s ==> %2s\n", getFile, message6)
	fmt.Printf("       %s ==> %2s\n", commands, message4)
	fmt.Printf(editText("<----------------------------------------------------------------->\n\n", color2, Intensity_1))
	fmt.Printf(editText("<--------------------------Start Chatting------------------------->\n", Magenta, Intensity_1))

}

// ========================== Utility Methods ============================ //

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

func disconnectClient() {
	// Can be a bit more verbose
	os.Exit(-1)
}

func removeNewLine(value string) (str string) {
	re := regexp.MustCompile(`\r?\n`)
	str = re.ReplaceAllString(value, "")
	return
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

// ========================== Main Method ============================ //
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [loadbalancer ip:port1] [loadbalancer ip:port2] [loadbalancer ip:port3]\n",
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

	// allocate messageChannel for global access
	messageChannel = make(chan string, bufferMax)
	signup = "<----------------------- JustChat Signup ----------------------->"
	welcome = "<--------------------- Welcome to JustChat --------------------->"
	// Registering RPC service for client's server
	clientService := new(ClientMessageService)
	rpc.Register(clientService)

	ip := getIP()
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

	fmt.Println("Client IP:Port --> ", clientRpcAddress)

	// Create log
	Logger = govec.InitializeMutipleExecutions("client "+clientRpcAddress, "sys")
	Logger.LogThis("Client was initialized", "client "+clientRpcAddress, "{\"client "+clientRpcAddress+"\":1}")

	// go routine to start rpc connection for client
	go func() {
		for {
			// Accept connection from incoming clients/servers
			conn, err := clientServer.Accept()
			if err != nil {
				log.Fatal("Connection error:", err)
			}
			go rpc.ServeConn(conn)
			Logger.LogLocalEvent("client rpc connection started")
			// Accept call from loadbalancer/server/client
		}
	}()
	// method to receive messages from channel
	go flushToConsole()

	clientSetup()

	clientServer.Close()
	os.Exit(0)
}
