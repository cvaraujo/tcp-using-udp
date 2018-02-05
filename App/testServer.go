package main

import (
	"fmt"
	"os"
	"../Protocol"
	"io/ioutil"
	"strconv"
)

var DIRECTORY string
/* Function to select the name of directory to save the files */
func createDirectory(directory string) {
	err := os.Mkdir(directory, 0777)
	if os.IsExist(err) {
		op := 0
		fmt.Println("Directory already exists")
		fmt.Println("1. Change directory name")
		fmt.Println("2. Replace")
		fmt.Scanf("%d", &op)
		if op == 1 {
			fmt.Scanln(DIRECTORY)
			createDirectory(DIRECTORY)
		} else if op == 2 {
			fmt.Println("Substituted!")
		} else {
			fmt.Println("Invalid option!")
			os.Exit(1)
		}
	} else {
		fmt.Println("Directory successfully created!")
	}
}

func writeClientFile(conn *Protocolo.Conn) {
	fmt.Println("A new connection accept!")
	var total []byte
	msg := make([]byte, 524)
	gaveError := false
	for !conn.Finished() {
		n, err := conn.Read(msg)
		if err != nil && n == 0{
			gaveError = true
			fmt.Println(err)
			break
		} else {
			total = append(total, msg[:n]...)
		}
	}
	if gaveError {
		fmt.Println("Error receiving file!")
		ioutil.WriteFile(DIRECTORY+"/"+strconv.Itoa(int(conn.Id))+"ERROR", total, 0777)
	} else {
		ioutil.WriteFile(DIRECTORY+"/"+strconv.Itoa(int(conn.Id))+".FILE", total, 0777)
		fmt.Println("Writed!", DIRECTORY)
	}
}
func main() {
	fmt.Println("Enter the Port")
	PORT := "10002"
	fmt.Scanln(&PORT)
	fmt.Println("Enter the Directory")
	DIRECTORY = "save"
	fmt.Scanln(&DIRECTORY)
	if DIRECTORY == "" {
		fmt.Println("You can not leave empty values!\n Try 'go run archive.go PORT DIRECTORY'\n")
		os.Exit(1)
	}

	conn, err := Protocolo.Listen("localhost", PORT)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer conn.Close()

	createDirectory(DIRECTORY)

	for {
		c, err := conn.Accept()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		go writeClientFile(c)
	}
}
