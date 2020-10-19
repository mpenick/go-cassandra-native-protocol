package message

import (
	"go-cassandra-native-protocol/cassandraprotocol"
	"go-cassandra-native-protocol/cassandraprotocol/primitives"
	"io"
)

type AuthResponse struct {
	Token []byte
}

func (m *AuthResponse) IsResponse() bool {
	return false
}

func (m *AuthResponse) GetOpCode() cassandraprotocol.OpCode {
	return cassandraprotocol.OpCodeAuthResponse
}

func (m *AuthResponse) String() string {
	return "AUTH_RESPONSE " + string(m.Token)
}

type AuthResponseCodec struct{}

func (c *AuthResponseCodec) Encode(msg Message, dest io.Writer, _ cassandraprotocol.ProtocolVersion) error {
	authResponse := msg.(*AuthResponse)
	return primitives.WriteBytes(authResponse.Token, dest)
}

func (c *AuthResponseCodec) EncodedLength(msg Message, _ cassandraprotocol.ProtocolVersion) (int, error) {
	authResponse := msg.(*AuthResponse)
	return primitives.LengthOfBytes(authResponse.Token), nil
}

func (c *AuthResponseCodec) Decode(source io.Reader, _ cassandraprotocol.ProtocolVersion) (Message, error) {
	if token, err := primitives.ReadBytes(source); err != nil {
		return nil, err
	} else {
		return &AuthResponse{Token: token}, nil
	}
}

func (c *AuthResponseCodec) GetOpCode() cassandraprotocol.OpCode {
	return cassandraprotocol.OpCodeAuthResponse
}
