package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"

	"github.com/arcaneiceman/GoVector/govec"
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

type ServerItem struct {
	Id         string
	Address    string
	Clients    int
	NextServer *ServerItem
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

type NodeListReply struct {
	ListOfNodes *ServerItem
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

//NewStorageNode Args
type NewNodeSetup struct {
	RPC_CLIENT_IPPORT string
	RPC_SERVER_IPPORT string
	UDP_IPPORT        string
}

//reply from node with message
type NodeReply struct {
	Message string
}

/*
	----GLOBAL VARIABLES----
*/
//Net Info of this server
var clientConnAddress string
var nodeConnAdress string
var lbDesignation int

//List of All LoadBalance Servers
var LBServers []string

//List of clients
var clientList *ClientItem
var serverList *ServerItem

//List of locks
var mutexForAddingNodes sync.Mutex
var addingCond *sync.Cond

var mutexForAddingClients sync.Mutex
var clientSync *sync.Cond

// GoVector log
var Logger *govec.GoLog

func main() {

	// Parse arguments
	usage := fmt.Sprintf("Usage: %s [client ip:port] [server ip:port] [designation ( 0 | 1 | 2 )] \n", os.Args[0])
	if len(os.Args) != 4 {
		fmt.Printf(usage)
		os.Exit(1)
	}

	clientConnAddress = os.Args[1]
	nodeConnAdress = os.Args[2]
	desTmp := os.Args[3]
	lbDesignation, _ = strconv.Atoi(desTmp)

	LBServers = []string{":10001", ":10002", ":10003"}

	////Print out address information
	ip := GetLocalIP()
	// listen on first open port server finds
	clientServer, err := net.Listen("tcp", ip+":0")
	if err != nil {
		fmt.Println("Client Server Error:", err)
		return
	}
	defer clientServer.Close()

	ipV4 := clientServer.Addr().String()
	fmt.Print("This machine's address: " + ipV4 + "\n")

	// Create log
	Logger = govec.InitializeMutipleExecutions("lb "+ipV4, "sys")

	//Locks
	mutexForAddingNodes = sync.Mutex{}
	addingCond = sync.NewCond(&mutexForAddingNodes)

	mutexForAddingClients = sync.Mutex{}
	clientSync = sync.NewCond(&mutexForAddingClients)

	//Initialize Clientlist and serverlist
	clientList = nil
	serverList = nil


	//Startup Method to get client list and server list from existing load balancers
	initializeLB()

	//setup to accept rpcCalls on the first availible port
	clientService := new(MessageService)
	rpc.Register(clientService)

	rpcListener, err := net.Listen("tcp", clientConnAddress)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	//Listener go function for clients
	go func() {
		for {
			println("Waiting for Client Calls")
			clientConnection, err := rpcListener.Accept()
			if err != nil {
				log.Fatal("Connection error:", err)
			}
			go rpc.ServeConn(clientConnection)
			Logger.LogLocalEvent("rpc client connection started")
			println("Accepted Call from " + clientConnection.RemoteAddr().String())
		}
	}()

	//Listener go function for other load balancers
	lBListener, _ := net.Listen("tcp", LBServers[lbDesignation])
	go func() {
		for {
			loadBalanceConnection, err := lBListener.Accept()
			if err != nil {
				log.Fatal("LoadBalancer Connection error: ", err)
			}

			go rpc.ServeConn(loadBalanceConnection)
			println("Accepted LoadBalancer Call from: " + loadBalanceConnection.RemoteAddr().String())
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

func initializeLB() {
	return
}

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
		fmt.Print((*toPrint).username)
		toPrint = (*toPrint).nextClient
	}

	return
}

//return selectedServer, error
func getServerForCLient() (*ServerItem, error) {
	//get the server with fewest clients connected to it
	next := serverList

	addingCond.L.Lock()

	for serverList == nil {
		addingCond.Wait()
	}

	lowestNumberServer := serverList

	//check to see if username exists
	for next != nil {
		if next.Clients > (*next).NextServer.Clients {
			lowestNumberServer = (*next).NextServer
		}

		next = (*next).NextServer
	}

	addingCond.L.Unlock()

	if lowestNumberServer != nil {
		return lowestNumberServer, nil
	} else {
		return nil, errors.New("No Connected Servers")
	}
}

func authenticationFailure(username string, password string) bool {

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

	//if username doesnt exist, add to list
	addClientToList(username, password)

	return false
}

func addNode(ident string, address string) {

	//TODO: need restart implementation

	addingCond.L.Lock()

	newNode := &ServerItem{ident, address, 0, nil}

	if serverList == nil {
		serverList = newNode
	} else {
		newNode.NextServer = serverList
		serverList = newNode
	}

	addingCond.L.Unlock()
	addingCond.Signal()

	return
}

func isNewNode(ident string) bool {
	next := serverList

	for next != nil {
		if (*next).Id == ident {
			return false
		}
		next = (*next).NextServer
	}

	return true
}

/*
	RPC METHODS FOR NODES
*/

//Function a node will call when it comes online
func (nodeSvc *NodeService) NewNode(message *NewNodeSetup, reply *NodeListReply) error {
	//add node to list on connection
	println("A new node is trying to connect")
	/*if isNewNode(message.Id) {
		addNode(message.Id, message.RPCAddress)
	}*/

	reply.ListOfNodes = serverList

	Logger.LogLocalEvent("new storage node online")

	return nil
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

	//check username, if taken reply username taken
	if authenticationFailure(message.UserName, message.Password) {
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

		//Dial and update the client with their server address
		rpcUpdateMessage.ServerName = "NameOfServer"
		rpcUpdateMessage.ServerRpcAddress = ":7000"
		selectedServer, selectionError := getServerForCLient()
		if selectionError != nil {
			println(selectionError.Error())
		}

		println(selectedServer)

		callErr := clientConn.Call("ClientMessageService.UpdateRpcChatServer", rpcUpdateMessage, &clientReply)
		if callErr != nil {
			reply.Message = "DIAL-ERROR"
			return nil
		}

		Logger.LogLocalEvent("client joined chat service successful")
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

/* Get local IP */
// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
