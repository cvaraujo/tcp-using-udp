package main

import (
	"../Protocol"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	fmt.Println("Enter the Server address")
	ADDR := "localhost"
	//fmt.Scanln(&ADDR)
	fmt.Println("Enter the Port")
	PORT := "10001"
	//fmt.Scanln(&PORT)
	fmt.Println("Enter the Archive name")
	FILENAME := "f.mp3"
	//fmt.Scanln(&FILENAME)
	//fmt.Scanln(&FILENAME)
	if ADDR == "" || PORT == "" || FILENAME == "" {
		fmt.Println("Inválid arguments!\n Try 'go run client.go ServerAddress Port'")
	}

	conn, err := Protocolo.DialTCP(ADDR, PORT)
	if err != nil {
		fmt.Println(err)
	}

	file, err := ioutil.ReadFile(FILENAME)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Starting send!")
	_, err = conn.Write(file)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Finished!")
}
