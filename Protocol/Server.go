package Protocol

import (
	"sync"
	"fmt"
	"encoding/binary"
	"net"
	"time"
	"errors"
)

/****************** Variables *******************/
var idSender uint16 = 1
var connections map[string]*Conn
var mutex = &sync.RWMutex{}
var serverConnection *net.UDPConn
var handConn chan newConn = make(chan newConn)
/************************************************/
/***************** Struct ***********************/
type Conn struct {
	Id                uint16
	ackResp           chan []byte
	channel           chan []byte
	Timeout           chan bool
	serverConnection  *net.UDPConn
	clientAddr        *net.UDPAddr
	seqNumberExpected uint32
	lastReceived      []byte
	finished          bool
}
type Header struct {
	seqNumber uint32
	ackNumber uint32
	id        uint16
	syn       bool
	ack       bool
	fin       bool
}
type newConn struct {
	addr *net.UDPAddr
	data []byte
}

/* Function to convert the header that is in bytes and return the structure */
func convertHeader(header []byte) Header {
	seqNumber := binary.LittleEndian.Uint32(header[:4])
	ackNumber := binary.LittleEndian.Uint32(header[4:8])
	idConnection := binary.LittleEndian.Uint16(header[8:10])
	flag := binary.LittleEndian.Uint16(header[10:12])
	h := Header{seqNumber, ackNumber, idConnection, false, false, false}

	if flag == 4 {
		h.syn = true
	} else if flag == 2 {
		h.ack = true
	} else if flag == 1 {
		h.fin = true
	} else if flag == 6 {
		h.ack = true
		h.syn = true
	} else if flag == 3 {
		h.ack = true
		h.fin = true
	}
	return h
}

/* Function to mount the new header */
func mountHeader(typeHeader int, header Header) []byte {
	if typeHeader == 1 {
		h := make([]byte, 12)
		binary.LittleEndian.PutUint32(h[:4], header.ackNumber)
		binary.LittleEndian.PutUint32(h[4:8], header.seqNumber+1)
		binary.LittleEndian.PutUint16(h[8:10], header.id)
		binary.LittleEndian.PutUint16(h[10:12], 6)
		return h
	} else if typeHeader == 2 {
		h := make([]byte, 12)
		binary.LittleEndian.PutUint32(h[:4], header.ackNumber)
		binary.LittleEndian.PutUint32(h[4:8], header.seqNumber+1)
		binary.LittleEndian.PutUint16(h[8:10], header.id)
		binary.LittleEndian.PutUint16(h[10:12], 2)
		return h
	} else {
		h := make([]byte, 12)
		binary.LittleEndian.PutUint32(h[:4], header.ackNumber)
		binary.LittleEndian.PutUint32(h[4:8], header.seqNumber+1)
		binary.LittleEndian.PutUint16(h[8:10], header.id)
		binary.LittleEndian.PutUint16(h[10:12], 3)
		return h
	}
}

/* Function to start the server */
func Listen(HOST string, PORT string) (*Conn, error) {
	/* Checks to start the server */
	if HOST == "" || PORT == "" {
		return nil, errors.New("You can not leave empty values!\n Try 'go run archive.go PORT DIRECTORY'\n")
	}
	ServerAddr, err := net.ResolveUDPAddr("udp", HOST+":"+PORT)
	if err != nil {
		return nil, err
	}
	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	/* Initializing the connections map */
	connections = make(map[string]*Conn)
	serverConnection = ServerConn

	conn := Conn{0, make(chan []byte), make(chan []byte),
				 make(chan bool), serverConnection,
				 nil, 511, make([]byte, 12), false}
	go read()
	return &conn, err
}
func (c *Conn) Close() {
	c.serverConnection.Close()
}
func (c *Conn) Accept() (*Conn, error) {
	for pack := range handConn {
		connection := Conn{idSender, make(chan []byte), make(chan []byte),
						   make(chan bool), serverConnection, pack.addr,
						   511, make([]byte, 12), false}
		mutex.Lock()
		connections[pack.addr.String()] = &connection
		mutex.Unlock()
		data := mountHeader(1, Header{0, 0, idSender,
									  false, false, false })
		idSender++
		connection.seqNumberExpected = 511
		connection.finished = false
		_, err := connection.serverConnection.WriteToUDP(data, pack.addr)
		if err != nil {
			fmt.Println(err)
		}

		go connection.ackResponse()
		go connection.timeout()
		return &connection, err
	}
	return &Conn{}, nil
}
func (c *Conn) Write(msg []byte) (int, error) {
	n, err := c.serverConnection.WriteTo(msg, c.clientAddr)
	return n, err
}
func (c *Conn) timeout() {
	stop := false
	for !stop {
		select {
		case <-time.After(10 * time.Second):
			fmt.Println("Timeout!", c.clientAddr)
			c.channel <- []byte("error")
			c.finished = false
			stop = true
			delete(connections, c.clientAddr.String())
			break
		case t := <-c.Timeout:
			if !t {
				stop = true
				break
			}
		}
	}
}

/* Trata dados lidos pelo servidor*/
func (c *Conn) Read(msg []byte) (int, error) {
	for data := range c.channel {
		if string(data[:5]) == "error" {
			return 0, errors.New("Error")
		}
		h := convertHeader(data[:12])
		if h.ack {
			if h.seqNumber == c.seqNumberExpected {
				c.seqNumberExpected += 512
				copy(msg, data[12:])
				lastHeader := mountHeader(2, h)
				copy(c.lastReceived, lastHeader)
				c.ackResp <- lastHeader
				c.Timeout <- true
				return len(data) - 12, nil
			} else {
				c.ackResp <- c.lastReceived
				return 0, nil
			}
		} else if h.fin {
			if c.finished == false {
				c.finished = true
				c.seqNumberExpected -= uint32(512 - len(data))
				copy(msg, data[12:])
				copy(c.lastReceived, data[:12])
				header := convertHeader(c.lastReceived)
				byteHeader := mountHeader(3, header)
				c.ackResp <- byteHeader
				c.Timeout <- false
				delete(connections, c.clientAddr.String())
				return len(data) - 12, nil
			} else {
				c.ackResp <- c.lastReceived
				return 0, nil
			}
		}
	}
	return 0, nil
}
func (c *Conn) ackResponse() {
	for data := range c.ackResp {
		//fmt.Println("Enviando", convertHeader(data))
		//ack := rand.Int()
		//if ack%10 == 0 {
		//	fmt.Println("Perdi")
		//} else {
		//fmt.Println(convertHeader(data))
		_, err := c.serverConnection.WriteToUDP(data, c.clientAddr)
		if err != nil {
			fmt.Println(err)
		}
		//}
	}
}
func (c *Conn) Finished() bool {
	return c.finished
}
func read() {
	msg := make([]byte, 524)
	for {
		n, addr, _ := serverConnection.ReadFromUDP(msg)
		h := convertHeader(msg[:12])
		if h.syn && h.ackNumber == 0 && h.id == 0 {
			data := make([]byte, n)
			copy(data, msg[:n])
			pack := newConn{addr, msg}
			handConn <- pack
		} else if h.ackNumber == 0 {
			data := make([]byte, n)
			copy(data, msg[:n])
			//if rand.Int()%23 == 0 {
			//} else {
			if len(data) != 0 {
				mutex.Lock()
				if connections[addr.String()] != nil {
					connections[addr.String()].channel <- data
				}
				mutex.Unlock()
				//	}
			}
		}
	}
}
