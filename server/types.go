package server

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type Opcode uint16

const (
	ReadOp           Opcode = 1
	WriteOp          Opcode = 2
	DataOp           Opcode = 3
	AcknowledgmentOp Opcode = 4
	ErrorOp          Opcode = 5
)

type ReadWriteRequest struct {
	Filename string
	Mode     string
}

func (r ReadWriteRequest) MarshalBinary() ([]byte, error) {
	var mode string
	if r.Mode != "" {
		mode = r.Mode
	} else {
		mode = "octet"
	}

	buf := new(bytes.Buffer)
	buf.Grow(6 + len(r.Filename) + len(mode)) // 2 (OpCode) + n (len(Filename)) + 1-byte (0) + m (len(mode)) + 1-byte (0)

	err := binary.Write(buf, binary.BigEndian, ReadOp)
	if err != nil {
		return nil, err
	}

	_, err = buf.WriteString(r.Filename)
	if err != nil {
		return nil, err
	}

	err = buf.WriteByte(0)
	if err != nil {
		return nil, err
	}

	_, err = buf.WriteString(mode)
	if err != nil {
		return nil, err
	}

	err = buf.WriteByte(0)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (r *ReadWriteRequest) UnmarshalBinary(buf []byte) error {
	reader := bytes.NewBuffer(buf)
	var code Opcode
	err := binary.Read(reader, binary.BigEndian, &code)
	if err != nil {
		return err
	}
	if code != ReadOp && code != WriteOp {
		return fmt.Errorf("invalid Read/Write request")
	}

	r.Filename, err = reader.ReadString(0)
	if err != nil {
		return fmt.Errorf("invalid Read/Write request")
	}
	r.Filename = strings.TrimRight(r.Filename, "\x00")
	if len(r.Filename) == 0 {
		return fmt.Errorf("invalid Read/Write request")
	}

	r.Mode, err = reader.ReadString(0)
	if err != nil {
		return fmt.Errorf("invalid Read/Write request")
	}
	r.Mode = strings.TrimRight(r.Mode, "\x00")
	r.Mode = strings.ToLower(r.Mode)
	if r.Mode != "octet" {
		return fmt.Errorf("binary (octet) is the only supported transfer")
	}

	return nil
}

type Data struct {
	BlockNum uint16
	Payload  io.Reader
}

func (d *Data) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(DatagramSize)

	err := binary.Write(buf, binary.BigEndian, DataOp)
	if err != nil {
		return nil, err
	}

	d.BlockNum++
	err = binary.Write(buf, binary.BigEndian, d.BlockNum)
	if err != nil {
		return nil, err
	}

	_, err = io.CopyN(buf, d.Payload, BlockSize)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (d *Data) UnmarshalBinary(buf []byte) error {
	var code Opcode
	err := binary.Read(bytes.NewReader(buf[:2]), binary.BigEndian, &code)
	if err != nil {
		return err
	}
	if code != DataOp {
		return fmt.Errorf("invalid Data")
	}

	err = binary.Read(bytes.NewReader(buf[2:4]), binary.BigEndian, &d.BlockNum)
	if err != nil {
		return err
	}

	d.Payload = bytes.NewBuffer(buf[4:])

	return nil
}

type Acknowledgment struct {
	BlockNum uint16
}

func (a Acknowledgment) MarshalBinary() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(4) // 2 (Opcode) + 2 (BlockNum)

	err := binary.Write(b, binary.BigEndian, AcknowledgmentOp)
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, a.BlockNum)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (a *Acknowledgment) UnmarshalBinary(buf []byte) error {
	reader := bytes.NewReader(buf)
	var code Opcode
	err := binary.Read(reader, binary.BigEndian, &code)
	if err != nil {
		return err
	}
	if code != AcknowledgmentOp {
		return fmt.Errorf("invalid Acknowledgment")
	}

	return binary.Read(reader, binary.BigEndian, &a.BlockNum)
}

type ErrCode uint16

const (
	ErrUnknown         ErrCode = 0
	ErrNotFound        ErrCode = 1
	ErrAccessViolation ErrCode = 2
	ErrDiskFull        ErrCode = 3
	ErrIllegalOp       ErrCode = 4
	ErrUnknownID       ErrCode = 5
	ErrFileExists      ErrCode = 6
	ErrNoUser          ErrCode = 7
)

type Err struct {
	Code    ErrCode
	Message string
}

func (e Err) MarshalBinary() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(5 + len(e.Message)) // 2 (OpCode) + 2 (ErrCode) + n (Message) + 1-byte (0)

	err := binary.Write(b, binary.BigEndian, ErrorOp)
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, e.Code)
	if err != nil {
		return nil, err
	}

	_, err = b.WriteString(e.Message)
	if err != nil {
		return nil, err
	}

	err = b.WriteByte(0)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (e *Err) UnmarshalBinary(buf []byte) error {
	reader := bytes.NewBuffer(buf)
	var code Opcode
	err := binary.Read(reader, binary.BigEndian, &code)
	if err != nil {
		return err
	}
	if code != ErrorOp {
		return fmt.Errorf("invalid Error")
	}

	err = binary.Read(reader, binary.BigEndian, &e.Code)
	if err != nil {
		return err
	}

	e.Message, err = reader.ReadString(0)
	e.Message = strings.TrimRight(e.Message, "\x00")

	return err
}

var (
	_ []encoding.BinaryMarshaler   = []encoding.BinaryMarshaler{ReadWriteRequest{}, &Data{}, Acknowledgment{}, Err{}}
	_ []encoding.BinaryUnmarshaler = []encoding.BinaryUnmarshaler{&ReadWriteRequest{}, &Data{}, &Acknowledgment{}, &Err{}}
)
