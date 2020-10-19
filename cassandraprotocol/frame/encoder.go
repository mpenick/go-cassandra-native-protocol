package frame

import (
	"bytes"
	"errors"
	"fmt"
	"go-cassandra-native-protocol/cassandraprotocol"
	"go-cassandra-native-protocol/cassandraprotocol/message"
	"go-cassandra-native-protocol/cassandraprotocol/primitives"
	"io"
)

const encodedHeaderLength = 9

func (c *Codec) Encode(frame *Frame) ([]byte, error) {
	version := frame.Header.Version
	if version < cassandraprotocol.ProtocolVersion4 && frame.Body.CustomPayload != nil {
		return nil, fmt.Errorf("custom payloads are not supported in protocol version %v", version)
	}
	if version < cassandraprotocol.ProtocolVersion4 && frame.Body.Warnings != nil {
		return nil, fmt.Errorf("warnings are not supported in protocol version %v", version)
	}
	if c.shouldCompress(frame) {
		return c.encodeFrameCompressed(frame)
	} else {
		return c.encodeFrameUncompressed(frame)
	}
}

func (c *Codec) findEncoder(frame *Frame) (encoder message.Encoder, err error) {
	opCode := frame.Body.Message.GetOpCode()
	encoder, found := c.codecs[opCode]
	if !found {
		err = errors.New(fmt.Sprintf("unsupported opcode %d", opCode))
	}
	return encoder, err
}

func (c *Codec) shouldCompress(frame *Frame) bool {
	opCode := frame.Body.Message.GetOpCode()
	return c.compressor != nil &&
		opCode != cassandraprotocol.OpCodeStartup &&
		opCode != cassandraprotocol.OpCodeOptions
}

func (c *Codec) encodeFrameUncompressed(frame *Frame) ([]byte, error) {
	var err error
	var encodedBodyLength int
	if encodedBodyLength, err = c.uncompressedBodyLength(frame); err != nil {
		return nil, fmt.Errorf("cannot compute length of uncompressed message body: %w", err)
	}
	encodedFrame := bytes.NewBuffer(make([]byte, 0, encodedHeaderLength+encodedBodyLength))
	if err = c.encodeHeader(frame, encodedBodyLength, encodedFrame); err != nil {
		return nil, fmt.Errorf("cannot encode frame header: %w", err)
	}
	if err = c.encodeBodyUncompressed(frame, encodedFrame); err != nil {
		return nil, fmt.Errorf("cannot encode frame body: %w", err)
	}
	return encodedFrame.Bytes(), nil
}

func (c *Codec) encodeFrameCompressed(frame *Frame) ([]byte, error) {
	var err error
	// 1) Encode uncompressed body
	var uncompressedBodyLength int
	if uncompressedBodyLength, err = c.uncompressedBodyLength(frame); err != nil {
		return nil, fmt.Errorf("cannot compute length of uncompressed message body: %w", err)
	}
	uncompressedBody := bytes.NewBuffer(make([]byte, 0, uncompressedBodyLength))
	if err = c.encodeBodyUncompressed(frame, uncompressedBody); err != nil {
		return nil, fmt.Errorf("cannot encode frame body: %w", err)
	}
	// 2) Compress body
	var compressedBody []byte
	if compressedBody, err = c.compressor.Compress(uncompressedBody); err != nil {
		return nil, fmt.Errorf("cannot compress frame body: %w", err)
	}
	compressedBodyLength := len(compressedBody)
	// 3) Encode header
	encodedFrame := bytes.NewBuffer(make([]byte, 0, encodedHeaderLength+compressedBodyLength))
	if err = c.encodeHeader(frame, compressedBodyLength, encodedFrame); err != nil {
		return nil, fmt.Errorf("cannot encode frame header: %w", err)
	}
	// 4) join header and compressed body
	encodedFrame.Write(compressedBody)
	return encodedFrame.Bytes(), nil
}

func (c *Codec) encodeHeader(frame *Frame, bodyLength int, dest io.Writer) error {
	versionAndDirection := frame.Header.Version
	if frame.Body.Message.IsResponse() {
		versionAndDirection |= 0b1000_0000
	}
	if err := primitives.WriteByte(versionAndDirection, dest); err != nil {
		return fmt.Errorf("cannot encode header version and direction: %w", err)
	}
	flags := c.encodeFlags(frame)
	if err := primitives.WriteByte(flags, dest); err != nil {
		return fmt.Errorf("cannot encode header flags: %w", err)
	} else if err = primitives.WriteShort(uint16(frame.Header.StreamId), dest); err != nil {
		return fmt.Errorf("cannot encode header stream id: %w", err)
	} else if err = primitives.WriteByte(frame.Body.Message.GetOpCode(), dest); err != nil {
		return fmt.Errorf("cannot encode header opcode: %w", err)
	} else if err = primitives.WriteInt(int32(bodyLength), dest); err != nil {
		return fmt.Errorf("cannot encode header body length: %w", err)
	}
	return nil
}

func (c *Codec) uncompressedBodyLength(frame *Frame) (length int, err error) {
	if encoder, err := c.findEncoder(frame); err != nil {
		return -1, err
	} else if length, err = encoder.EncodedLength(frame.Body.Message, frame.Header.Version); err != nil {
		return -1, fmt.Errorf("cannot compute message length: %w", err)
	}
	if frame.Body.TracingId != nil {
		length += primitives.LengthOfUuid
	}
	if frame.Body.CustomPayload != nil {
		length += primitives.LengthOfBytesMap(frame.Body.CustomPayload)
	}
	if frame.Body.Warnings != nil {
		length += primitives.LengthOfStringList(frame.Body.Warnings)
	}
	return length, nil
}

func (c *Codec) encodeBodyUncompressed(frame *Frame, dest io.Writer) error {
	var err error
	if frame.Body.Message.IsResponse() && frame.Body.TracingId != nil {
		if err = primitives.WriteUuid(frame.Body.TracingId, dest); err != nil {
			return fmt.Errorf("cannot encode body tracing id: %w", err)
		}
	}
	if frame.Body.CustomPayload != nil {
		if err = primitives.WriteBytesMap(frame.Body.CustomPayload, dest); err != nil {
			return fmt.Errorf("cannot encode body custom payload: %w", err)
		}
	}
	if frame.Body.Warnings != nil {
		if err = primitives.WriteStringList(frame.Body.Warnings, dest); err != nil {
			return fmt.Errorf("cannot encode body warnings: %w", err)
		}
	}
	if encoder, err := c.findEncoder(frame); err != nil {
		return err
	} else if err = encoder.Encode(frame.Body.Message, dest, frame.Header.Version); err != nil {
		return fmt.Errorf("cannot encode body message: %w", err)
	}
	return nil
}

func (c *Codec) encodeFlags(frame *Frame) cassandraprotocol.HeaderFlag {
	var flags cassandraprotocol.HeaderFlag = 0
	if c.shouldCompress(frame) {
		flags |= cassandraprotocol.HeaderFlagCompressed
	}
	if frame.Body.TracingId != nil || frame.Header.TracingRequested {
		flags |= cassandraprotocol.HeaderFlagTracing
	}
	if frame.Body.CustomPayload != nil {
		flags |= cassandraprotocol.HeaderFlagCustomPayload
	}
	if frame.Body.Warnings != nil {
		flags |= cassandraprotocol.HeaderFlagWarning
	}
	if frame.Header.Version == cassandraprotocol.ProtocolVersionBeta {
		flags |= cassandraprotocol.HeaderFlagUseBeta
	}
	return flags
}
