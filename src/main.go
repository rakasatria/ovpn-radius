package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
)

var config Config

//code 1
func init() {
	jsonFile, err := os.Open("/etc/openvpn/plugin/config.json")
	if err != nil {
		log.Errorf("init: failed with %s\n", err)
		os.Exit(10)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)

	if err != nil {
		log.Errorf("init: failed with %s\n", err)
		os.Exit(11)
	}

	json.Unmarshal(byteValue, &config)

	if len(config.LogFile) <= 0 {
		log.Errorf("init: config file is null")
		os.Exit(12)
	}

	file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Errorf("init: failed with %s\n", err)
		os.Exit(13)
	}

	log.SetOutput(file)

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, PadLevelText: true})
}

//code 2
func getEnvironment() {
	printEnvExecutable := "/usr/bin/printenv"

	log.Info("getEnvironment: executing " + printEnvExecutable)

	output, err := exec.Command(printEnvExecutable).Output()

	if err != nil {
		log.Errorf("getEnvironment: failed with %s\n", err)
		os.Exit(20)
	}

	outStrs := strings.Split(string(output), "\n")
	if len(outStrs) > 0 {
		for _, outStr := range outStrs {
			if !strings.Contains(strings.ToLower(outStr), "password") || len(outStr) == 0 {
				log.Info("getEnvironment: " + outStr)
			}
		}
	}

	log.Info("getEnvironment: finished. " + printEnvExecutable)
	os.Exit(0)
}

//code 3
func authenticateUser(repository *SQLiteRepository) {
	if len(os.Args) <= 2 {
		log.Errorf("authenticate: 'null' file path.")
		os.Exit(30)
	}

	authFilePath := string(os.Args[2])
	log.Info("authenticate: Autentication using filepath: " + authFilePath)

	authFile, authErr := os.Open(authFilePath)
	if authErr != nil {
		log.Errorf("authenticate: failed with %s\n", authErr)
		os.Exit(31)
	}
	defer authFile.Close()

	readFile, readErr := ioutil.ReadAll(authFile)
	if readErr != nil {
		log.Errorf("authenticate: failed with %s\n", readErr)
		os.Exit(32)
	}

	array := strings.Split(string(readFile), "\n")

	var username = array[0]
	var password = array[1]

	if len(username) <= 0 || len(password) <= 0 {
		log.Errorf("authenticate: unable to authenticate username or password is null")
		os.Exit(33)
	} else {
		log.Info("authenticate: trying to authenticate to " + config.Radius.Authentication.Server)
		authenticationData := "Response-Packet-Type=Access-Accept,NAS-Identifier=" + config.ServerInfo.Identifier + ",NAS-Port-Type=" + config.ServerInfo.PortType + ",NAS-IP-Address=" + config.ServerInfo.IpAddress + ",Service-Type=" + config.ServerInfo.ServiceType + ",Framed-Protocol=1,User-Name=" + username + ",User-Password='" + password + "',Framed-Protocol=PPP,Message-Authenticator=0x00"

		radClientPath := "/usr/bin/radclient"

		cmdArgs := []string{"-x", config.Radius.Authentication.Server, "auth", config.Radius.Authentication.Secret}

		var stdout, stderr bytes.Buffer

		cmd := exec.Command(radClientPath, cmdArgs...)

		cmd.Stdin = bytes.NewBufferString(authenticationData)

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		if err != nil {
			log.Errorf("authenticate: Error: %s", err.Error())
			os.Exit(34)
		}

		outStrs := strings.Split(stdout.String(), "\n")
		//errStrs := strings.Split(stderr.String(), "\n")

		if len(outStrs) <= 0 {
			log.Errorf("authenticate: No Output Received!")
			os.Exit(35)
		}

		var isAutenticated bool

		var className string

		for _, outStr := range outStrs {
			if strings.HasPrefix(outStr, "Received Access-Accept Id") {
				isAutenticated = true
			}

			if strings.HasPrefix(outStr, "\tClass") {
				classArray := strings.Split(outStr, "=")
				untestedClassName := strings.Trim(classArray[1], " ")
				if isValidUTF8FromHex(untestedClassName) {
					className = untestedClassName
				}
			}
		}

		if !isAutenticated {
			log.Errorf("authenticate: failed to authenticate!")
			os.Exit(36)
		}

		log.Info("authenticate: user '" + username + "' with class '" + className + "' is authenticated sucessfully")

		// If AuthenticationOnly is enabled no need to update DB
		if !config.Radius.AuthenticationOnly {
			newClient := OVPNClient{
				Id:         os.Getenv("untrusted_ip") + ":" + os.Getenv("untrusted_port"),
				CommonName: username,
				ClassName:  className,
			}

			_, errCreate := repository.Create(newClient)

			if errCreate != nil {
				log.Errorf("authenticate: failed to save account data with error %s\n", errCreate)
				os.Exit(37)
			}

			log.Info("authenticate: user '" + username + "' with class '" + className + "' data is saved.")
		}

		os.Exit(0)
	}
}

//code 5
func isValidUTF8FromHex(hexaString string) bool {
	numberStr := strings.Replace(strings.ToLower(hexaString), "0x", "", -1)

	decoded, err := hex.DecodeString(numberStr)
	if err != nil {
		log.Errorf("isValidUTF8FromHex: failed with %s\n", err)
		os.Exit(50)
	}

	return utf8.Valid(decoded)
}

//code 6
func accountingRequest(requestType string, repository *SQLiteRepository, sessionId int) {
	log.Info("accountingRequest: prepare send request to " + config.Radius.Accounting.Server + " with request type: " + requestType)
	var accountingCommand string
	userId := os.Getenv("trusted_ip") + ":" + os.Getenv("trusted_port")
	userIpAddress := os.Getenv("ifconfig_pool_remote_ip")

	log.Info("accountingRequest: get user data with Id " + userId)
	userClient, errClient := repository.GetById(userId)
	if errClient != nil {
		log.Errorf("accountingRequest: Error: %s", errClient.Error())
		os.Exit(60)
	}

	if requestType == "start" {
		log.Info("accountingRequest: update user data ip address to " + userIpAddress + " with Id " + userId)
		userClient.IpAddress = userIpAddress
		if _, errClient := repository.Update(*userClient); errClient != nil {
			log.Errorf("accountingRequest: Error: %s", errClient.Error())
			os.Exit(61)
		}
	}

	switch requestType {
	case "start":
		accountingCommand = "Class=" + userClient.ClassName + ",Acct-Session-Id=" + strconv.Itoa(sessionId) + ",Acct-Status-Type=Start,User-Name=" + userClient.CommonName + ",Calling-Station-Id=" + config.ServerInfo.IpAddress + ",NAS-Identifier=" + config.ServerInfo.Identifier + ",Framed-IP-Address=" + userIpAddress
	case "update":
		accountingCommand = "Class=" + userClient.ClassName + ",Acct-Session-Id=" + strconv.Itoa(sessionId) + ",Acct-Status-Type=Interim-Update,User-Name=" + userClient.CommonName + ",Calling-Station-Id=" + config.ServerInfo.IpAddress + ",NAS-Identifier=" + config.ServerInfo.Identifier + ",Framed-IP-Address=" + userIpAddress
	case "stop":
		accountingCommand = "Class=" + userClient.ClassName + ",Acct-Session-Id=" + strconv.Itoa(sessionId) + ",Acct-Status-Type=Stop,User-Name=" + userClient.CommonName + ",Calling-Station-Id=" + config.ServerInfo.IpAddress + ",NAS-Identifier=" + config.ServerInfo.Identifier + ",Framed-IP-Address=" + userIpAddress + ",Acct-Terminate-Cause=User-Request"
	default:
		log.Errorf("accountingRequest: '" + requestType + "' request type is unknown.")
		os.Exit(61)
	}

	log.Info("accountingRequest: sent request to " + config.Radius.Accounting.Server + " with request type: " + requestType)

	radClientPath := "/usr/bin/radclient"

	cmdArgs := []string{"-x", config.Radius.Accounting.Server, "acct", config.Radius.Accounting.Secret}

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(radClientPath, cmdArgs...)

	cmd.Stdin = bytes.NewBufferString(accountingCommand)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		log.Errorf("accountingRequest: error: %s", err.Error())
		os.Exit(62)
	}

	outStrs := strings.Split(stdout.String(), "\n")

	if len(outStrs) <= 0 {
		log.Errorf("accountingRequest: no output received!")
		os.Exit(63)
	}

	var isReceived bool

	for _, outStr := range outStrs {
		if strings.HasPrefix(outStr, "Received Accounting-Response Id") {
			isReceived = true
		}
	}

	if !isReceived {
		log.Errorf("accountingRequest: no Accounting-Response received!")
		os.Exit(64)
	}

	log.Info("accountingRequest: received Accounting-Response from " + config.Radius.Accounting.Server)

	if requestType == "stop" {
		if err := repository.Delete(userClient.Id); err != nil {
			log.Errorf("accountingRequest: unable to delete data %s", err.Error())
			os.Exit(65)
		}

		log.Info("accountingRequest: delete user data with Id " + userId)
	}

	if requestType == "start" {
		accountingRequest("update", repository, sessionId)
	}

	os.Exit(0)
}

func main() {
	user, err := user.Current()
	if err != nil {
		log.Errorf("main: error %s.", err)
		os.Exit(200)
	}

	log.Infof("main: running with username %s", user.Username)

	if len(os.Args) <= 1 {
		log.Errorf("main: 'null' execution type.")
		os.Exit(100)
	}

	repository, err := InitializeDatabase(false)
	if err != nil {
		log.Errorf("main: error %s.", err)
		os.Exit(101)
	}

	executionType := string(os.Args[1])

	switch executionType {
	case "env":
		log.Info("main: running with execution type 'env'")
		getEnvironment()
	case "auth":
		log.Info("main: running with execution type 'auth'")
		authenticateUser(repository)
	case "acct":
		log.Info("main: running with execution type 'acct'")
		sessionId := rand.Intn(9999)
		accountingRequest("start", repository, sessionId)
	case "stop":
		log.Info("main: running with execution type 'stop'")
		sessionId := rand.Intn(9999)
		accountingRequest("stop", repository, sessionId)
	default:
		log.Errorf("main: '" + executionType + "' execution type is unknown.")
		os.Exit(101)
	}
}
