package server

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"
)

type TFTPServer struct {
	address string
	payload []byte
	retries uint8
	timeout time.Duration
}

func NewTFTPServer(host string, port int, file string) *TFTPServer {
	p, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return &TFTPServer{address: net.JoinHostPort(host, strconv.Itoa(port)), payload: p, retries: 10, timeout: 5 * time.Second}
}

const (
	DatagramSize = 516
	BlockSize    = DatagramSize - 4 // DatagramSize - 4-byte tftp header
)

func (s *TFTPServer) ListenAndServe() error {
	listener, err := net.ListenPacket("udp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Printf("Listening on: %v", listener.LocalAddr())

	var rwRequest ReadWriteRequest

	for {
		var buf [DatagramSize]byte
		_, senderAddr, err := listener.ReadFrom(buf[:])
		if err != nil {
			return err
		}

		err = rwRequest.UnmarshalBinary(buf[:])
		if err != nil {
			listener.WriteTo([]byte{byte(ErrorOp), byte(ErrIllegalOp), 0}, senderAddr)
			log.Printf("invalid request from %v: %v", senderAddr, err)
			continue
		}

		go s.handle(senderAddr, rwRequest)
	}

}

func (s *TFTPServer) handle(clientAddr net.Addr, request ReadWriteRequest) {
	log.Printf("[%s] requested file: %s\n", clientAddr.String(), request.Filename)

	conn, err := net.Dial("udp", clientAddr.String())
	if err != nil {
		log.Printf("[%s] dial: %v\n", clientAddr.String(), err)
		return
	}
	defer conn.Close()

	var (
		code  Opcode
		ackM  Acknowledgment
		errM  Err
		dataM = Data{Payload: bytes.NewReader(s.payload)}
		buf   = make([]byte, DatagramSize) // for replies (Ack / Error) from the client
	)

	n := DatagramSize

NEXT_PACKET:
	for n == DatagramSize {
		data, err := dataM.MarshalBinary()
		if err != nil {
			log.Printf("[%s] preparing data packet: %v", clientAddr.String(), err)
			return
		}

	RETRIES:
		for i := 0; i < int(s.retries); i++ {
			n, err = conn.Write(data)
			if err != nil {
				log.Printf("[%s] write: %v", clientAddr.String(), err)
				return
			}

			conn.SetReadDeadline(time.Now().Add(s.timeout))
			_, err = conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue RETRIES
				}
				log.Printf("[%s] waiting for ACK: %v", clientAddr.String(), err)
				return
			}

			code = Opcode(binary.BigEndian.Uint16(buf[:2]))

			switch code {
			case AcknowledgmentOp:
				err = ackM.UnmarshalBinary(buf)
				if err != nil {
					continue RETRIES
				}
				if ackM.BlockNum == dataM.BlockNum {
					continue NEXT_PACKET
				}
			case ErrorOp:
				err = errM.UnmarshalBinary(buf)
				if err != nil {
					continue RETRIES
				}
				log.Printf("[%s] received error: %v", clientAddr.String(), err)
				return
			default:
				log.Printf("[%s] bad packet", clientAddr.String())
			}
		}

		// execution comes here only when we exhauste retries
		log.Printf("[%s] exhausted retries", clientAddr.String())
		return
	}

	// well done ... the file has been sent successfully
	log.Printf("[%s] sent %d blocks", clientAddr.String(), dataM.BlockNum)
}
