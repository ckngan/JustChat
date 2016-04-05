package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
)

/*
	----DECLARED TYPES----
*/
//RPC Values
type MessageService int
type NodeService int

//Client object
type ClientItem struct {
	username   string
	password   string
	nextClient *ClientItem
}

// Message Format from client
type ClientMessage struct {
	UserName   string
	Message    string
	Password   string
	RpcAddress string
}

// Struct to join chat service
type NewClientSetup struct {
	UserName   string
	Password   string
	RpcAddress string
}

//Retrun to client
type ServerReply struct {
	Message string
}

//Retrun to client
type ClientReply struct {
	Message string
}

// address of chat server
type ChatServer struct {
	ServerName       string
	ServerRpcAddress string
}

/*
	----GLOBAL VARIABLES----
*/
//Net Info of this server
var clientConnAddress string
var nodeConnAdress string

//List of clients
var clientList *ClientItem

func main() {

	// Parse arguments
	usage := fmt.Sprintf("Usage: %s [client ip:port] [server ip:port] \n", os.Args[0])
	if len(os.Args) != 3 {
		fmt.Printf(usage)
		os.Exit(1)
	}

	clientConnAddress = os.Args[1]
	nodeConnAdress = os.Args[2]

	//Initialize Clientlist
	clientList = nil

	//setup to accept rpcCalls on the first availible port
	clientService := new(MessageService)
	rpc.Register(clientService)

	rpcListener, err := net.Listen("tcp", clientConnAddress)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	go func() {
		for {
			println("Waiting for Client Calls")
			clientConnection, err := rpcListener.Accept()
			if err != nil {
				log.Fatal("Connection error:", err)
			}
			go rpc.ServeConn(clientConnection)
			println("Accepted Call from " + clientConnection.RemoteAddr().String())
		}
	}()

	//setup to accept rpcCalls from message servers
	messageNodeService := new(NodeService)
	rpc.Register(messageNodeService)

	//Handle message/storage connection setup
	messageNodeListener, err := net.Listen("tcp", nodeConnAdress)
	checkError(err)

	for {
		messageNodeConn, err := messageNodeListener.Accept()
		checkError(err)
		if err != nil {
			log.Fatal("Connection error:", err)
		}
		go rpc.ServeConn(messageNodeConn)
	}
}

/*
	LOCAL HELPER FUNCTIONS
*/
func addClientToList(username string, password string) {

	newClient := &ClientItem{username, password, nil}

	if clientList == nil {
		clientList = newClient
	} else {
		newClient.nextClient = clientList
		clientList = newClient
	}

	//print list of clients
	toPrint := clientList
	println(" ")
	println("List of Clients")
	println("---------------")
	for toPrint != nil {
		println((*toPrint).username)
		toPrint = (*toPrint).nextClient
	}

	return
}

func authenticateFailure(username string, password string) bool {
	next := clientList

	//check to see if username exists
	for next != nil {
		if (*next).username == username {
			if (*next).password == password {
				//username match and password match
				return false
			}
			//username exists but password doesn't match
			return true
		}
		next = (*next).nextClient
	}

	return false
}

func sendToAllClients(from string, message string) {

}

/*
	RPC METHODS FOR CLIENTS

*/

//Function for receiving a message from a client
func (msgSvc *MessageService) JoinChatService(message *NewClientSetup, reply *ServerReply) error {

	// if user name not taken, server dials RPC address in message.RPCAddress
	// and updates client with new rpc address, then replies WELCOME
	// unless there is error dialing RPC to client then replies DIAL-ERROR
	// otherwise, server replies, USERNAME-TAKEN

	//check username
	//if taken reply username taken
	if authenticateFailure(message.UserName, message.Password) {

		reply.Message = "USERNAME-TAKEN"

		//else dial rpc
	} else {

		clientConn, err := rpc.Dial("tcp", message.RpcAddress)
		if err != nil {
			reply.Message = "DIAL-ERROR"
			return nil
		}

		var clientReply ClientReply
		var rpcUpdateMessage ChatServer

		//Dial and update the cient with their server address
		rpcUpdateMessage.ServerName = "NameOfServer"
		rpcUpdateMessage.ServerRpcAddress = ":7000"

		callErr := clientConn.Call("ClientMessageService.UpdateRpcChatServer", rpcUpdateMessage, &clientReply)
		if callErr != nil {
			reply.Message = "DIAL-ERROR"
			return nil
		}

		reply.Message = "WELCOME"

	}

	return nil
}

/*
	CHECK for ERRORS

*/
func checkError(err error) {
	if err != nil {
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
