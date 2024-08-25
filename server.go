package main

import (
	"log"
	"net"
	"os"
)

func main() {
	mainlog := log.New(os.Stdout, "server:", log.LstdFlags)
	var dbConn DatabaseConnection

	mainlog.Print("Reading database parameters...")
	if err := getDatabaseParams(&dbConn); err != nil {
		mainlog.Println("Failed")
		mainlog.Printf("Unable to connect to database: %v\n", err)
		return
	}
	mainlog.Println("Sucsess")
	mainlog.Print("Attempting to connect to database...")

	err := dbConn.OpenConnection()
	if err != nil {
		mainlog.Println("Failed")
		mainlog.Printf("Unable to connect to database: %v\n", err)
		return
	}
	mainlog.Println("Sucsess")
	defer dbConn.CloseConnection()

	mainlog.Print("Attempting to start server...")

	tcpAddr, listener, err := createTcpServerConnection("3241")
	if err != nil {
		mainlog.Println("Failed")
		mainlog.Printf("Unable to create tcp server: %v\n", err)
		return
	}
	mainlog.Println("Sucsess")
	mainlog.Printf("Server is online on %v\n", tcpAddr)
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go HandleClient(dbConn, tcpConn)
	}

}

func getDatabaseParams(dbConn *DatabaseConnection) error {
	params, err := ReadParamsFromFile("DatabaseParams.txt")

	if err != nil {
		return err
	}
	dbConn.SetBaseName(params[0])
	dbConn.SetBaseHost(params[1])
	dbConn.SetUserName(params[2])
	dbConn.SetPassword(params[3])
	return nil
}

func createTcpServerConnection(port string) (*net.TCPAddr, *net.TCPListener, error) {
	service := "localhost:" + port
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, nil, err
	}
	return tcpAddr, listener, nil
}
