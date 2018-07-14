package h2conn

import "bytes"

func encode(msg []byte) []byte {
	msg = bytes.Replace(msg, []byte{'\n'}, []byte{' '}, -1)
	msg = append(msg, '\n')
	return msg
}

func decode(msg []byte) []byte {
	msg = bytes.TrimRight(msg, "\n")
	return msg
}
