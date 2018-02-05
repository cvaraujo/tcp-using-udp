package Protocol

import (
	"net"
	"encoding/binary"
	"fmt"
	"time"
	"errors"
)

/************* Variables ****************/
var c clientConnection
var archive []byte
var w window = window{0, 1, 0, 512,
					  1, 10000, 1, 512}
var packages map[int]*pack
var acks map[uint32]int
/************* Structs *******************/
type clientConnection struct {
	idClient uint16
	conn     *net.UDPConn
	ackMsg   chan []byte
	archive  chan []byte
	finished chan bool
}
type window struct {
	firstSend int
	lastSend  int
	eow       int
	expected  uint32
	CWND      int
	SSTHRESH  int
	increase  int
	cwndsize  int
}
type pack struct {
	Data []byte
	Send bool
}

/*******************************************/
func mountClientHeader(typeHeader int, seq uint32, id int) []byte {
	h := make([]byte, 12)
	if typeHeader == 1 {
		binary.LittleEndian.PutUint32(h[:4], seq)
		binary.LittleEndian.PutUint32(h[4:8], 0)
		binary.LittleEndian.PutUint16(h[8:10], uint16(id))
		binary.LittleEndian.PutUint16(h[10:12], 4)
		return h
	} else if typeHeader == 2 {
		binary.LittleEndian.PutUint32(h[:4], seq)
		binary.LittleEndian.PutUint32(h[4:8], 0)
		binary.LittleEndian.PutUint16(h[8:10], uint16(id))
		binary.LittleEndian.PutUint16(h[10:12], 2)
		return h
	} else {
		binary.LittleEndian.PutUint32(h[:4], seq)
		binary.LittleEndian.PutUint32(h[4:8], 0)
		binary.LittleEndian.PutUint16(h[8:10], uint16(id))
		binary.LittleEndian.PutUint16(h[10:12], 1)
		return h
	}
}
func DialTCP(ADDR string, PORT string) (*clientConnection, error) {
	/* Resolve the server address */
	ServerAddr, err := net.ResolveUDPAddr("udp", ADDR+":"+PORT)
	if err != nil {
		return nil, err
	}
	/* Resolve the local address*/
	LocalAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return nil, err
	}
	clientConn, err := net.DialUDP("udp", LocalAddr, ServerAddr)

	msg := mountClientHeader(1, 12345, 0)
	clientConn.Write(msg)
	c = clientConnection{0, clientConn, make(chan []byte),
						 make(chan []byte), make(chan bool)}

	buf := make([]byte, 12)
	for c.idClient == 0 {
		n, _, err := clientConn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		header := convertHeader(buf[:n])
		c.idClient = header.id
	}
	fmt.Println("Connection Successfully made!")

	go listenServer()
	return &c, err
}
func (c *clientConnection) sendMsg() {
	for data := range c.archive {
		if len(data) != 0 {
			_, err := c.conn.Write(data)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
func (c *clientConnection) Write(msg []byte) (int, error) {
	if len(msg) <= 100000000 {
		archive = make([]byte, len(msg))
		copy(archive, msg)
		packages = make(map[int]*pack)
		acks = make(map[uint32]int)
		c.mountPackages()
		/***************/
		go c.sendMsg()
		/******* Starting the select ********/
		c.archive <- packages[0].Data
		packages[0].Send = true
		count := 0
		finished := true
		for finished {
			select {
			case ack := <-c.ackMsg:
				h := convertHeader(ack)
				fmt.Println("Recebido", h.ackNumber, w.expected)
				if h.ack && !h.fin {
					/* Se um ack de um pacote que não é final foi recebido
					Então eu aumento o valor esperando pra a próxima posição do vetor*/
					if h.ackNumber == w.expected {
						count = 0
						w.expected += 512
						/* Verificação para aumentar a janela, se ela é maior que o SSTHRESH ou não */
						if w.cwndsize >= w.SSTHRESH {
							w.cwndsize += (512 * 512) / w.SSTHRESH
							fmt.Println(w.cwndsize, w.CWND, w.SSTHRESH, w.increase)
							if w.cwndsize-w.SSTHRESH >= w.increase*512 {
								w.CWND += 1
								w.cwndsize += 512
								w.increase += 1
							}
							/* Movimenta o FS pra posição a frente */
							w.firstSend = acks[h.ackNumber+511]
							w.eow = w.firstSend + w.CWND

							if w.firstSend >= w.lastSend {
								begin := w.firstSend
								end := w.eow
								for ; begin < end && begin < len(packages); begin++ {
									if packages[begin].Send != true {
										packages[begin].Send = true
										w.lastSend = begin
										c.archive <- packages[begin].Data
									}
								}
							} else {
								begin := w.lastSend
								end := w.eow
								for ; begin < end && begin < len(packages); begin++ {
									if packages[begin].Send != true {
										packages[begin].Send = true
										w.lastSend = begin
										c.archive <- packages[begin].Data
									}
								}
							}
						} else {
							/* Caso a janela não esteja maior que o SSTHRESH eu aumento normalmente */
							/* Aumenta a janela */
							w.CWND += 1
							/* Movimenta o FS pra posição a frente */
							w.firstSend = acks[h.ackNumber+511]
							/* Marca o limite da janela */
							w.eow = w.firstSend + w.CWND

							if w.firstSend >= w.lastSend {
								begin := w.firstSend
								end := w.eow
								for ; begin < end && begin < len(packages); begin++ {
									if packages[begin].Send != true {
										packages[begin].Send = true
										w.lastSend = begin
										c.archive <- packages[begin].Data
									}
								}
							} else {
								begin := w.lastSend
								end := w.eow
								for ; begin < end && begin < len(packages); begin++ {
									if packages[begin].Send != true {
										packages[begin].Send = true
										w.lastSend = begin
										c.archive <- packages[begin].Data
									}
								}
							}
						}
					} else if h.ackNumber > w.expected {
						/* aumenta o expected para o próximo depois do ack recebido, e o first também */
						for w.expected < h.ackNumber {
							w.expected += 512
						}
						w.firstSend = acks[w.expected-1]

						if w.firstSend >= w.lastSend {
							w.eow += w.CWND
							begin := w.lastSend
							end := w.eow
							for ; begin < end && begin < len(packages); begin++ {
								if packages[begin].Send != true {
									packages[begin].Send = true
									w.lastSend = begin
									c.archive <- packages[begin].Data
								}
							}
						} else {
							received := acks[h.ackNumber-1]
							increase := received - w.firstSend
							w.eow += increase
							begin := w.lastSend
							end := w.eow
							for ; begin < end && begin < len(packages); begin++ {
								if packages[begin].Send != true {
									packages[begin].Send = true
									w.lastSend = begin
									c.archive <- packages[begin].Data
								}
							}
						}
					}
				} else if h.fin {
					finished = false
					break
				}
				/* fim da primeira opção do select */
			case <-time.After(500 * time.Millisecond):
				count++
				if count < 20 {
					w.SSTHRESH = w.cwndsize
					w.cwndsize = 512
					w.increase = 1
					w.CWND = 1
					begin := acks[w.expected-1]
					w.firstSend = begin
					w.lastSend = begin + 30
					end := w.lastSend
					for ; begin < end && begin < len(packages); begin++ {
						w.lastSend = begin
						c.archive <- packages[begin].Data
					}
					break
				} else {
					finished = false
					return 0, errors.New("Error to access server!")
				}
			}
		}
		//fmt.Println(w.CWND)
		return len(msg), nil
	} else {
		return 0, errors.New("File too large!")
	}
}
func (c *clientConnection) mountPackages() {
	i := 0
	k := 512
	j := 0
	for {
		if k > len(archive) {
			h := mountClientHeader(3, uint32(len(archive)-1), int(c.idClient))
			p := pack{append(h, archive[i:]...), false}
			packages[j] = &p
			acks[uint32(k-1)] = j
			break
		} else {
			h := mountClientHeader(2, uint32(k-1), int(c.idClient))
			p := pack{append(h, archive[i:k]...), false}
			packages[j] = &p
			acks[uint32(k-1)] = j
			i = k
			k += 512
			j++
		}
	}
}
func (c *clientConnection) Read(msg []byte) (int, error) {
	for {
		n, _, err := c.conn.ReadFromUDP(msg)
		if n != 0 {
			return n, err
		}
	}
}
func (c *clientConnection) Close() error {
	err := c.conn.Close()
	return err
}
func listenServer() {
	msg := make([]byte, 12)
	for {
		n, _, err := c.conn.ReadFromUDP(msg)
		if err != nil {
			fmt.Println(err)
		}
		c.ackMsg <- msg[:n]
	}
}
