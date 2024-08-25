package main

import (
	"errors"
	"log"
	"net"
	"os"
)

var hLog *log.Logger

func HandleClient(dbConn DatabaseConnection, tcpConn net.Conn) {
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
	}

	if err != nil {
		hLog.Printf("Failed to handle request: %v\n", err)
		tcpConn.Write([]byte{0xF0, 0x02}) // 02 - request handling error
		return
	}
}

func handleHelloRequest(tcpConn net.Conn) error {
	_, err := tcpConn.Write([]byte{0x0F, 0x00}) // 0 - no error
	if err != nil {
		return err
	}
	return nil
}

func handleCreateUserRequest(request []byte, dbConn DatabaseConnection, tcpConn net.Conn) error {
	var (
		roleID         int
		userNameLength uint8
		pwdHashLength  uint8
		newUserName    string
		newPwdHash     []byte
	)
	switch request[0] {
	case 0x11:
		roleID = 1
	case 0x12:
		roleID = 2
	case 0x13:
		roleID = 3
	default:
		// unknown role recived
		return errors.New("unknown role recived")
	}
	userNameLength = request[1]
	newUserName = string(request[2 : userNameLength+2])
	pwdHashLength = request[userNameLength+2]
	newPwdHash = request[userNameLength+2 : pwdHashLength+userNameLength+2]

	err := dbConn.insertNewUser(newUserName, newPwdHash, roleID)

	if err != nil {
		return err
	}

	_, err = tcpConn.Write([]byte{0x0F, 0x00}) // 0 - no error
	if err != nil {
		return err
	}
	return nil
}
