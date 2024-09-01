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

// struct for operations with main database
type DatabaseConnection struct {
	pool      *pgxpool.Pool
	connected bool

	baseName string
	baseHost string
	userName string
	password string
}

// sets database name
func (dbConn *DatabaseConnection) SetBaseName(baseName string) {
	dbConn.baseName = baseName
}

// sets database host address
func (dbConn *DatabaseConnection) SetBaseHost(baseHost string) {
	dbConn.baseHost = baseHost
}

// sets username to connect to database
func (dbConn *DatabaseConnection) SetUserName(userName string) {
	dbConn.userName = userName
}

// sets password to connect to database
func (dbConn *DatabaseConnection) SetPassword(password string) {
	dbConn.password = password
}

// opens connection to database and creates connection pool
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

// closes connection to database
func (dbConn *DatabaseConnection) CloseConnection() {
	DBlog := log.New(os.Stdout, "DB:", log.LstdFlags)
	dbConn.pool.Close()
	DBlog.Println("Connection Closed")
}

// inserts new user to Users table with given name, password (md5 hash) and role (roles are defined in Roles table)
func (dbConn *DatabaseConnection) insertNewUser(newUserName string, newPassword []byte, role int) error {
	rows, err := dbConn.pool.Query(context.Background(), `select "Users_username" from public."Users"`)
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
		`Insert into public."Users" ("Users_username", "Users_pswdmd5", "Users_roleId") values ($1,$2,$3)`,
		newUserName, newPassword, role)
	if err != nil {
		return err
	}

	return nil
}

// checks users authentication factors, returns his role id, according to Users table and current session token
func (dbConn *DatabaseConnection) userAuthentication(loginUserName string, loginPassword []byte) (int, int32, error) {
	rows, err := dbConn.pool.Query(context.Background(), `select "Users_username", "Users_pswdmd5", "Users_roleId" 
																from public."Users"`)
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

	alreadyConnected, err := dbConn.checkConnection(loginUserName)
	if err != nil {
		return -1, -1, errors.New("failed to check existing connection")
	}

	if alreadyConnected {
		return -1, -1, errors.New("user is already connected")
	}

	userToken, err := dbConn.generateToken()
	if err != nil {
		return -1, -1, errors.New("failed to generate token for user")
	}

	err = dbConn.saveToken(loginUserName, userToken)
	if err != nil {
		return -1, -1, errors.New("failed to save token for user")
	}

	return rowRoleId, userToken, nil
}

// generates token for current session
func (dbConn *DatabaseConnection) generateToken() (int32, error) {
	for {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		tokenInt := r.Int31()
		var foundToken int32

		err := dbConn.pool.QueryRow(context.Background(), `select "Connection_token" 
																from public."Connections" 
																where "Connection_token" = $1`,
			tokenInt).Scan(&foundToken)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return tokenInt, nil
			}
			return 0, err
		}
	}
}

// creates record in Connections table, where userId, connection start time, connection expires time and current token are saved
func (dbConn *DatabaseConnection) saveToken(user string, token int32) error {
	tx, err := dbConn.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	var userId int
	err = tx.QueryRow(context.Background(), `select "Users_id" 
													from public."Users" 
													where "Users_username" = $1`,
		user).Scan(&userId)
	if err != nil {
		tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("failed to find user among registered Users")
		}
		return err
	}

	_, err = tx.Exec(context.Background(),
		`insert into public."Connections" ("Connection_userId", "Connection_dt", "Connection_expires", "Connection_token") 
				values ($1,$2,$3,$4)`,
		userId, time.Now(), time.Now().Add(time.Minute*15), token)
	if err != nil {
		tx.Rollback(context.Background())
		return err
	}
	err = tx.Commit(context.Background())
	if err != nil {
		return err
	}

	return nil
}

// checks if user is already connected to server
func (dbConn *DatabaseConnection) checkConnection(user string) (bool, error) {
	tx, err := dbConn.pool.Begin(context.Background())
	if err != nil {
		return false, err
	}
	var userId int
	err = tx.QueryRow(context.Background(), `select "Users_id" 
													from public."Users" 
													where "Users_username" = $1`,
		user).Scan(&userId)
	if err != nil {
		tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			return false, errors.New("failed to find user among registered Users")
		}
		return false, err
	}

	err = tx.QueryRow(context.Background(),
		`select "Connection_userId" from public."Connections" where "Connection_userId" = $1`,
		userId).Scan(&userId)
	if err != nil {
		tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	err = tx.Commit(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

// clears Connections table. Executes everytime server starts
func (dbConn *DatabaseConnection) clearConnectionTable() error {
	_, err := dbConn.pool.Exec(context.Background(), `delete from public."Connections"`)
	if err != nil {
		return err
	}
	return nil
}

// removes connection with given token from database
func (dbConn *DatabaseConnection) removeConnectionViaToken(token uint32) error {
	_, err := dbConn.pool.Exec(context.Background(),
		`delete from public."Connections" where "Connection_token" = $1`,
		token)
	if err != nil {
		return err
	}

	return nil
}

func (dbConn *DatabaseConnection) removeConnectionViaUserName(removeUserName string) error {
	_, err := dbConn.pool.Exec(context.Background(), `delete from public."Connections" c 
									using public."Users" u 
									where u."Users_id" = c."Connection_userId" and u."Users_username" = $1`, removeUserName)
	if err != nil {
		return err
	}
	return nil
}

// closes expired sessions, need to be executed over time
func (dbConn *DatabaseConnection) closeExpiredSession() error {
	_, err := dbConn.pool.Exec(context.Background(), `delete from public."Connections" 
       														where "Connection_expires" < $1`,
		time.Now())
	if err != nil {
		return err
	}
	return nil
}

// return role of connected user referring to his token
func (dbConn *DatabaseConnection) getRole(token int) (string, error) {
	var role string
	err := dbConn.pool.QueryRow(context.Background(),
		`select "Roles_name" from public."Roles" r 
    			join public."Users" u on r."Roles_id"=u."Users_id" 
    			join public."Connections" c on c."Connection_userId"=u."Users_id" 
				where c."Connection_token"=$1`,
		token).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("no such token in database")
		}
		return "", err
	}

	return role, nil
}

// remove user with removeUserName as name from database
func (dbConn *DatabaseConnection) removeUser(removeUserName string) error {
	err := dbConn.removeConnectionViaUserName(removeUserName)
	if err != nil {
		return err
	}
	_, err = dbConn.pool.Exec(context.Background(), `delete from public."Users"
															where "Users_username" = $1`,
		removeUserName)
	if err != nil {
		return err
	}
	return nil
}
