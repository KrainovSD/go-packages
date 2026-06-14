package helpers

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

func PrintBits[T ~uint32 | ~uint8 | ~uint16 | ~uint64](value T) {
	size := unsafe.Sizeof(value)

	fmt.Print("[")
	for i := range int(size) {
		b := byte(value >> (i * 8))

		for j := 7; j >= 0; j-- {
			if (b>>j)&1 == 1 {
				fmt.Print("1")
			} else {
				fmt.Print("0")
			}
		}

		if i < int(size)-1 {
			fmt.Print(" ")
		}
	}
	fmt.Print("]")
	fmt.Println()
}

func PrintBytesBits(data []byte) {
	fmt.Print("[")
	for i := range len(data) {
		b := data[i]

		for j := 7; j >= 0; j-- {
			if (b>>j)&1 == 1 {
				fmt.Print("1")
			} else {
				fmt.Print("0")
			}
		}

		if i < len(data)-1 {
			fmt.Print(" ")
		}
	}
	fmt.Print("]")
	fmt.Println()

}

func PrintAnyBytes(bytes []byte) {
	var any any
	var err error
	if err = json.Unmarshal(bytes, &any); err != nil {
		fmt.Println("error: ", err.Error())
		return
	}
	fmt.Println(any)
}

func PrintSctruct(bytes any) {
	fmt.Printf("%+v", bytes)
}

func PrintJson(v any) {
	var b, err = json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(b))
}
