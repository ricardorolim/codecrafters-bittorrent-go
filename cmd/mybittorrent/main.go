package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedReader *bufio.Reader) (interface{}, error) {
	for {
		peeked, err := bencodedReader.Peek(1)
		if err != nil {
			return nil, err
		}
		r := peeked[0]

		if r == 'l' {
			if _, err := bencodedReader.Discard(1); err != nil {
				return 0, err
			}

			decoded := []interface{}{}

			for {
				peeked, err := bencodedReader.Peek(1)
				if err != nil {
					return nil, err
				}

				if peeked[0] == 'e' {
					if _, err := bencodedReader.Discard(1); err != nil {
						return 0, err
					}

					return decoded, nil
				}

				item, err := decodeBencode(bencodedReader)
				if err != nil {
					return nil, err
				}

				decoded = append(decoded, item)
			}
		} else if r == 'd' {
			if _, err := bencodedReader.Discard(1); err != nil {
				return 0, err
			}

			decoded := map[string]interface{}{}

			for {
				peeked, err := bencodedReader.Peek(1)
				if err != nil {
					return nil, err
				}

				if peeked[0] == 'e' {
					if _, err := bencodedReader.Discard(1); err != nil {
						return 0, err
					}

					return decoded, nil
				}

				key, err := decodeString(bencodedReader)
				if err != nil {
					return nil, err
				}

				value, err := decodeBencode(bencodedReader)
				if err != nil {
					return nil, err
				}

				decoded[key] = value
			}
		} else {
			return decodePrimitive(bencodedReader)
		}
	}
}

func decodePrimitive(bencodedReader *bufio.Reader) (interface{}, error) {
	peeked, err := bencodedReader.Peek(1)
	if err != nil {
		return nil, err
	}
	r := peeked[0]

	if unicode.IsDigit(rune(r)) {
		return decodeString(bencodedReader)
	} else if r == 'i' {
		if _, err := bencodedReader.Discard(1); err != nil {
			return 0, err
		}

		intStr, err := bencodedReader.ReadString('e')
		if err != nil {
			return 0, err
		}

		l := len(intStr)
		return strconv.Atoi(intStr[:l - 1])
	} else {
		return nil, fmt.Errorf("Unrecognized primitive")
	}
}

func decodeString(bencodedReader *bufio.Reader) (string, error) {
	peeked, err := bencodedReader.Peek(1)
	if err != nil {
		return "", err
	}

	if !unicode.IsDigit(rune(peeked[0])) {
		return "", errors.New("invalid string")
	}

	lengthStr, err := bencodedReader.ReadString(':')
	if err != nil {
		return "", err
	}

	l := len(lengthStr)
	length, err := strconv.Atoi(lengthStr[:l - 1])
	if err != nil {
		return "", err
	}

	var decodedString = make([]byte, length)
	if _, err := bencodedReader.Read(decodedString); err != nil {
		return "", err
	}

	return string(decodedString), nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := bufio.NewReader(strings.NewReader(os.Args[2]))

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		if len(os.Args) < 3 {
			fmt.Printf("usage: %s info filename\n", os.Args[0])
			os.Exit(1)
		}

		f, err := os.Open(os.Args[2])
		defer f.Close()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
	    }

		decoded, err := decodeBencode(bufio.NewReader(f))
		if err != nil {
			fmt.Println(err)
			return
		}

		var metainfo struct {
			Announce string
			Info struct {
				Length uint64
				Name string
				PieceLength uint64 `json:"piece length"`
				Pieces []byte
			}
		}

		jsonOutput, _ := json.Marshal(decoded)
		json.Unmarshal(jsonOutput, &metainfo)

		fmt.Println("Tracker URL:", metainfo.Announce)
		fmt.Println("Length:", metainfo.Info.Length)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
