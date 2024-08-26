package main

import (
	"log"
	"net"
	"os"
	"time"
)

func main() {
	mainlog := log.New(os.Stdout, "server:", log.LstdFlags)
	var dbConn DatabaseConnection

	mainlog.Print("Reading database parameters...")
	if err := setDatabaseParams(&dbConn); err != nil {
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

	err = dbConn.clearConnectionTable()
	if err != nil {
		mainlog.Println("Failed to clear Connections table")
		return
	}

	mainlog.Print("Attempting to start server...")

	tcpAddr, listener, err := createTcpServerConnection("3241")
	if err != nil {
		mainlog.Println("Failed")
		mainlog.Printf("Unable to create tcp server: %v\n", err)
		return
	}
	mainlog.Println("Sucsess")
	mainlog.Printf("Server is online on %v\n", tcpAddr)

	ticker := time.NewTicker(time.Minute * 5)
	tickerDone := make(chan bool)
	startConnectionsCleanser(&dbConn, ticker, tickerDone)
	defer stopConnectionCleanser(ticker, tickerDone)

	stopServer := make(chan bool)

	for {
		select {
		case <-stopServer:
			return
		default:
		}
		listener.SetDeadline(time.Now().Add(time.Second * 1))
		tcpConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go HandleClient(dbConn, tcpConn, stopServer)
	}
}

// set database params to connect to database
func setDatabaseParams(dbConn *DatabaseConnection) error {
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

// create tcp connection to which clients can connect
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

// starts ticker to close expired sessions and remove them from database
func startConnectionsCleanser(dbConn *DatabaseConnection, ticker *time.Ticker, done chan bool) {

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				dbConn.closeExpiredSession()
			}
		}
	}()
}

// stops ticker which closes sessions over time
func stopConnectionCleanser(ticker *time.Ticker, done chan bool) {
	ticker.Stop()
	done <- true
}
