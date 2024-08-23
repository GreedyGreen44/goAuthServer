package main

import (
	"fmt"
	"os"
)

func main() {
	var dbConn DatabaseConnection

	fmt.Print("Reading database parameters...")
	if err := getDatabaseParams(&dbConn); err != nil {
		fmt.Println("Failed")
		fmt.Println(err)
		return
	}
	fmt.Println("Sucsess")
	fmt.Print("Attempting to start server...")

	err := dbConn.OpenConnection()
	if err != nil {
		fmt.Println("Failed")
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		return
	}
	fmt.Println("Sucsess")
	defer dbConn.CloseConnection()
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
