package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnection struct {
	pool      *pgxpool.Pool
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
	dbConn.pool, err = pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return err
	}

	if err := dbConn.pool.Ping(context.Background()); err != nil {
		return err
	}
	dbConn.connected = true
	return nil
}

func (dbConn *DatabaseConnection) CloseConnection() {
	DBlog := log.New(os.Stdout, "DB:", log.LstdFlags)
	dbConn.pool.Close()
	DBlog.Println("Connection Closed")
}

func (dbConn *DatabaseConnection) insertNewUser(newUserName string, newPassword []byte, role int) error {
	rows, err := dbConn.pool.Query(context.Background(), "select \"Users_username\" from public.\"Users\"")
	if err != nil {
		return err
	}
	defer rows.Close()
	var rowUserName string
	for rows.Next() {
		rows.Scan(&rowUserName)
		if rowUserName == newUserName {
			return errors.New("username already exists in database")
		}
	}
	err = rows.Err()
	if err != nil {
		return err
	}
	rows.Close()

	_, err = dbConn.pool.Exec(context.Background(),
		"Insert into public.\"Users\" (\"Users_username\", \"Users_pswdmd5\", \"Users_roleId\") values ($1,$2,$3)",
		newUserName, newPassword, role)
	if err != nil {
		return err
	}

	return nil
}

func (dbConn *DatabaseConnection) userAuthentification(loginUserName string, loginPassword []byte) (int, int32, error) {
	rows, err := dbConn.pool.Query(context.Background(), "select \"Users_username\", \"Users_pswdmd5\", \"Users_roleId\" from public.\"Users\"")
	if err != nil {
		return -1, -1, err
	}
	defer rows.Close()
	var (
		rowUserName string
		rowPwdHash  []byte
		rowRoleId   int
	)
	for rows.Next() {
		rows.Scan(&rowUserName, &rowPwdHash, &rowRoleId)
		if rowUserName == loginUserName {
			break
			// user with this username exists
		}
	}
	err = rows.Err()
	if err != nil {
		return -1, -1, err
	}
	rows.Close()

	if rowUserName == "" {
		return -1, -1, errors.New("no such username found")
	}

	if !bytes.Equal(loginPassword, rowPwdHash) {
		return -1, -1, errors.New("incorrect password")
	}

	userToken, err := dbConn.generateToken()
	if err != nil {
		return -1, -1, errors.New("failed to generate token for user")
	}

	return rowRoleId, userToken, nil
}

func (dbConn *DatabaseConnection) generateToken() (int32, error) {
	for {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		tokenInt := r.Int31()
		var foundToken int32

		err := dbConn.pool.QueryRow(context.Background(), "select \"Connection_token\" from public.\"Connections\" where \"Connection_token\" = $1", tokenInt).Scan(&foundToken)
		if err != nil {
			if err == pgx.ErrNoRows {
				return tokenInt, nil
			}
			return 0, err
		}
	}
}
