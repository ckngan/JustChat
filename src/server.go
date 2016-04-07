package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"errors"
	"sync"
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
	id string
	address string
	clients int
	nextServer *ServerItem
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


// Message from new Node
type NewNodeSetup struct {
	Id string
	RPCAddress string
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

//List of clients
var clientList *ClientItem
var serverList *ServerItem

var mutexForAddingNodes sync.Mutex
var addingCond *sync.Cond

func main() {

	// Parse arguments
	usage := fmt.Sprintf("Usage: %s [client ip:port] [server ip:port] \n", os.Args[0])
	if len(os.Args) != 3 {
		fmt.Printf(usage)
		os.Exit(1)
	}

	clientConnAddress = os.Args[1]
	nodeConnAdress = os.Args[2]

	
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
	fmt.Print("This machine's address: "+ ipV4 + "\n")



	mutexForAddingNodes = sync.Mutex{}
	addingCond = sync.NewCond(&mutexForAddingNodes)

	



	//Initialize Clientlist and serverlist
	clientList = nil
	serverList = nil

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
		fmt.Print((*toPrint).username)
		toPrint = (*toPrint).nextClient
	}

	return
}


//return selectedServer, error
func getServerForCLient() (*ServerItem, error) {
	//get the server with fewest clients connected to it
	next := serverList

	//TODO: block until at least one server on list ???

	addingCond.L.Lock()
	for (serverList == nil){
		addingCond.Wait()
	}



	lowestNumberServer := serverList

	//check to see if username exists
	for next != nil {
		if (next.clients > (*next).nextServer.clients){
			lowestNumberServer = (*next).nextServer
		}

		next = (*next).nextServer
	}


	addingCond.L.Unlock()


	if (lowestNumberServer != nil){
		return lowestNumberServer, nil
	} else {
		return nil, errors.New("No Connected Servers")
	}
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

	//if username doesnt exist, add to list
	addClientToList(username, password)

	return false
}

func addNode(ident string, address string) {

	//TODO: need restart implenentation

	addingCond.L.Lock()

	newNode := &ServerItem{ident, address, 0, nil}

	if serverList == nil {
		serverList = newNode
	} else {
		newNode.nextServer = serverList
		serverList = newNode
	}

	addingCond.L.Unlock()
	addingCond.Signal()

	return
}

func isNewNode(ident string) bool {
	next := serverList 

	for next != nil {
		if (*next).id == ident {
			return false
		}
		next = (*next).nextServer
	}

	return true
}



/* 
	RPC METHODS FOR NODES

*/

//Function a node will call when it comes online
func (nodeSvc *NodeService) NewNode(message *NewNodeSetup, reply *NodeListReply) error {
	//add node to list on connection

	if (isNewNode(message.Id)){
		addNode(message.Id, message.RPCAddress)
	}

	reply.ListOfNodes = serverList

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
		selectedServer, selectionError := getServerForCLient();
		if (selectionError != nil) {
			println(selectionError.Error())
		}

		println(selectedServer)
		

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



//Error creation, etc.
/*
type error interface {
    Error() string
}
type errorString struct {
    s string
}

func (e *errorString) Error() string {
    return e.s
}

func New(text string) error {
    return &errorString{text}
}
*/