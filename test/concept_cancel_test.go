package test

import (
	"encoding/binary"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestCancelAnywhere(t *testing.T) {

	timeout := time.After(11 * time.Second)
	out, in := io.Pipe()
	go func() {
		i := 0
		for {
			select {
			case <-time.After(time.Second):
				fmt.Printf("[Tick] Time = %d\n", i)
				i++
			case <-timeout:
				return
			}
		}
	}()
	go func() {
		for i := 0; i < 5; i++ {
			select {
			case <-time.NewTimer(5 * time.Second).C:
				err := binary.Write(in, binary.LittleEndian, int8(i))
				if err != nil {
					panic(err)
				}
			case <-timeout:
				err := in.Close()
				if err != nil {
					panic(err)
				}
				return
			}
		}
	}()

OUTER:
	for {
		select {
		default:
			// simulated blocking io
			b := make([]byte, 16)
			n, err := out.Read(b)
			if err != nil {
				fmt.Printf("Read error: %s\n", err.Error())
				break OUTER
			}

			for i := 0; i < n; i++ {
				fmt.Printf("Received byte: %#x\n", b[i])
			}

		case <-timeout:
			fmt.Println("Cancelled")
			break OUTER
		}
	}
}
