package message

import (
	"go-cassandra-native-protocol/cassandraprotocol"
	"io"
)

type Ready struct {
}

func (m *Ready) IsResponse() bool {
	return false
}

func (m *Ready) GetOpCode() cassandraprotocol.OpCode {
	return cassandraprotocol.OpCodeReady
}

func (m *Ready) String() string {
	return "READY"
}

type ReadyCodec struct{}

func (c *ReadyCodec) Encode(_ Message, _ io.Writer, _ cassandraprotocol.ProtocolVersion) error {
	return nil
}

func (c *ReadyCodec) EncodedLength(_ Message, _ cassandraprotocol.ProtocolVersion) (int, error) {
	return 0, nil
}

func (c *ReadyCodec) Decode(_ io.Reader, _ cassandraprotocol.ProtocolVersion) (Message, error) {
	return &Ready{}, nil
}

func (c *ReadyCodec) GetOpCode() cassandraprotocol.OpCode {
	return cassandraprotocol.OpCodeReady
}
