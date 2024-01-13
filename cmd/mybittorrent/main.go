package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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

type InfoMap struct {
	Length int
	Name string
	PieceLength int
	Pieces string
	PieceSlice []string
	infohash []byte
}

func NewInfoMap(info map[string]any) (InfoMap, error) {
	length, ok := info["length"].(int)
	if !ok {
		return InfoMap{}, fmt.Errorf("unexpected type for %v", info["length"])
	}

	name, ok := info["name"].(string)
	if !ok {
		return InfoMap{}, fmt.Errorf("unexpected type for %v", info["name"])
	}

	pieceLength, ok := info["piece length"].(int)
	if !ok {
		return InfoMap{}, fmt.Errorf("unexpected type for %v", info["piece length"])
	}

	pieces, ok := info["pieces"].(string)
	if !ok {
		return InfoMap{}, fmt.Errorf("unexpected type for %v", info["pieces"])
	}
	
	infoMap := InfoMap {
		Length: length,
		Name: name,
		PieceLength: pieceLength,
		Pieces: pieces,
	}

	var err error
	infoMap.infohash, err = infoMap.Hash()
	if err != nil {
		return InfoMap{}, err
	}

	return infoMap, nil
}

func (m *InfoMap) Hash() ([]byte, error) {
	encoded, err := m.Encode()
	if err != nil {
		return nil, err
	}

	h := sha1.New()
	h.Write([]byte(encoded))
	return h.Sum(nil), nil
}

func (m *InfoMap) Encode() (string, error) {
	strs := []string{}

	kvpairs := m.Map()
	keys := make([]string, 0, len(kvpairs))

	for k := range kvpairs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		strs = append(strs, fmt.Sprintf("%d:%s", len(k), k))
		v := kvpairs[k]

		switch v := v.(type) {
		case int:
			n := fmt.Sprintf("i%de", v)
			strs = append(strs, n)
		case string:
			s := fmt.Sprintf("%d:%s", len(v), v)
			strs = append(strs, s)
		default:
			return "", errors.New("unknown encoding type")
		}
	}

	encoded := fmt.Sprintf("d%se", strings.Join(strs, ""))
	return encoded, nil
}

func (i *InfoMap) Map() map[string]any {
	return map[string]interface{} {
		"length": i.Length,
		"name": i.Name,
		"piece length": i.PieceLength,
		"pieces": i.Pieces,
	}
}

func (m *InfoMap) PieceHashes() []string {
	var hashes = make([]string, 0)

	for i := 0; i < len(m.Pieces); i += 20 {
		hashes = append(hashes, m.Pieces[i : i + 20])
	}

	return hashes
}

type MetaInfo struct {
	Announce string
	Info InfoMap
}

func NewMetaInfo(decoded map[string]any) (MetaInfo, error) {
	announce, ok := decoded["announce"].(string)
	if !ok {
		return MetaInfo{}, fmt.Errorf("unexpected type for %v", decoded["announce"])
	}

	info, ok := decoded["info"].(map[string]any)
	if !ok {
		return MetaInfo{}, fmt.Errorf("unexpected type for %v", decoded["info"])
	}

	infomap, err := NewInfoMap(info)
	if err != nil {
		return MetaInfo{}, err
	}

	return MetaInfo {
		Announce: announce,
		Info: infomap,
	}, nil
}

func (m MetaInfo) String() string {
	s := fmt.Sprintln("Tracker URL:", m.Announce)
	s += fmt.Sprintln("Length:", m.Info.Length)
	s += fmt.Sprintln("Info Hash:", fmt.Sprintf("%x", m.Info.infohash))
	s += fmt.Sprintln("Piece Length:", m.Info.PieceLength)
	s += fmt.Sprintln("Piece Hashes:")
	for _, hash := range m.Info.PieceHashes() {
		s += fmt.Sprintln(fmt.Sprintf("%x", hash))
	}

	return s
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

		decoded_map, ok := decoded.(map[string]any)
		if !ok {
			fmt.Printf("unexpected type for %v\n", decoded);
			os.Exit(1)
		}

		metainfo, err := NewMetaInfo(decoded_map)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Print(metainfo)
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
