package main

import (
	"flag"
	"log"

	"github.com/OmarTariq612/tftp-server/server"
)

func main() {
	host := flag.String("host", "", "socks server host")
	port := flag.Int("port", 69, "socks server port")
	file := flag.String("file", "", "the file shared")
	flag.Parse()
	s := server.NewTFTPServer(*host, *port, *file)
	err := s.ListenAndServe()
	if err != nil {
		log.Println(err)
	}
}
