package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
)

var hLog *log.Logger

// handler for every client request
func HandleClient(dbConn DatabaseConnection, tcpConn net.Conn, stopServer chan bool) {
	defer tcpConn.Close()

	hLog = log.New(os.Stderr, "handler:", log.LstdFlags)

	hLog.Println("Client reached server")

	var rcvMsg [1024]byte

	_, err := tcpConn.Read(rcvMsg[0:])
	if err != nil {
		hLog.Println("Failed to recive message from client")
		return
	}
	if !dbConn.connected {
		tcpConn.Write([]byte{0xF0, 0x01}) // last byte - error always error code
		return                            // 01 - database connection failure
	}
	switch rcvMsg[0] {
	case 0xAA:
		err = handleHelloRequest(tcpConn)
	case 0x10:
		err = handleCreateUserRequest(rcvMsg[1:], dbConn, tcpConn)
	case 0x20:
		err = handleAuthentificationRequest(rcvMsg[1:], dbConn, tcpConn)
	case 0x21:
		err = handleLogoutRequest(rcvMsg[1:], dbConn, tcpConn)
	case 0x01:
		err = handleShutDownServer(rcvMsg[1:], dbConn, stopServer)
	default:
		hLog.Printf("Unknown command: %v\n", rcvMsg[0])
	}

	if err != nil {
		hLog.Printf("Failed to handle request: %v\n", err)
		return
	}
}

// request to test connection, replies to client with OK byte
func handleHelloRequest(tcpConn net.Conn) error {
	_, err := tcpConn.Write([]byte{0x0F, 0x00}) // 0 - no error
	if err != nil {
		return err
	}
	return nil
}

// request to create new user, replies to client with OK byte
func handleCreateUserRequest(request []byte, dbConn DatabaseConnection, tcpConn net.Conn) error {
	var (
		roleID         int
		userNameLength uint8
		pwdHashLength  uint8
		newUserName    string
		newPwdHash     []byte
	)
	token := binary.LittleEndian.Uint32(request[0:4])
	role, err := dbConn.getRole(int(token))
	if err != nil {
		tcpConn.Write([]byte{0xF0, 0x02}) //  02 - request handling error
		return err
	}
	if role != "SUPERUSER" {
		tcpConn.Write([]byte{0xF0, 0x02}) //  02 - request handling error
		return errors.New("not enough rights to execute command")
	}
	switch request[4] {
	case 0x11:
		roleID = 1
	case 0x12:
		roleID = 2
	case 0x13:
		roleID = 3
	default:
		tcpConn.Write([]byte{0xF0, 0x02}) //  02 - request handling error
		return errors.New("unknown role recived")
	}

	userNameLength = request[5]
	newUserName = string(request[6 : userNameLength+6])
	pwdHashLength = request[userNameLength+6]
	newPwdHash = request[userNameLength+6 : pwdHashLength+userNameLength+6]

	err = dbConn.insertNewUser(newUserName, newPwdHash, roleID)

	if err != nil {
		tcpConn.Write([]byte{0xF0, 0x02}) //  02 - request handling error
		return err
	}

	_, err = tcpConn.Write([]byte{0x0F, 0x00}) // 0 - no error
	if err != nil {
		return err
	}

	return nil
}

// request to login by user, replies with OK flag and generated token
func handleAuthentificationRequest(request []byte, dbConn DatabaseConnection, tcpConn net.Conn) error {
	var (
		userNameLength uint8
		pwdHashLength  uint8
		loginUserName  string
		loginPwdHash   []byte
	)

	userNameLength = request[0]
	loginUserName = string(request[1 : userNameLength+1])
	pwdHashLength = request[userNameLength+1]
	loginPwdHash = request[userNameLength+1 : pwdHashLength+userNameLength+1]

	userRoleId, userToken, err := dbConn.userAuthentification(loginUserName, loginPwdHash)
	if err != nil {
		tcpConn.Write([]byte{0xF0, 0x02})
		return err
	}
	var answer []byte
	token := make([]byte, 4)
	binary.LittleEndian.PutUint32(token, uint32(userToken))
	answer = append(answer, 0x0F, 0x00, uint8(userRoleId))
	answer = append(answer, token...)

	_, err = tcpConn.Write(answer)
	if err != nil {
		return err
	}

	return nil
}

// request to logout
func handleLogoutRequest(request []byte, dbConn DatabaseConnection, tcpConn net.Conn) error {
	token := request[:4]
	err := dbConn.removeConnection(binary.LittleEndian.Uint32(token))
	if err != nil {
		_, err = tcpConn.Write([]byte{0xF0, 0x02})
		return err
	}

	_, err = tcpConn.Write([]byte{0x0F, 0x00})
	if err != nil {
		return err
	}

	return nil
}

// request to shut down server
func handleShutDownServer(request []byte, dbConn DatabaseConnection, stopServer chan<- bool) error {
	token := binary.LittleEndian.Uint32(request[0:4])
	role, err := dbConn.getRole(int(token))
	if err != nil {
		return err
	}
	if role != "SUPERUSER" {
		return errors.New("not enough rights to execute command")
	}

	stopServer <- true
	return nil
}
