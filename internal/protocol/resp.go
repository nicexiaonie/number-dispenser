package protocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RESP (Redis Serialization Protocol) 实现

var (
	ErrInvalidProtocol = errors.New("invalid RESP protocol")
	ErrInvalidBulkSize = errors.New("invalid bulk string size")
)

// RESPType represents the type of RESP data
type RESPType byte

const (
	SimpleString RESPType = '+'
	Error        RESPType = '-'
	Integer      RESPType = ':'
	BulkString   RESPType = '$'
	Array        RESPType = '*'
)

// Value represents a RESP value
type Value struct {
	Type  RESPType
	Str   string
	Num   int64
	Bulk  string
	Array []Value
}

// Reader reads RESP protocol messages
type Reader struct {
	reader *bufio.Reader
}

// NewReader creates a new RESP reader
func NewReader(rd io.Reader) *Reader {
	return &Reader{
		reader: bufio.NewReader(rd),
	}
}

// ReadValue reads a complete RESP value
func (r *Reader) ReadValue() (Value, error) {
	typeByte, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch RESPType(typeByte) {
	case SimpleString:
		return r.readSimpleString()
	case Error:
		return r.readError()
	case Integer:
		return r.readInteger()
	case BulkString:
		return r.readBulkString()
	case Array:
		return r.readArray()
	default:
		return Value{}, ErrInvalidProtocol
	}
}

func (r *Reader) readSimpleString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: SimpleString, Str: line}, nil
}

func (r *Reader) readError() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: Error, Str: line}, nil
}

func (r *Reader) readInteger() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	num, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, err
	}
	return Value{Type: Integer, Num: num}, nil
}

func (r *Reader) readBulkString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	size, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, ErrInvalidBulkSize
	}

	if size == -1 {
		return Value{Type: BulkString, Bulk: ""}, nil
	}

	if size < -1 {
		return Value{}, ErrInvalidBulkSize
	}

	bulk := make([]byte, size+2) // +2 for \r\n
	_, err = io.ReadFull(r.reader, bulk)
	if err != nil {
		return Value{}, err
	}

	return Value{Type: BulkString, Bulk: string(bulk[:size])}, nil
}

func (r *Reader) readArray() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	count, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, err
	}

	if count == -1 {
		return Value{Type: Array, Array: nil}, nil
	}

	array := make([]Value, count)
	for i := 0; i < count; i++ {
		val, err := r.ReadValue()
		if err != nil {
			return Value{}, err
		}
		array[i] = val
	}

	return Value{Type: Array, Array: array}, nil
}

func (r *Reader) readLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

// Writer writes RESP protocol messages
type Writer struct {
	writer *bufio.Writer
}

// NewWriter creates a new RESP writer
func NewWriter(wr io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(wr),
	}
}

// WriteValue writes a RESP value
func (w *Writer) WriteValue(val Value) error {
	switch val.Type {
	case SimpleString:
		return w.WriteSimpleString(val.Str)
	case Error:
		return w.WriteError(val.Str)
	case Integer:
		return w.WriteInteger(val.Num)
	case BulkString:
		return w.WriteBulkString(val.Bulk)
	case Array:
		return w.WriteArray(val.Array)
	default:
		return ErrInvalidProtocol
	}
}

// WriteSimpleString writes a simple string
func (w *Writer) WriteSimpleString(s string) error {
	_, err := w.writer.WriteString(fmt.Sprintf("+%s\r\n", s))
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteError writes an error
func (w *Writer) WriteError(s string) error {
	_, err := w.writer.WriteString(fmt.Sprintf("-%s\r\n", s))
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteInteger writes an integer
func (w *Writer) WriteInteger(n int64) error {
	_, err := w.writer.WriteString(fmt.Sprintf(":%d\r\n", n))
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteBulkString writes a bulk string
func (w *Writer) WriteBulkString(s string) error {
	_, err := w.writer.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(s), s))
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteNull writes a null bulk string
func (w *Writer) WriteNull() error {
	_, err := w.writer.WriteString("$-1\r\n")
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteArray writes an array
func (w *Writer) WriteArray(arr []Value) error {
	if arr == nil {
		_, err := w.writer.WriteString("*-1\r\n")
		if err != nil {
			return err
		}
		return w.writer.Flush()
	}

	_, err := w.writer.WriteString(fmt.Sprintf("*%d\r\n", len(arr)))
	if err != nil {
		return err
	}

	for _, val := range arr {
		if err := w.WriteValue(val); err != nil {
			return err
		}
	}

	return nil
}
