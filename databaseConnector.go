package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type DatabaseConnection struct {
	conn      *pgx.Conn
	connected bool

	baseName string
	baseHost string
	userName string
	password string
}

func (dbConn *DatabaseConnection) SetBaseName(baseName string) {
	dbConn.baseName = baseName
}

func (dbConn *DatabaseConnection) SetBaseHost(baseHost string) {
	dbConn.baseHost = baseHost
}

func (dbConn *DatabaseConnection) SetUserName(userName string) {
	dbConn.userName = userName
}

func (dbConn *DatabaseConnection) SetPassword(password string) {
	dbConn.password = password
}

func (dbConn *DatabaseConnection) OpenConnection() error {
	var err error
	connectionString := "postgresql://" + dbConn.userName + ":" + dbConn.password + "@" + dbConn.baseHost + ":5432/" + dbConn.baseName
	dbConn.conn, err = pgx.Connect(context.Background(), connectionString)
	if err != nil {
		return err
	}

	if err := dbConn.conn.Ping(context.Background()); err != nil {
		return err
	}
	dbConn.connected = true
	return nil
}

func (dbConn *DatabaseConnection) CloseConnection() {
	dbConn.conn.Close(context.Background())
	fmt.Println("Connection Closed")
}
