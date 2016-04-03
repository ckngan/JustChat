package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
)

/*
	----DECLARED TYPES----
*/
//RPC Value for recieving messages
type MessageService int

//Client object
type ClientItem struct {
	username   string
	recMsgConn *net.Conn
	status     string
	nextClient *ClientItem
}

// Message Format from client
type ClientMessage struct {
	UserName string
	Message  string
	Password string
	RpcAddress string
}

// Struct to join chat service
type NewClientSetup struct {
	UserName string
	Password string
	RpcAddress string
}
//Retrun to client
type ServerReply struct {
	Message string
}

/*
	----GLOBAL VARIABLES----
*/
//Net Info of this server
var serverIPPort string

//List of clients
var clientList *ClientItem

func main() {

	// Parse arguments
	usage := fmt.Sprintf("Usage: %s [server ip:port]\n", os.Args[0])
	if len(os.Args) != 2 {
		fmt.Printf(usage)
		os.Exit(1)
	}
	serverIPPort = os.Args[1]

	//Initialize Clientlist
	clientList = nil

	//setup to accept rpcCalls on the first availible port
	clientService := new(MessageService)
	rpc.Register(clientService)

	rpcListener, err := net.Listen("tcp", ":0")
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

	//Handle client connection setup
	clientSetupListener, err := net.Listen("tcp", serverIPPort)
	checkError(err)

	for {
		newClientConn, err := clientSetupListener.Accept()
		checkError(err)
		println("New Client Connection")
		//setupNewClient(net.conn, rpcAddress)
		fmt.Println(rpcListener.Addr().String())
		go setupNewClient(&newClientConn, rpcListener.Addr().String())
	}

	/*otherListen, err := net.Listen("tcp", "localhost:10000")
	checkError(err)
	for {
		//println("Blocking for Listen")
		_, err := otherListen.Accept();
		checkError(err)
	}*/

	//println("END")

}

/*
	LOCAL HELPER FUNCTIONS
*/
func setupNewClient(clientConn *net.Conn, rpcAddrString string) {
	//Send the information for the client to make RPC Calls to server
	//     format of data is [IP]:[PORT]
	//     depending on implementation, may have to split to obtain port
	userName, _, err := bufio.NewReader(*clientConn).ReadLine()
	checkError(err)
	println("Got username: " + string(userName))

	connectionAttempts := 1

	for usernameTaken(string(userName)) {
		if connectionAttempts == 5 {
			(*clientConn).Close()
			return
		}

		(*clientConn).Write([]byte("USERNAME-TAKEN" + "\n"))
		println("Letting client know username is taken")
		userName, _, err = bufio.NewReader(*clientConn).ReadLine()
		checkError(err)
		println("Got new username: " + string(userName))
		connectionAttempts++
	}

	(*clientConn).Write([]byte("Welcome:" + string(userName) + "\n"))
	(*clientConn).Write([]byte("rpcAddress-"+ rpcAddrString + "\n"))
	println("Welcoming " + string(userName))

	addClientToList(string(userName))
}

func addClientToList(username string) {

	newClient := &ClientItem{username, nil, "ONLINE", nil}

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

func usernameTaken(username string) bool {
	next := clientList
	for next != nil {
		if (*next).username == username {
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
	// otherwise, server replies, USERNAME-TAKEN

	return nil
}

//Function for recieving a message from a client
func (msgSvc *MessageService) SendMessage(message *ClientMessage, reply *ServerReply) error {

	sendToAllClients(message.UserName, message.Message)

	reply.Message = "Message Recieved"

	return nil
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
