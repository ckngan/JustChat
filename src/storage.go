package main

import (
	//"bufio"
	//"io"
	"fmt"
	"log"
	"net"
	//"strings"
	"net/rpc"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"
)

/*
	----DECLARED TYPES----
*/
//RPC Value for recieving messages
type NodeService int
type MessageService int

// Reply
type ValReply struct {
	Val string // value; depends on the call
}

type ServerReply struct {
	Message string // value; depends on the call
}

type NodeListReply struct {
	ListOfNodes *ServerItem
}

type ServerItem struct {
	UDP_IPPORT        string
	RPC_CLIENT_IPPORT string
	RPC_SERVER_IPPORT string
	Clients           int
	NextServer        *ServerItem
}

//NewStorageNode Args
type NewNodeSetup struct {
	RPC_CLIENT_IPPORT string
	RPC_SERVER_IPPORT string
	UDP_IPPORT        string
}

// Message Format from client
type ClientMessage struct {
	UserName string
	Message  string
}

type ClientRequest struct {
	UserName          string // client making the request for the username
	RequestedUsername string // return the rpc address of this client
	RpcAddress        string // RpcAddress of the client making the request
	FileName          string
}

// FileInfoData to build file structure in rpc call
type FileData struct {
	UserName string
	FileName string
	FileSize int64
	Data     []byte
}

//Client object
type ClientItem struct {
	Username   string
	RPC_IPPORT   string
	NextClient *ClientItem
}

//cient info
type ClientInfo struct {
	UserName string
	RPC_IPPORT string
}


//GLOBALS
var LOAD_BALANCER_IPPORT string
var SEND_PING_IPPORT string
var RECEIVE_PING_ADDR string
var RPC_SYSTEM_IPPORT string
var RPC_CLIENT_IPPORT string
var serverList *ServerItem
var serverListMutex *sync.Mutex
var clientList *ClientItem
var clientListMutex *sync.Mutex


//****************************BACK-END RPC METHODS***********************************//
func (nodeSvc *NodeService) NewStorageNode(args *NewNodeSetup, reply *ServerReply) error {
	println("A new server node has joined the system")

	println("RPC IP PORT: " + args.RPC_SERVER_IPPORT + " UDP IPPORT " + args.UDP_IPPORT)
	addNode(args.UDP_IPPORT, args.RPC_CLIENT_IPPORT, args.RPC_SERVER_IPPORT)

	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) SendPublicMsg(args *ClientMessage, reply *ValReply) error {
	println("We received a new message")
	println("username: " + args.UserName + " Message: " + args.Message)
	//for each client currently connected
		//send the message

	reply.Val = "success"
	return nil
}

func (nodeSvc *NodeService) SendPublicFile(args *ClientMessage, reply *ServerReply) error {
	println("We received a new message")
	println("username: " + args.UserName + " Message: " + args.Message)
	//find file by name and user upload? and send it

	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) StoreFile(args *FileData, reply *ServerReply) error {
	println("YOU'VE BEEN CHOSEN TO STORE A FILE :D")
	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) GetFile(args *FileData, reply *ServerReply) error {
	println("gimme shit")
	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) DeleteFile(args *FileData, reply *ServerReply) error {
	println("delete that shit i told you to store")
	reply.Message = "success"
	return nil
}



//***********************CLIENT RPC METHODS **********************************************//
//method for joining the storage node
func (msgSvc *MessageService) ConnectionInit(message *ClientInfo, reply *ServerReply) error {
		println("someone wants to join us :D")
	//create client node
	//add to linked list
	//
	reply.Message = "success"
	return nil
}


// method for public message transfer
func (ms *MessageService) SendPublicMsg(args *ClientMessage, reply *ServerReply) error {
	println("We received a new message")
	println("username: " + args.UserName + " Message: " + args.Message)
	//for each node in serverList
		//call their sendPublicMsg methid wth these args
	//for each client currently connected
		//send the message
	reply.Message = "success"
	return nil
}

// method for public file transfer
func (ms *MessageService) SendPublicFile(args *FileData, reply *ServerReply) error {
	println("We received a new file")
	println("username: " + args.UserName + "filename:" + args.FileName)
	reply.Message = "success"
	return nil
}

// Method to request client information for private correspondence
func (ms *MessageService) SendPrivate(args *ClientRequest, reply *ServerReply) error {
	println("We received a new file")
	println("username requested: " + args.UserName + "filename:" + args.FileName)
	//find requested user's IP and send it back
	reply.Message = "success"
	return nil
}


func main() {
	////////////////////////////////////////////////////////////////////////////////////////
	// PARSE ARGS
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [load_balancer_ip:port1 udp_ping_ip:port2]\n",
			os.Args[0])
		os.Exit(1)
	}

	LOAD_BALANCER_IPPORT = os.Args[1]
	SEND_PING_IPPORT = os.Args[2]
	println("LOAD_BALANCER: ", LOAD_BALANCER_IPPORT, " SEND_PINGS: ", SEND_PING_IPPORT)

	serverListMutex = &sync.Mutex{}
	clientListMutex = &sync.Mutex{}
 	////////////////////////////////////////////////////////////////////////////////////////
 	// LOAD BALANCER tcp.rpc


	nodeService := new(NodeService)
	rpc.Register(nodeService)
	c := make(chan int)
	go func() {
		systemListenServe("localhost:0", c)
	}()
	RPC_system_port := <-c
	RPC_SYSTEM_IPPORT = "localhost" + ":" + strconv.Itoa(RPC_system_port)
	println("RPC PORT FOR SYSTEMS: " + RPC_SYSTEM_IPPORT)
	/////////////////////////////////////////////////////////////////////////////////////////
	//CLIENT tcp.rpc

	messageService := new(MessageService)
	rpc.Register(messageService)
	ch := make(chan int)
	go func() {
		clientListenServe("localhost:0", ch)
	}()
	RPC_client_port := <-ch
	RPC_CLIENT_IPPORT = "localhost" + ":" + strconv.Itoa(RPC_client_port)
	println("RPC PORT FOR CLIENTS: " + RPC_CLIENT_IPPORT)

	/////////////////////////////////////////////////////////////////////////////////////////
	// UDP PING AND PING RECEIVE
	println("START")
	PingAddr, err := net.ResolveUDPAddr("udp", SEND_PING_IPPORT)
	checkError(err)
	ListenAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	checkError(err)
	ListenConn, err := net.ListenUDP("udp", ListenAddr)
	checkError(err)
	RECEIVE_PING_ADDR = ListenConn.LocalAddr().String()
	println("WE'RE LISTENING ON: ", RECEIVE_PING_ADDR)
	println("we're sending pings on: ", SEND_PING_IPPORT)
	joinStorageServers()

	//this is for testing but shoulh be locked
	x :=sizeOfServerList()

	println("WE RECEIVED A LIST OF SIZE: ", x)
	//println("This is the first item in the list: ", serverList.UDP_IPPORT)
	/*systemService, err := rpc.Dial("tcp", "localhost:53346")
	checkError(err)

	var kvVal ValReply;

	clientMessage := ClientMessage{
		Username : "Billy",
		Message : "I hate everybody",
		Password	: "PASSWORD"}
	err = systemService.Call("MessageService.SendPublicMsg", clientMessage, &kvVal)
	checkError(err)
	fmt.Println("Server replied: " + kvVal.Val) */
	///////////////////////////////////////////////////////////
	fmt.Println("type of: ", reflect.TypeOf(RECEIVE_PING_ADDR))

	/////////////////FAKE CLIENT DATA
///////////////////////////////////////////////

	//TESTING SENDPUBLICMSG

println("END UDP STUFF")

	systemService, err := rpc.Dial("tcp", RPC_CLIENT_IPPORT)
	checkError(err)

	var reply ValReply

	clientMessage := ClientMessage{
		UserName: "dude",
		Message: "this chat system sucks"}

	err = systemService.Call("NodeService.SendPublicMsg", clientMessage, &reply)
	checkError(err)
	fmt.Println("we received a reply from the server: ", reply.Val)


//////////////////////////////////////////////
	go pingServers(PingAddr)
	UDPService(ListenConn)

	////////////////////////////////////////////////////////////////////////////////////////

	/*
	   	  connection, err := net.Dial("tcp", "localhost:8888")
	       if err != nil {
	           fmt.Println("There was an error making a connection")
	       }
	       //file to read
	       file, err := os.Open(strings.TrimSpace("patrick-star.jpg")) // For read access.
	       if err != nil {
	           connection.Write([]byte("-1"))
	           log.Fatal(err)
	       }
	   	n, errr := io.Copy(connection, file)
	   		if errr != nil {
	       		log.Fatal(err)
	   		}
	   	file.Close()
	   	fmt.Println(n, "bytes sent")
	   	connection.Close()
	*/
}

// If error is non-nil, print it out and halt.
func checkError(err error) {
	if err != nil {
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}

/*
* Deletes server with IP:PORT equal to 'a' inside of list if it is found
 */

func deleteServerFromList(udp string) {
	next := serverList
	inner := serverList

	println("WE WANNA DELETE: ", udp)

	for next != nil {
		println("WWHAT IS NEXT ", next.UDP_IPPORT)
		if !isNewNode(udp) {
			//println("node you tryna delete is in the list")

			//println("WE COMPARING--> NEXT: ", (*next).UDP_IPPORT, " and: ", udp)
			if (*next).UDP_IPPORT == udp {
				//println("DEY DA SAME! NEXT: ", (*next).UDP_IPPORT, " wanna del: ", udp)
				//if we find the node we want to delete

				for inner != nil {
					//println("INNERLOOP ")
					//println("INNERLOOP: ", (*inner).UDP_IPPORT,"NEXT INNEPLOOP",(*inner).NextServer.UDP_IPPORT , " delete: ",(*next).UDP_IPPORT)
					//cycle through the array again and find the prior node, and make it;s next node equal to this nodes, next node.
					//handle the case where its the first node that must be deleted
					if (*inner).NextServer.UDP_IPPORT == next.UDP_IPPORT {
						//println("INNERLOOP: ", (*serverList).NextServer.UDP_IPPORT, " NEXT: ",next.UDP_IPPORT)
						(*inner).NextServer = (*next).NextServer
						//break
						return
					} else if (*inner).UDP_IPPORT == next.UDP_IPPORT {
						serverList = (*inner).NextServer
						return
					}//else if ((*inner).NextServer.UDP_IPPORT == next.UDP_IPPORT && next.NextServer == nil){
					//	(*inner).NextServer = nil
					//}
					//println("this isnt part of the plan")
						inner = (*inner).NextServer
				}
			}

		} else {
			println("Node not found in list")
		}

		next = (*next).NextServer

	}
}

/*
* cycles through list of connected servers and pings them to make sure theyre still active
 */

func pingServers(LocalAddr *net.UDPAddr){
for{

	timer1 := time.NewTimer(time.Second * 10)
	<-timer1.C

	serverListMutex.Lock()
	next := serverList
	serverListMutex.Unlock()
	for next != nil {
			ServerAddr,err := net.ResolveUDPAddr("udp",(*next).UDP_IPPORT)
    		checkError(err)
    		Conn, err := net.DialUDP("udp", LocalAddr, ServerAddr)
    		checkError(err)
    		dead := pingServer(Conn, 0)

    		if dead {
					println("Assume node", (*next).UDP_IPPORT," is dead!!!! HANDLE THAT SHIT")
					serverListMutex.Lock()
					n := sizeOfServerList()
					println("Size of list ", n)
					deleteServerFromList((*next).UDP_IPPORT)
					n = sizeOfServerList()
					serverListMutex.Unlock()
					println("Size of list ", n)

					println("This is what's in list of servers: ", serverList.UDP_IPPORT)
    			}

			next = (*next).NextServer
		}
}

}

/*
* Writes to the UDP connection for a given server and waits for a reply to make sure server is still active
 */

func pingServer(Conn *net.UDPConn, attempt int) (dead bool) {

	msg := "lbping"
	write_buf := []byte(msg)
	_, err := Conn.Write(write_buf)
	checkError(err)
	read_buf := make([]byte, 1024)
	Conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	for {
		_, _, err := Conn.ReadFromUDP(read_buf)
		if err != nil {
			handlePingReply(Conn, err, attempt)
			dead = true
			break
		} else {
			//fmt.Println("Received ",string(read_buf[0:n])," size ",n, " from ",addr)
			dead = false
			Conn.Close()
			break
		}
	}

	return
}

/*
* Checks to see if server is replying, if not it attempts to ping again, if tried more than 2 times, it returns true
* to state that the server has died
 */

func handlePingReply(Conn *net.UDPConn, err error, attempt int) {
	if e := err.(net.Error); e.Timeout() {

		if attempt < 1 {
			//try to connect to server again
			println("retrying to connect to server node. Retry attempt: " + strconv.Itoa(attempt))
			pingServer(Conn, attempt+1)

		} else {
			//assume server is dead
			Conn.Close()
		}
	}

	if e, ok := err.(net.Error); !ok || !e.Timeout() {
		// error that isn't a timeout error
		println(e)
		Conn.Close()
	}
}

/*
* Waits for pings, ie Reads from UDP socket
 */
func UDPService(ServerConn *net.UDPConn) {
	println("WE MADE IT TO UDP SERVICE")
	buf := make([]byte, 1500)
	for {
		//	println("WE ABOUT TO READ")
		n, addr, err := ServerConn.ReadFromUDP(buf)
		checkError(err)
		//fmt.Println("Received From Server ",string(buf[0:n])," size ",n, " from ",addr)
		go handleUDP(string(buf[0:n]), ServerConn, addr)
	}
}

/*
* write back to server after a ping is received
 */

func handleUDP(recmsg string, Conn *net.UDPConn, addr *net.UDPAddr) {
	//println("WE MADE IT TO HANDLE")
	buf := []byte(RECEIVE_PING_ADDR)
	_, err := Conn.WriteToUDP(buf, addr)
	checkError(err)
	//     println("WE FINISHED WRITING")
	//time.Sleep(time.Second * 1)
}

/*
* listening for RPC calls from the other servers
 */

func systemListenServe(local string, c chan int) {
	ll, ee := net.Listen("tcp", local)
	nodePORT := ll.Addr().(*net.TCPAddr).Port
	c <- nodePORT
	if ee != nil {
		log.Fatal("listen error:", ee)
	}
	for {
		conn, _ := ll.Accept()
		go rpc.ServeConn(conn)
	}
}

/*
* listening for RPC calls from the clients
 */

func clientListenServe(local string, ch chan int) {
	ll, ee := net.Listen("tcp", local)
	nodePORT := ll.Addr().(*net.TCPAddr).Port
	ch <- nodePORT
	if ee != nil {
		log.Fatal("listen error:", ee)
	}
	for {
		conn, _ := ll.Accept()
		go rpc.ServeConn(conn)
	}
}

/*
*  Join storage servers
 */
func joinStorageServers() {
	systemService, err := rpc.Dial("tcp", LOAD_BALANCER_IPPORT)
	checkError(err)

	var reply NodeListReply

	newNodeSetup := NewNodeSetup{
		RPC_CLIENT_IPPORT: RPC_CLIENT_IPPORT,
		RPC_SERVER_IPPORT: RPC_SYSTEM_IPPORT,
		UDP_IPPORT:        RECEIVE_PING_ADDR}

	err = systemService.Call("NodeService.NewNode", newNodeSetup, &reply)
	checkError(err)
	list := reply.ListOfNodes
	fmt.Println("Nodes So Far: ", list.UDP_IPPORT)
	serverListMutex.Lock()
	serverList = list
	serverListMutex.Unlock()

}

/*
* Add A node to our linked list of server nodes
 */
func addNode(udp string, clientRPC string, serverRPC string) {

	serverListMutex.Lock()
	if isNewNode(udp) {

		newNode := &ServerItem{udp, clientRPC, serverRPC, 0, nil}

		if serverList == nil {
			serverList = newNode
		} else {
			newNode.NextServer = serverList
			serverList = newNode
		}
	}

	serverListMutex.Unlock()
	return
}

func isNewNode(ident string) bool {

	next := serverList

	for next != nil {
		if (*next).UDP_IPPORT == ident {
			return false
		}
		next = (*next).NextServer
	}

	return true
}


func sizeOfServerList() (total int){

	next := serverList
	total = 0
	for next != nil {
		total++
		next = (*next).NextServer
	}

	return
}


/*
* Add A node to our linked list of clients
*/
func addClient(username string, rpc string) {

	clientListMutex.Lock()
	if isNewClient(username) {

		newNode := &ClientItem{username, rpc, nil}

		if clientList == nil {
			clientList = newNode
		} else {
			newNode.NextClient = clientList
			clientList = newNode
		}
	}else{
		next := clientList
		for next != nil {
		if (*next).Username == username {
			(*next).RPC_IPPORT = rpc
			break
		}
		next = (*next).NextClient
		}

	}

	clientListMutex.Unlock()
	return
}

func isNewClient(ident string) bool {

	next := clientList

	for next != nil {
		if (*next).Username == ident {
			return false
		}
		next = (*next).NextClient
	}

	return true
}

