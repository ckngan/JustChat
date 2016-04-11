// Author: Haniel Martino
package clientutil

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

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

// FileData to build file structure in rpc call
type FileData struct {
	Username string
	FileName string
	FileSize int64
	Data     []byte
}

// Method to get client's username
func GetClientUsername() string {
	// Reading input from user for username
	uname := ""
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(EditText("Please enter your username:", Blue_B, Intensity_1), " ")
		inputUsername, _ := reader.ReadString('\n')
		uname = inputUsername

		if len(uname) > 0 {
			break
		} else {
			fmt.Println(EditText("Must enter 1 or more characters\n", Red, Intensity_1))
		}
	}
	return RemoveNewLine(uname)
}

// Method to get client's username
func GetClientPassword() string {
	// Reading input from user for username
	pword := ""
	for {
		fmt.Print(EditText("Please enter your password:", Blue_B, Intensity_1), " ")
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
			fmt.Println(EditText("Must enter 3 or more characters\n", Red, Intensity_1))
		}
	}
	return RemoveNewLine(pword)
}

/* Helper method to read file from disk and package it into a FileData struct
 */
func PackageFile(username string, path string) (fileData FileData, err error) {
	//var fileData FileData

	_, file := filepath.Split(path)

	r, err := os.Open(path)
	if err != nil {
		fileData.Username = ""
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

// get ip address of the host for client's server
func GetIP() (ip string) {

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

func RemoveNewLine(value string) (str string) {
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
func EditText(text string, color int, intensity int) string {
	return "\x1b[" + strconv.Itoa(color) + ";" +
		strconv.Itoa(intensity) + "m" +
		text + "\x1b[0m"
}

func IncorrectCommand() {
	fmt.Println("Incorrect command!!!!!")
	MessageCommands()
}

// method to print the commands users can use
func MessageCommands() {

	color1 := Red_B
	color2 := Yellow

	// 3 strings (message in first)
	privateMessage1 := EditText("Private Message", color1, Intensity_1)
	message1 := EditText("#message #username #some_message", color2, Intensity_2)

	// 3 strings (share in first)
	privateMessage2 := EditText("Private File", color1, Intensity_1)
	message2 := EditText("#share #username #/path/to/file/name", color2, Intensity_2)

	// 2 strings (share in first)
	publicMessage1 := EditText("Public File", color1, Intensity_1)
	message3 := EditText("#share #/path/to/file/name", color2, Intensity_2)

	// 1 string (share in first)
	commands := EditText("Commands", color1, Intensity_1)
	message4 := EditText("#commands", color2, Intensity_2)

	files := EditText("Available Files", color1, Intensity_1)
	message5 := EditText("#available files", color2, Intensity_2)

	getFile := EditText("Get File", color1, Intensity_1)
	message6 := EditText("#get #filename", color2, Intensity_2)

	fmt.Printf(EditText("<----------------------- JustChat Commands "+
		"----------------------->\n", color2, Intensity_1))
	fmt.Printf("%s ==> %2s\n", privateMessage1, message1)
	fmt.Printf("   %s ==> %2s\n", privateMessage2, message2)
	fmt.Printf("    %s ==> %2s\n", publicMessage1, message3)
	fmt.Printf("%s ==> %2s\n", files, message5)
	fmt.Printf("       %s ==> %2s\n", getFile, message6)
	fmt.Printf("       %s ==> %2s\n", commands, message4)
	fmt.Printf(EditText("<--------------------------------"+
		"--------------------------------->\n\n", color2, Intensity_1))
	fmt.Printf(EditText("<--------------------------Start"+
		" Chatting------------------------->\n", Magenta, Intensity_1))

}
